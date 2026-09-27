package relay_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/pkg/blob/fs"
	nagojson "go.wdy.de/nago/pkg/data/json"

	"github.com/torbenschinke/eventprint/app/pairing"
	"github.com/torbenschinke/eventprint/app/relay"
	"github.com/torbenschinke/eventprint/requirements/fun/upload"
)

// fakeService ist der Upload-Dienst mit seiner echten Kopplungslogik hinter
// denselben zwei Adressen, die photoupld anbietet.
func fakeService(t *testing.T) (*httptest.Server, *string) {
	t.Helper()

	var mailBody string
	uc := pairing.NewUseCases(pairing.Options{
		Accounts: func(mail string) (pairing.Account, bool, error) {
			return pairing.Account{ID: "u1", Mail: mail}, mail == "anna@example.org", nil
		},
		Mailer: func(_, _, body string) error {
			mailBody = body
			return nil
		},
		Issuer: func(pairing.Account, string) (pairing.BoxID, string, error) {
			return "box1", "secret-token-for-box", nil
		},
		Boxes: boxStore(t),
		Owner: func(string) error { return nil },
	})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/pairing", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Mail, Device string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		if r.Header.Get("Authorization") != "" {
			t.Error("pairing request must not carry a token")
		}

		id, err := uc.RequestPairing(user.SU(), pairing.RequestCmd{Mail: in.Mail, Device: in.Device})
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{"pairing": id})
	})
	mux.HandleFunc("POST /api/v1/pairing/confirm", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Pairing pairing.ID
			Code    string
		}
		_ = json.NewDecoder(r.Body).Decode(&in)

		res, err := uc.ConfirmPairing(user.SU(), pairing.ConfirmCmd{Pairing: in.Pairing, Code: in.Code})
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(res)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv, &mailBody
}

func boxStore(t *testing.T) pairing.Boxes {
	t.Helper()

	store, err := fs.NewBlobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// Schließen, sonst bricht die Dateisperre von nago mit einem Panic ab,
	// sobald die Speicherbereinigung den Store einsammelt – je nach
	// Zeitpunkt, also gelegentlich.
	t.Cleanup(func() { _ = store.Close() })

	return nagojson.NewSloppyJSONRepository[pairing.Box, pairing.BoxID](store)
}

// An der Box nur die Mailadresse und den Code eintippen: Dienst und Box
// tauschen das Token selbst aus, und die Box speichert es.
func TestBoxPairsWithMailAndCode(t *testing.T) {
	srv, mail := fakeService(t)

	var saved [3]string
	uc := relay.NewUseCases(nil, func(url, token, account string) error {
		saved = [3]string{url, token, account}
		return nil
	})
	ctx := context.Background()

	if _, err := uc.BeginPairing(su, ctx, relay.BeginPairingCmd{URL: srv.URL, Mail: "keine adresse"}); !errors.Is(err, relay.ErrMail) {
		t.Fatalf("invalid mail: %v", err)
	}

	p, err := uc.BeginPairing(su, ctx, relay.BeginPairingCmd{URL: srv.URL, Mail: "anna@example.org", Device: "Wohnzimmer"})
	if err != nil {
		t.Fatal(err)
	}

	code := regexp.MustCompile(`Dein Code: (\d{6})`).FindStringSubmatch(*mail)[1]

	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}

	outcome, err := uc.CompletePairing(su, ctx, p, wrong)
	if err != nil || outcome != relay.PairingInvalid || saved[1] != "" {
		t.Fatalf("wrong code: %v, %v, saved %v", outcome, err, saved)
	}

	outcome, err = uc.CompletePairing(su, ctx, p, code)
	if err != nil || outcome != relay.PairingPaired {
		t.Fatalf("right code: %v, %v", outcome, err)
	}

	if saved != [3]string{srv.URL, "secret-token-for-box", "anna@example.org"} {
		t.Fatalf("saved credentials = %v", saved)
	}

	// Ein Gast darf die Box nicht an sein Konto hängen.
	if _, err := uc.BeginPairing(nobody{}, ctx, relay.BeginPairingCmd{URL: srv.URL, Mail: "anna@example.org"}); err == nil {
		t.Fatal("guest must not start pairing")
	}

	if _, err := uc.CompletePairing(nobody{}, ctx, p, code); err == nil {
		t.Fatal("guest must not complete pairing")
	}

	spec.Verified(t, upload.RUploadKopplung)
}

// Eine unbekannte Adresse sieht an der Box aus wie jede andere: Die
// Anfrage gelingt, nur passt danach kein Code.
func TestBoxCannotTellUnknownAccounts(t *testing.T) {
	srv, mail := fakeService(t)
	uc := relay.NewUseCases(nil, func(string, string, string) error { return nil })

	p, err := uc.BeginPairing(su, context.Background(), relay.BeginPairingCmd{URL: srv.URL, Mail: "fremd@example.org"})
	if err != nil || p.ID == "" {
		t.Fatalf("unknown address: %+v, %v", p, err)
	}

	if *mail != "" {
		t.Fatal("no mail may go to an unknown address")
	}

	outcome, _ := uc.CompletePairing(su, context.Background(), p, "123456")
	if outcome != relay.PairingInvalid {
		t.Fatalf("outcome = %v", outcome)
	}

	spec.Verified(t, upload.RUploadKopplung)
}
