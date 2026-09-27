package pairing_test

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/pkg/blob/fs"
	nagojson "go.wdy.de/nago/pkg/data/json"

	"github.com/torbenschinke/eventprint/app/pairing"
	"github.com/torbenschinke/eventprint/requirements/fun/upload"
)

// service ist der Upload-Dienst im Kleinen: ein bestätigter Nutzer, ein
// Postfach, eine Uhr und ein Token-Aussteller.
type service struct {
	mu      sync.Mutex
	mails   []sent
	tokens  []string
	revoked []pairing.BoxID
	owners  []string
	now     time.Time
	uc      pairing.UseCases
}

// asUser ist ein angemeldeter Nutzer mit allen Rechten, aber eigener
// Kennung – für die Frage, wem eine Box gehört.
type asUser struct {
	user.Subject
	id user.ID
}

func (a asUser) ID() user.ID { return a.id }

// boxes ist ein Speicher für gekoppelte Boxen auf der Platte des Tests.
func boxes(t *testing.T) pairing.Boxes {
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

type sent struct{ to, subject, body string }

func newService(t *testing.T) *service {
	t.Helper()

	s := &service{now: time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)}
	s.uc = pairing.NewUseCases(pairing.Options{
		Accounts: func(mail string) (pairing.Account, bool, error) {
			if mail == "anna@example.org" {
				return pairing.Account{ID: "u1", Mail: "anna@example.org"}, true, nil
			}

			return pairing.Account{}, false, nil
		},
		Mailer: func(to, subject, body string) error {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.mails = append(s.mails, sent{to, subject, body})
			return nil
		},
		Issuer: func(a pairing.Account, device string) (pairing.BoxID, string, error) {
			tok := "token-" + a.ID + "-" + device
			s.tokens = append(s.tokens, tok)
			return pairing.BoxID(fmt.Sprint("box", len(s.tokens))), tok, nil
		},
		Revoker: func(id pairing.BoxID) error {
			s.revoked = append(s.revoked, id)
			return nil
		},
		Owner: func(id string) error {
			s.owners = append(s.owners, id)
			return nil
		},
		Boxes: boxes(t),
		Now:   func() time.Time { return s.now },
	})

	return s
}

var codePattern = regexp.MustCompile(`Dein Code: (\d{6})`)

func (s *service) lastCode(t *testing.T) string {
	t.Helper()

	if len(s.mails) == 0 {
		t.Fatal("no mail sent")
	}

	m := codePattern.FindStringSubmatch(s.mails[len(s.mails)-1].body)
	if m == nil {
		t.Fatalf("no code in mail %q", s.mails[len(s.mails)-1].body)
	}

	return m[1]
}

// Eine Box wird mit Mail und Code gekoppelt, ohne dass jemand ein Token
// abtippt; das Token kommt genau einmal.
func TestPairWithMailAndCode(t *testing.T) {
	s := newService(t)

	id, err := s.uc.RequestPairing(su, pairing.RequestCmd{Mail: "  Anna@Example.org ", Device: "Hochzeit Anna & Ben"})
	if err != nil || id == "" {
		t.Fatalf("request: %q, %v", id, err)
	}

	code := s.lastCode(t)
	if s.mails[0].to != "anna@example.org" || !strings.Contains(s.mails[0].body, "Hochzeit Anna & Ben") {
		t.Fatalf("mail = %+v", s.mails[0])
	}

	res, err := s.uc.ConfirmPairing(su, pairing.ConfirmCmd{Pairing: id, Code: code[:3] + " " + code[3:]})
	if err != nil {
		t.Fatal(err)
	}

	if res.Status != pairing.StatusPaired || res.Token != "token-u1-Hochzeit Anna & Ben" {
		t.Fatalf("confirm = %+v", res)
	}

	// Die Bestätigungsmail sagt, welche Box jetzt verbunden ist.
	if last := s.mails[len(s.mails)-1]; !strings.Contains(last.body, "ist jetzt mit deinem Konto verbunden") {
		t.Fatalf("confirmation mail = %+v", last)
	}

	// Der Code ist verbraucht: kein zweites Token.
	again, _ := s.uc.ConfirmPairing(su, pairing.ConfirmCmd{Pairing: id, Code: code})
	if again.Status == pairing.StatusPaired || len(s.tokens) != 1 {
		t.Fatalf("second confirm = %+v, tokens %d", again, len(s.tokens))
	}

	spec.Verified(t, upload.RUploadKopplung)
}

// Die Box erfährt nicht, ob es das Konto gibt: gleiche Antwort, kein Fehler,
// und nichts passt danach.
func TestUnknownMailLooksTheSame(t *testing.T) {
	s := newService(t)

	id, err := s.uc.RequestPairing(su, pairing.RequestCmd{Mail: "nobody@example.org"})
	if err != nil || id == "" {
		t.Fatalf("unknown mail: %q, %v", id, err)
	}

	if len(s.mails) != 0 {
		t.Fatalf("mail sent to unknown address: %+v", s.mails)
	}

	res, err := s.uc.ConfirmPairing(su, pairing.ConfirmCmd{Pairing: id, Code: "000000"})
	if err != nil || res.Status != pairing.StatusInvalid {
		t.Fatalf("confirm on unknown account = %+v, %v", res, err)
	}

	// Ein zweiter Tipp innerhalb einer Minute gibt dieselbe Kennung – für
	// unbekannte Adressen genauso wie für bekannte.
	again, _ := s.uc.RequestPairing(su, pairing.RequestCmd{Mail: "nobody@example.org"})
	if again != id {
		t.Fatal("unknown address must behave like a known one on a repeated request")
	}

	spec.Verified(t, upload.RUploadKopplung)
}

// Ein zweiter Tipp auf "Code senden" schickt keine zweite Mail und behält
// die Kennung, damit der erste Code passt. Die Drosselung je Adresse hält
// Fremde davon ab, ein Postfach zuzuschütten.
func TestRepeatedRequestsAreThrottled(t *testing.T) {
	s := newService(t)

	first, _ := s.uc.RequestPairing(su, pairing.RequestCmd{Mail: "anna@example.org"})
	second, _ := s.uc.RequestPairing(su, pairing.RequestCmd{Mail: "anna@example.org"})
	if first != second || len(s.mails) != 1 {
		t.Fatalf("double tap: ids %q/%q, mails %d", first, second, len(s.mails))
	}

	for i := 0; i < 10; i++ {
		s.now = s.now.Add(pairing.MailCooldown)
		if _, err := s.uc.RequestPairing(su, pairing.RequestCmd{Mail: "anna@example.org"}); err != nil {
			t.Fatal(err)
		}
	}

	if len(s.mails) > pairing.MailsPerHour {
		t.Fatalf("%d mails within an hour, want at most %d", len(s.mails), pairing.MailsPerHour)
	}

	spec.Verified(t, upload.RUploadKopplung)
}

// Nach 30 Minuten ist der Code abgelaufen, nach fünf falschen Versuchen ist
// die Kopplung gesperrt – auch für den richtigen Code.
func TestExpiryAndLockout(t *testing.T) {
	s := newService(t)

	id, _ := s.uc.RequestPairing(su, pairing.RequestCmd{Mail: "anna@example.org"})
	code := s.lastCode(t)

	s.now = s.now.Add(pairing.CodeTTL + time.Second)
	if res, _ := s.uc.ConfirmPairing(su, pairing.ConfirmCmd{Pairing: id, Code: code}); res.Status != pairing.StatusExpired {
		t.Fatalf("after 30 minutes = %+v", res)
	}

	s.now = s.now.Add(time.Hour)
	id, _ = s.uc.RequestPairing(su, pairing.RequestCmd{Mail: "anna@example.org"})
	code = s.lastCode(t)

	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}

	for i := 1; i < pairing.MaxAttempts; i++ {
		if res, _ := s.uc.ConfirmPairing(su, pairing.ConfirmCmd{Pairing: id, Code: wrong}); res.Status != pairing.StatusInvalid {
			t.Fatalf("attempt %d = %+v", i, res)
		}
	}

	if res, _ := s.uc.ConfirmPairing(su, pairing.ConfirmCmd{Pairing: id, Code: wrong}); res.Status != pairing.StatusLocked {
		t.Fatalf("last attempt = %+v, want locked", res)
	}

	if res, _ := s.uc.ConfirmPairing(su, pairing.ConfirmCmd{Pairing: id, Code: code}); res.Status != pairing.StatusLocked || len(s.tokens) != 0 {
		t.Fatalf("right code after lockout = %+v", res)
	}

	spec.Verified(t, upload.RUploadKopplung)
}

// Scheitert das Ausstellen des Tokens, ist das ein Fehler und kein
// "falscher Code".
func TestIssuerFailureIsAnError(t *testing.T) {
	uc := pairing.NewUseCases(pairing.Options{
		Accounts: func(string) (pairing.Account, bool, error) { return pairing.Account{ID: "u", Mail: "a@b.c"}, true, nil },
		Mailer: func(_, _, body string) error {
			lastBody = body
			return nil
		},
		Issuer: func(pairing.Account, string) (pairing.BoxID, string, error) { return "", "", errors.New("db down") },
		Boxes:  boxes(t),
		Owner:  func(string) error { return nil },
	})

	id, _ := uc.RequestPairing(su, pairing.RequestCmd{Mail: "a@b.c"})
	code := codePattern.FindStringSubmatch(lastBody)[1]

	if _, err := uc.ConfirmPairing(su, pairing.ConfirmCmd{Pairing: id, Code: code}); err == nil {
		t.Fatal("issuer failure must surface as an error")
	}

	spec.Verified(t, upload.RUploadKopplung)
}

var lastBody string

var su = user.SU()

// Der Nutzer sieht seine gekoppelten Boxen und trennt sie selbst; fremde
// Boxen sieht er nicht und trennt er nicht.
func TestOwnerSeesAndUnpairsOwnBoxes(t *testing.T) {
	s := newService(t)

	id, _ := s.uc.RequestPairing(su, pairing.RequestCmd{Mail: "anna@example.org", Device: "Wohnzimmer"})
	if res, _ := s.uc.ConfirmPairing(su, pairing.ConfirmCmd{Pairing: id, Code: s.lastCode(t)}); res.Status != pairing.StatusPaired {
		t.Fatalf("pairing failed: %+v", res)
	}

	// Beim ersten Koppeln bekommt der Nutzer die Rolle, mit der er seine
	// Boxen sieht.
	if len(s.owners) != 1 || s.owners[0] != "u1" {
		t.Fatalf("owner role granted to %v", s.owners)
	}

	anna := asUser{Subject: su, id: "u1"}
	ben := asUser{Subject: su, id: "u2"}

	mine, err := s.uc.FindMyBoxes(anna)
	if err != nil || len(mine) != 1 || mine[0].Device != "Wohnzimmer" || mine[0].Mail != "anna@example.org" {
		t.Fatalf("anna's boxes = %+v, %v", mine, err)
	}

	if others, _ := s.uc.FindMyBoxes(ben); len(others) != 0 {
		t.Fatalf("ben sees %+v", others)
	}

	if err := s.uc.UnpairBox(ben, mine[0].ID); err == nil || len(s.revoked) != 0 {
		t.Fatalf("ben unpaired anna's box: %v, revoked %v", err, s.revoked)
	}

	if err := s.uc.UnpairBox(anna, mine[0].ID); err != nil {
		t.Fatal(err)
	}

	if len(s.revoked) != 1 || s.revoked[0] != mine[0].ID {
		t.Fatalf("revoked = %v", s.revoked)
	}

	if left, _ := s.uc.FindMyBoxes(anna); len(left) != 0 {
		t.Fatalf("after unpairing anna still has %+v", left)
	}

	spec.Verified(t, upload.RUploadKopplung)
}
