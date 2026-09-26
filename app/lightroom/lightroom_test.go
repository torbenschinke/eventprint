package lightroom_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"testing"
	"time"

	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/lightroom"
)

func setup(t *testing.T) (*fakeAdobe, *memStore, lightroom.UseCases) {
	t.Helper()

	fake := newFakeAdobe(t)
	store := &memStore{}

	return fake, store, lightroom.NewUseCases(fake.config, store)
}

// TestConnectViaRelay spielt den ganzen Weg: QR-Code, Warten, Anmeldung auf
// dem Telefon, Übernahme der Schlüssel.
func TestConnectViaRelay(t *testing.T) {
	fake, store, uc := setup(t)
	ctx := context.Background()
	sub := &allow{}

	before := time.Now()

	auth, err := uc.BeginConnect(sub, ctx)
	if err != nil {
		t.Fatalf("BeginConnect: %v", err)
	}

	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(auth.State) {
		t.Fatalf("State = %q, erwartet 32 Hexzeichen", auth.State)
	}

	if auth.StartURL != fake.srv.URL+"/oauth/adobe/start?s="+auth.State {
		t.Fatalf("StartURL = %q", auth.StartURL)
	}

	if d := auth.ExpiresAt.Sub(before); d < 14*time.Minute || d > 16*time.Minute {
		t.Fatalf("ExpiresAt liegt %v in der Zukunft, erwartet etwa 15 Minuten", d)
	}

	fake.mu.Lock()
	registered := fake.registered[auth.State]
	fake.mu.Unlock()

	u, err := url.Parse(registered)
	if err != nil {
		t.Fatalf("Anmeldeadresse %q: %v", registered, err)
	}

	if u.Path != "/ims/authorize/v2" {
		t.Fatalf("Pfad der Anmeldeadresse = %q", u.Path)
	}

	want := map[string]string{
		"client_id":     testClientID,
		"redirect_uri":  fake.srv.URL + "/oauth/adobe/callback",
		"scope":         lightroom.DefaultScopes,
		"response_type": "code",
		"state":         auth.State,
	}

	for k, v := range want {
		if got := u.Query().Get(k); got != v {
			t.Errorf("Parameter %s = %q, erwartet %q", k, got, v)
		}
	}

	if u.Query().Has("client_secret") {
		t.Fatal("das Client-Secret darf nie in der Anmeldeadresse stehen, sie landet auf dem Telefon")
	}

	status, err := uc.AwaitConnect(sub, ctx, auth.State)
	if err != nil || status != lightroom.StatusPending {
		t.Fatalf("vor der Anmeldung: status=%v err=%v, erwartet pending", status, err)
	}

	if _, ok := store.get(); ok {
		t.Fatal("vor der Anmeldung dürfen keine Schlüssel gespeichert sein")
	}

	fake.complete(auth.State)

	status, err = uc.AwaitConnect(sub, ctx, auth.State)
	if err != nil || status != lightroom.StatusConnected {
		t.Fatalf("nach der Anmeldung: status=%v err=%v, erwartet connected", status, err)
	}

	tokens, ok := store.get()
	if !ok || tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("Schlüssel nicht gespeichert: %+v", tokens)
	}

	if d := time.Until(tokens.ExpiresAt); d < 23*time.Hour || d > 25*time.Hour {
		t.Fatalf("Ablauf %v, erwartet etwa 24 h aus expires_in in Sekunden", d)
	}

	fake.mu.Lock()
	form := fake.lastTokenForm
	fake.mu.Unlock()

	if form["grant_type"] != "authorization_code" || form["code"] != "code-for-"+auth.State {
		t.Fatalf("Token-Tausch mit %v", form)
	}

	acc, err := uc.Account(sub, ctx)
	if err != nil || !acc.Connected || acc.Name != "Torben Schinke" {
		t.Fatalf("Account = %+v, err = %v", acc, err)
	}
}

func TestAwaitConnect(t *testing.T) {
	tests := []struct {
		name       string
		prepare    func(f *fakeAdobe, state string)
		state      string
		wantStatus lightroom.Status
		wantErr    bool
	}{
		{
			name:       "unbekannter State ist abgelaufen",
			state:      "0123456789abcdef0123456789abcdef",
			wantStatus: lightroom.StatusExpired,
		},
		{
			name:    "ungültiger State wird nicht an den Relay geschickt",
			state:   "../../etc/passwd",
			wantErr: true,
		},
		{
			name:  "Relay reicht Ablehnung durch",
			state: "0123456789abcdef0123456789abcdef",
			prepare: func(f *fakeAdobe, state string) {
				f.codes[state] = ""
				f.relayError[state] = "access_denied"
			},
			wantErr: true,
		},
		{
			name:  "Code ungültig beim Tausch",
			state: "0123456789abcdef0123456789abcdef",
			prepare: func(f *fakeAdobe, state string) {
				f.codes[state] = "falsch"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, store, uc := setup(t)
			if tt.prepare != nil {
				fake.mu.Lock()
				tt.prepare(fake, tt.state)
				fake.mu.Unlock()
			}

			before := fake.requests()
			status, err := uc.AwaitConnect(&allow{}, context.Background(), tt.state)

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, erwartet Fehler: %v", err, tt.wantErr)
			}

			if !tt.wantErr && status != tt.wantStatus {
				t.Fatalf("status = %v, erwartet %v", status, tt.wantStatus)
			}

			if tt.state == "../../etc/passwd" && fake.requests() != before {
				t.Fatal("ungültiger State ging trotzdem an den Relay")
			}

			if _, ok := store.get(); ok {
				t.Fatal("ohne erfolgreichen Tausch dürfen keine Schlüssel gespeichert sein")
			}
		})
	}
}

func TestConnectNeedsConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		change func(*lightroom.Config)
	}{
		{"ohne Client-ID", func(c *lightroom.Config) { c.ClientID = "" }},
		{"ohne Client-Secret", func(c *lightroom.Config) { c.ClientSecret = " " }},
		{"ohne Relay", func(c *lightroom.Config) { c.RelayURL = "" }},
		{"ohne Relay-Token", func(c *lightroom.Config) { c.RelayToken = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeAdobe(t)
			uc := lightroom.NewUseCases(func() lightroom.Config {
				c := fake.config()
				tt.change(&c)
				return c
			}, &memStore{})

			_, err := uc.BeginConnect(&allow{}, context.Background())

			var nc lightroom.NotConfiguredError
			if !errors.As(err, &nc) {
				t.Fatalf("err = %v, erwartet NotConfiguredError", err)
			}

			if fake.requests() != 0 {
				t.Fatal("ohne vollständige Einstellungen darf nichts ins Netz gehen")
			}
		})
	}
}

func TestRelayRejectsWrongToken(t *testing.T) {
	fake := newFakeAdobe(t)
	uc := lightroom.NewUseCases(func() lightroom.Config {
		c := fake.config()
		c.RelayToken = "falsch"
		return c
	}, &memStore{})

	_, err := uc.BeginConnect(&allow{}, context.Background())

	var httpErr lightroom.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusUnauthorized {
		t.Fatalf("err = %v, erwartet HTTP 401 vom Relay", err)
	}
}

// TestEveryUseCaseAuditsItsOwnPermission prüft, dass jeder Anwendungsfall
// zuerst seine eigene Berechtigung prüft und ohne sie nichts tut.
func TestEveryUseCaseAuditsItsOwnPermission(t *testing.T) {
	ctx := context.Background()
	state := "0123456789abcdef0123456789abcdef"

	calls := []struct {
		name string
		perm permission.ID
		call func(uc lightroom.UseCases, s permission.Auditable) error
	}{
		{"BeginConnect", lightroom.PermBeginConnect, func(uc lightroom.UseCases, s permission.Auditable) error {
			_, err := uc.BeginConnect(s, ctx)
			return err
		}},
		{"AwaitConnect", lightroom.PermAwaitConnect, func(uc lightroom.UseCases, s permission.Auditable) error {
			_, err := uc.AwaitConnect(s, ctx, state)
			return err
		}},
		{"Disconnect", lightroom.PermDisconnect, func(uc lightroom.UseCases, s permission.Auditable) error {
			return uc.Disconnect(s)
		}},
		{"Account", lightroom.PermAccount, func(uc lightroom.UseCases, s permission.Auditable) error {
			_, err := uc.Account(s, ctx)
			return err
		}},
		{"Albums", lightroom.PermAlbums, func(uc lightroom.UseCases, s permission.Auditable) error {
			_, err := uc.Albums(s, ctx)
			return err
		}},
		{"Assets", lightroom.PermAssets, func(uc lightroom.UseCases, s permission.Auditable) error {
			_, err := uc.Assets(s, ctx, "", "")
			return err
		}},
		{"OpenThumbnail", lightroom.PermOpenThumbnail, func(uc lightroom.UseCases, s permission.Auditable) error {
			rc, err := uc.OpenThumbnail(s, ctx, "img-1")
			if rc != nil {
				_ = rc.Close()
			}
			return err
		}},
		{"Download", lightroom.PermDownload, func(uc lightroom.UseCases, s permission.Auditable) error {
			_, err := uc.Download(s, ctx, "img-1")
			return err
		}},
	}

	if len(calls) != len(lightroom.Permissions()) {
		t.Fatalf("%d Anwendungsfälle geprüft, aber %d Berechtigungen deklariert", len(calls), len(lightroom.Permissions()))
	}

	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			fake, store, uc := setup(t)
			fake.grant(store, time.Hour)

			if err := c.call(uc, deny{}); !errors.Is(err, errDenied) {
				t.Fatalf("err = %v, erwartet die Verweigerung", err)
			}

			if fake.requests() != 0 {
				t.Fatal("trotz Verweigerung ging eine Anfrage hinaus")
			}

			if _, ok := store.get(); !ok {
				t.Fatal("trotz Verweigerung wurden die Schlüssel verändert")
			}

			sub := &allow{}
			_ = c.call(uc, sub)

			if len(sub.audited) == 0 || sub.audited[0] != c.perm {
				t.Fatalf("geprüft wurde %v, erwartet zuerst %v", sub.audited, c.perm)
			}

			if !slices.Contains(lightroom.Permissions(), c.perm) {
				t.Fatalf("%v fehlt in Permissions()", c.perm)
			}
		})
	}
}

func TestAlbumsFollowAllPagesAndSkipFolders(t *testing.T) {
	fake, store, uc := setup(t)
	fake.grant(store, time.Hour)

	for range 2 {
		albums, err := uc.Albums(&allow{}, context.Background())
		if err != nil {
			t.Fatalf("Albums: %v", err)
		}

		want := []lightroom.Album{
			{ID: "a-geburtstag", Name: "Geburtstag"},
			{ID: "a-hochzeit", Name: "hochzeit"},
			{ID: "a-zoo", Name: "Zoo", CoverAssetID: "cover-1"},
		}

		if !slices.Equal(albums, want) {
			t.Fatalf("Alben = %+v\nerwartet %+v", albums, want)
		}
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()

	if fake.catalogCalls != 1 {
		t.Fatalf("Katalog %d-mal abgefragt, erwartet einmal (zwischengespeichert)", fake.catalogCalls)
	}

	if fake.lastAPIKey != testClientID {
		t.Fatalf("X-API-Key = %q", fake.lastAPIKey)
	}
}

func TestAssets(t *testing.T) {
	local := func(s string) time.Time {
		v, _ := time.ParseInLocation("2006-01-02T15:04:05", s, time.Local)
		return v
	}

	t.Run("Album seitenweise, Videos übersprungen", func(t *testing.T) {
		fake, store, uc := setup(t)
		fake.grant(store, time.Hour)

		first, err := uc.Assets(&allow{}, context.Background(), "a-zoo", "")
		if err != nil {
			t.Fatalf("erste Seite: %v", err)
		}

		want := []lightroom.Asset{{
			ID: "img-1", FileName: "IMG_0001.CR3", Width: 3000, Height: 2000, Rating: 3,
			CapturedAt: local("2024-06-01T14:30:00"),
		}}

		if !assetsEqual(first.Assets, want) {
			t.Fatalf("erste Seite = %+v\nerwartet %+v", first.Assets, want)
		}

		if first.Next == "" {
			t.Fatal("erste Seite ohne Marke für die nächste")
		}

		second, err := uc.Assets(&allow{}, context.Background(), "a-zoo", first.Next)
		if err != nil {
			t.Fatalf("zweite Seite: %v", err)
		}

		want = []lightroom.Asset{{
			ID: "img-2", FileName: "DSC_2.jpg", Width: 4000, Height: 6000, Rating: 4,
			CapturedAt: time.Date(2024, 6, 1, 15, 0, 0, 123_000_000, time.UTC),
		}}

		if !assetsEqual(second.Assets, want) || second.Next != "" {
			t.Fatalf("zweite Seite = %+v next=%q", second.Assets, second.Next)
		}
	})

	t.Run("Seite nur mit Videos wird übersprungen", func(t *testing.T) {
		fake, store, uc := setup(t)
		fake.grant(store, time.Hour)

		page, err := uc.Assets(&allow{}, context.Background(), "a-videos", "")
		if err != nil {
			t.Fatalf("Assets: %v", err)
		}

		if len(page.Assets) != 1 || page.Assets[0].ID != "img-3" || page.Next != "" {
			t.Fatalf("Seite = %+v", page)
		}
	})

	t.Run("alle Fotos", func(t *testing.T) {
		fake, store, uc := setup(t)
		fake.grant(store, time.Hour)

		page, err := uc.Assets(&allow{}, context.Background(), "", "")
		if err != nil {
			t.Fatalf("Assets: %v", err)
		}

		want := []lightroom.Asset{{ID: "img-9", FileName: "IMG_9.HEIC", Width: 4032, Height: 3024}}
		if !assetsEqual(page.Assets, want) || page.Next != "" {
			t.Fatalf("Seite = %+v", page)
		}
	})

	t.Run("fremde Marken werden abgelehnt", func(t *testing.T) {
		for _, cursor := range []string{
			"//evil.example/v2/catalogs/cat1/assets",
			"https://evil.example/v2/catalogs/cat1/assets",
			"/v2/catalogs/other/assets?x=1",
		} {
			fake, store, uc := setup(t)
			fake.grant(store, time.Hour)

			if _, err := uc.Assets(&allow{}, context.Background(), "", cursor); err == nil {
				t.Fatalf("Marke %q wurde angenommen", cursor)
			}
		}
	})

	t.Run("Verweis auf fremden Rechner", func(t *testing.T) {
		fake, store, uc := setup(t)
		fake.grant(store, time.Hour)

		if _, err := uc.Assets(&allow{}, context.Background(), "a-evil", ""); err == nil {
			t.Fatal("Verweis auf einen fremden Rechner wurde übernommen")
		}
	})
}

func assetsEqual(a, b []lightroom.Asset) bool {
	return slices.EqualFunc(a, b, func(x, y lightroom.Asset) bool {
		return x.ID == y.ID && x.FileName == y.FileName && x.Width == y.Width && x.Height == y.Height &&
			x.Rating == y.Rating && x.CapturedAt.Equal(y.CapturedAt)
	})
}

func TestTokenRefresh(t *testing.T) {
	tests := []struct {
		name          string
		expiresIn     time.Duration
		revoke        bool // Lightroom lehnt den Schlüssel trotz gültiger Uhr ab
		rotate        bool
		refreshStatus int
		wantRefreshes int
		wantErr       bool
		wantNotConn   bool
		wantTokens    bool
	}{
		{name: "gültiger Schlüssel wird nicht erneuert", expiresIn: time.Hour, wantTokens: true},
		{name: "kurz vor Ablauf wird erneuert", expiresIn: 30 * time.Second, wantRefreshes: 1, wantTokens: true},
		{name: "abgelaufen wird erneuert", expiresIn: -time.Hour, wantRefreshes: 1, wantTokens: true},
		{name: "neuer Refresh-Token wird übernommen", expiresIn: -time.Hour, rotate: true, wantRefreshes: 1, wantTokens: true},
		{name: "401 von Lightroom erneuert einmal", expiresIn: time.Hour, revoke: true, wantRefreshes: 1, wantTokens: true},
		{name: "Erneuerung mit 400 trennt", expiresIn: -time.Hour, refreshStatus: 400, wantRefreshes: 1, wantErr: true, wantNotConn: true},
		{name: "Erneuerung mit 401 trennt", expiresIn: -time.Hour, refreshStatus: 401, wantRefreshes: 1, wantErr: true, wantNotConn: true},
		{name: "Störung bei Adobe behält die Schlüssel", expiresIn: -time.Hour, refreshStatus: 503, wantRefreshes: 1, wantErr: true, wantTokens: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, store, uc := setup(t)
			fake.grant(store, tt.expiresIn)
			old, _ := store.get()

			fake.mu.Lock()
			fake.refreshStatus = tt.refreshStatus
			fake.rotateRefresh = tt.rotate
			fake.mu.Unlock()

			if tt.revoke {
				fake.revokeAccess()
			}

			_, err := uc.Albums(&allow{}, context.Background())

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, erwartet Fehler: %v", err, tt.wantErr)
			}

			var nc lightroom.NotConnectedError
			if errors.As(err, &nc) != tt.wantNotConn {
				t.Fatalf("err = %v, NotConnectedError erwartet: %v", err, tt.wantNotConn)
			}

			fake.mu.Lock()
			refreshes := fake.refreshCalls
			fake.mu.Unlock()

			if refreshes != tt.wantRefreshes {
				t.Fatalf("%d Erneuerungen, erwartet %d", refreshes, tt.wantRefreshes)
			}

			tokens, ok := store.get()
			if ok != tt.wantTokens {
				t.Fatalf("Schlüssel vorhanden: %v, erwartet %v", ok, tt.wantTokens)
			}

			if !ok {
				acc, err := uc.Account(&allow{}, context.Background())
				if err != nil || acc.Connected {
					t.Fatalf("nach dem Verlust: Account = %+v, err = %v", acc, err)
				}

				return
			}

			if tt.wantRefreshes > 0 && tt.refreshStatus == 0 {
				if tokens.AccessToken == old.AccessToken {
					t.Fatal("Zugriffsschlüssel nicht erneuert")
				}

				if !tt.rotate && tokens.RefreshToken != old.RefreshToken {
					t.Fatal("ohne neuen Refresh-Token muss der alte erhalten bleiben")
				}

				if tt.rotate && tokens.RefreshToken == old.RefreshToken {
					t.Fatal("neuer Refresh-Token wurde nicht übernommen")
				}

				if time.Until(tokens.ExpiresAt) < time.Hour {
					t.Fatalf("Ablauf nicht fortgeschrieben: %v", tokens.ExpiresAt)
				}
			}
		})
	}
}

// TestConcurrentRequestsRefreshOnce: Die Übersicht lädt viele Vorschauen
// zugleich; erneuert werden darf trotzdem nur einmal.
func TestConcurrentRequestsRefreshOnce(t *testing.T) {
	fake, store, uc := setup(t)
	fake.grant(store, -time.Hour)
	fake.renditions["img-1"] = map[string][]byte{"thumbnail2x": []byte("thumb")}

	errs := make(chan error, 16)
	for range cap(errs) {
		go func() {
			rc, err := uc.OpenThumbnail(&allow{}, context.Background(), "img-1")
			if err == nil {
				_, err = io.ReadAll(rc)
				_ = rc.Close()
			}
			errs <- err
		}()
	}

	for range cap(errs) {
		if err := <-errs; err != nil {
			t.Fatalf("OpenThumbnail: %v", err)
		}
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()

	if fake.refreshCalls != 1 {
		t.Fatalf("%d Erneuerungen, erwartet genau eine", fake.refreshCalls)
	}
}

func TestOpenThumbnail(t *testing.T) {
	fake, store, uc := setup(t)
	fake.grant(store, time.Hour)
	fake.renditions["img-1"] = map[string][]byte{"thumbnail2x": []byte("thumb-bytes")}

	rc, err := uc.OpenThumbnail(&allow{}, context.Background(), "img-1")
	if err != nil {
		t.Fatalf("OpenThumbnail: %v", err)
	}
	defer rc.Close()

	got, _ := io.ReadAll(rc)
	if string(got) != "thumb-bytes" {
		t.Fatalf("Vorschau = %q", got)
	}

	_, err = uc.OpenThumbnail(&allow{}, context.Background(), "fehlt")

	var httpErr lightroom.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusNotFound {
		t.Fatalf("err = %v, erwartet HTTP 404", err)
	}
}

func TestDownload(t *testing.T) {
	big := bytes.Repeat([]byte{0xff}, 2048)
	small := bytes.Repeat([]byte{0xaa}, 1280)

	tests := []struct {
		name       string
		renditions map[string][]byte
		fileName   string // leer: Metadaten fehlen
		wantData   []byte
		wantName   string
		wantErr    bool
	}{
		{
			name:       "2048 bevorzugt",
			renditions: map[string][]byte{"2048": big, "1280": small, "thumbnail2x": []byte("t")},
			fileName:   "IMG_0001.CR3",
			wantData:   big,
			wantName:   "IMG_0001.jpg",
		},
		{
			name:       "ohne 2048 gilt 1280",
			renditions: map[string][]byte{"1280": small},
			fileName:   `C:\Fotos\Feier.JPEG`,
			wantData:   small,
			wantName:   "Feier.jpg",
		},
		{
			name:       "ohne Metadaten ein Name aus der Kennung",
			renditions: map[string][]byte{"2048": big},
			wantData:   big,
			wantName:   "lightroom-asset1.jpg",
		},
		{
			name:       "ohne Druckstufe ein Fehler",
			renditions: map[string][]byte{"thumbnail2x": []byte("t")},
			fileName:   "x.jpg",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, store, uc := setup(t)
			fake.grant(store, time.Hour)
			fake.renditions["asset1"] = tt.renditions

			if tt.fileName != "" {
				fake.fileNames["asset1"] = tt.fileName
			}

			photo, err := uc.Download(&allow{}, context.Background(), "asset1")
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, erwartet Fehler: %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if !bytes.Equal(photo.Data, tt.wantData) {
				t.Fatalf("%d Bytes geladen, erwartet %d", len(photo.Data), len(tt.wantData))
			}

			if photo.Name != tt.wantName {
				t.Fatalf("Name = %q, erwartet %q", photo.Name, tt.wantName)
			}
		})
	}
}

func TestDisconnect(t *testing.T) {
	fake, store, uc := setup(t)
	fake.grant(store, time.Hour)
	ctx := context.Background()

	if _, err := uc.Albums(&allow{}, ctx); err != nil {
		t.Fatalf("Albums vor dem Trennen: %v", err)
	}

	if err := uc.Disconnect(&allow{}); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}

	if _, ok := store.get(); ok {
		t.Fatal("Schlüssel nach dem Trennen noch vorhanden")
	}

	acc, err := uc.Account(&allow{}, ctx)
	if err != nil || acc.Connected {
		t.Fatalf("Account nach dem Trennen = %+v, err = %v", acc, err)
	}

	_, err = uc.Albums(&allow{}, ctx)

	var nc lightroom.NotConnectedError
	if !errors.As(err, &nc) {
		t.Fatalf("err = %v, erwartet NotConnectedError (auch nicht aus dem Zwischenspeicher)", err)
	}

	if err := uc.Disconnect(&allow{}); err != nil {
		t.Fatalf("zweites Trennen: %v", err)
	}
}

// TestAccountStaysConnectedWhileOffline: Fehlt nur das Netz, darf die Box
// nicht zur neuen Anmeldung auffordern.
func TestAccountStaysConnectedWhileOffline(t *testing.T) {
	fake, store, uc := setup(t)
	fake.grant(store, time.Hour)
	fake.srv.Close()

	acc, err := uc.Account(&allow{}, context.Background())
	if err != nil || !acc.Connected {
		t.Fatalf("Account = %+v, err = %v", acc, err)
	}
}

// TestConfigIsReadOnEveryCall: Geänderte Einstellungen gelten ohne Neustart.
func TestConfigIsReadOnEveryCall(t *testing.T) {
	fake := newFakeAdobe(t)
	store := &memStore{}
	fake.grant(store, time.Hour)

	key := "anders"
	uc := lightroom.NewUseCases(func() lightroom.Config {
		c := fake.config()
		c.ClientID = key
		return c
	}, store)

	if _, err := uc.Albums(&allow{}, context.Background()); err == nil {
		t.Fatal("mit falscher Client-ID erwartet Lightroom eine Ablehnung")
	}

	key = testClientID

	if _, err := uc.Albums(&allow{}, context.Background()); err != nil {
		t.Fatalf("nach Korrektur der Einstellungen: %v", err)
	}
}
