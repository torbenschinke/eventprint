package relay_test

import (
	"strings"
	"testing"
	"time"

	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/app/relay"
	"github.com/torbenschinke/eventprint/requirements/fun/upload"
)

const interval = 5 * time.Millisecond

// Ein QR-Code, der ins Leere führt, ist schlimmer als keiner. Solange keine
// Sitzung steht, gibt es deshalb keine Adresse, sondern einen Satz, der sagt,
// woran es liegt.
func TestUploadAddressExplainsWhyThereIsNoCode(t *testing.T) {
	cases := []struct {
		name    string
		opts    relay.Options
		state   relay.State
		problem string
	}{
		{"nicht eingerichtet", relay.Options{}, relay.StateOff, "Kein Upload-Dienst eingerichtet."},
		{"Token fehlt", relay.Options{URL: "https://upload.example"}, relay.StateMissingToken, "Für den Upload-Dienst fehlt das Zugangstoken."},
		{"nur Leerzeichen", relay.Options{URL: "https://upload.example", Token: "  "}, relay.StateMissingToken, "Für den Upload-Dienst fehlt das Zugangstoken."},
		{"noch keine Sitzung", relay.Options{URL: "https://upload.example", Token: token}, relay.StateConnecting, "Verbindung zum Upload-Dienst wird aufgebaut."},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := relay.NewPoller(newSettings(c.opts).load, (&inbox{}).deliver)
			uc := relay.NewUseCases(p, nil)

			for _, inboxMode := range []bool{false, true} {
				a, err := uc.UploadAddress(su, inboxMode)
				if err != nil {
					t.Fatal(err)
				}

				if a.URL != "" || a.State != c.state || a.Problem != c.problem {
					t.Fatalf("inbox=%v: %+v, erwartet State %d und %q", inboxMode, a, c.state, c.problem)
				}
			}

			spec.Verified(t, upload.RUploadSitzung)
		})
	}
}

// Ein abgewiesenes Token sieht von außen aus wie "Dienst nicht da". Die
// Anzeige muss das vom bloßen Verbindungsaufbau unterscheiden, sonst wartet
// man vor dem Gerät auf etwas, das nie kommt.
func TestUploadAddressReportsRejectedToken(t *testing.T) {
	f := newFakeRelay(t)
	p := relay.NewPoller(newSettings(relay.Options{URL: f.srv.URL, Token: "falsch", Interval: interval}).load, (&inbox{}).deliver)
	run(t, p)

	waitFor(t, "Fehler nach abgewiesenem Token", func() bool { return p.LastError() != "" })

	a, err := relay.NewUploadAddress(p)(su, true)
	if err != nil {
		t.Fatal(err)
	}

	if a.URL != "" || a.State != relay.StateConnecting || a.Problem != "Upload-Dienst nicht erreichbar." {
		t.Fatalf("Adresse = %+v", a)
	}

	// Die Begründung des Dienstes landet in der Meldung, samt Hinweis aufs
	// Token.
	if !strings.Contains(p.LastError(), "unbekanntes Token") || !strings.Contains(p.LastError(), "Token prüfen") {
		t.Fatalf("LastError = %q", p.LastError())
	}

	spec.Verified(t, upload.RUploadSitzung)
}

func TestUploadAddressNeedsPermission(t *testing.T) {
	p := relay.NewPoller(newSettings(relay.Options{}).load, (&inbox{}).deliver)
	if _, err := relay.NewUploadAddress(p)(nobody{}, false); err == nil {
		t.Fatal("UploadAddress ohne Berechtigung gelang")
	}
}

// Das Gerät öffnet genau eine Sitzung und zeigt deren Adresse – ohne bei
// jedem Durchlauf eine neue zu holen, denn dann wäre der gerade gezeigte
// QR-Code schon wieder veraltet.
func TestSessionProvidesTheUploadAddress(t *testing.T) {
	f := newFakeRelay(t)
	p := relay.NewPoller(newSettings(relay.Options{URL: f.srv.URL, Token: token, Interval: interval}).load, (&inbox{}).deliver)
	run(t, p)

	waitFor(t, "Sitzung", func() bool { return p.UploadURL() != "" })
	settle()

	if _, sessions, _ := f.snapshot(); sessions != 1 {
		t.Fatalf("%d Sitzungen geöffnet, erwartet 1", sessions)
	}

	if p.State() != relay.StateReady || p.LastError() != "" {
		t.Fatalf("State=%d LastError=%q", p.State(), p.LastError())
	}

	a, err := relay.NewUploadAddress(p)(su, false)
	if err != nil {
		t.Fatal(err)
	}

	if a.URL != f.srv.URL+"/upload?u=s1" || a.State != relay.StateReady || a.Problem != "" {
		t.Fatalf("Adresse = %+v", a)
	}

	spec.Verified(t, upload.RUploadSitzung)
}

// Im Heimbetrieb bestimmt die Adresse im QR-Code, dass die Upload-Seite
// mehrere Bilder ohne Layoutwahl annimmt. Der Parameter kommt hinzu, ohne die
// Kennung der Sitzung zu verlieren – ohne sie nähme der Dienst gar nichts an.
func TestInboxAddressAddsModeAndKeepsSession(t *testing.T) {
	f := newFakeRelay(t)
	p := relay.NewPoller(newSettings(relay.Options{URL: f.srv.URL, Token: token, Interval: interval}).load, (&inbox{}).deliver)
	run(t, p)

	waitFor(t, "Sitzung", func() bool { return p.UploadURL() != "" })

	uc := relay.NewUseCases(p, nil)

	kiosk, err := uc.UploadAddress(su, false)
	if err != nil {
		t.Fatal(err)
	}

	home, err := uc.UploadAddress(su, true)
	if err != nil {
		t.Fatal(err)
	}

	k, h := mustParse(t, kiosk.URL), mustParse(t, home.URL)

	if k.Query().Has("m") {
		t.Fatalf("Kiosk-Adresse trägt den Eingang: %s", kiosk.URL)
	}

	if h.Query().Get("m") != "inbox" || h.Query().Get("u") != "s1" {
		t.Fatalf("Heim-Adresse = %s, erwartet m=inbox und u=s1", home.URL)
	}

	if h.Scheme != k.Scheme || h.Host != k.Host || h.Path != k.Path {
		t.Fatalf("Heim-Adresse %s weicht von %s ab", home.URL, kiosk.URL)
	}

	spec.Verified(t, upload.RUploadEingang, upload.RUploadSitzung)
}

// Die Upload-Adresse ist kurzlebig: Verwirft der Dienst die Sitzung, holt
// das Gerät eine neue, statt einen toten QR-Code weiter zu zeigen.
func TestExpiredSessionIsReplaced(t *testing.T) {
	f := newFakeRelay(t)
	p := relay.NewPoller(newSettings(relay.Options{URL: f.srv.URL, Token: token, Interval: interval}).load, (&inbox{}).deliver)
	run(t, p)

	waitFor(t, "erste Sitzung", func() bool { return p.UploadURL() != "" })

	f.mu.Lock()
	f.expire = 1
	f.mu.Unlock()

	waitFor(t, "zweite Sitzung", func() bool { return strings.HasSuffix(p.UploadURL(), "u=s2") })

	spec.Verified(t, upload.RUploadSitzung)
}

// Wer im Betrieb Adresse oder Token ändert, bekommt eine neue Sitzung ohne
// Neustart des Geräts.
func TestChangedSettingsOpenANewSession(t *testing.T) {
	first, second := newFakeRelay(t), newFakeRelay(t)
	s := newSettings(relay.Options{URL: first.srv.URL, Token: token, Interval: interval})
	p := relay.NewPoller(s.load, (&inbox{}).deliver)
	run(t, p)

	waitFor(t, "Sitzung beim ersten Dienst", func() bool { return strings.HasPrefix(p.UploadURL(), first.srv.URL) })

	s.set(relay.Options{URL: second.srv.URL, Token: token, Interval: interval})
	waitFor(t, "Sitzung beim zweiten Dienst", func() bool { return strings.HasPrefix(p.UploadURL(), second.srv.URL) })

	// Ohne Dienst verschwindet die Adresse sofort wieder.
	s.set(relay.Options{Interval: interval})
	waitFor(t, "keine Adresse", func() bool { return p.UploadURL() == "" && p.State() == relay.StateOff })

	spec.Verified(t, upload.RUploadSitzung)
}

// Ein Upload in den Eingang hat keine Vorlage; ein Kiosk-Upload trägt das
// Layout, das der Gast gewählt hat. Beide kommen genau einmal an, mit genau
// den hochgeladenen Bytes, und werden danach beim Dienst bestätigt.
func TestJobsAreDeliveredOnceAndAcknowledged(t *testing.T) {
	f := newFakeRelay(t)
	homeData, kioskData := []byte("\xff\xd8 eingang"), []byte("\xff\xd8 kiosk")
	f.add(relay.Job{ID: "a", Filename: "IMG_1.jpg"}, homeData)
	f.add(relay.Job{ID: "b", Template: "polaroid", Filename: "IMG_2.jpg"}, kioskData)

	in := &inbox{}
	p := relay.NewPoller(newSettings(relay.Options{URL: f.srv.URL, Token: token, Interval: interval}).load, in.deliver)
	run(t, p)

	waitFor(t, "alle Aufträge bestätigt", func() bool { pending, _, _ := f.snapshot(); return pending == 0 })
	settle()

	calls := in.snapshot()
	if len(calls) != 2 {
		t.Fatalf("%d Übergaben, erwartet 2", len(calls))
	}

	for _, c := range calls {
		switch c.job.ID {
		case "a":
			if c.job.Template != "" || c.job.Filename != "IMG_1.jpg" || !sameBytes(c.data, homeData) {
				t.Errorf("Eingangs-Upload = %+v", c.job)
			}
		case "b":
			if c.job.Template != "polaroid" || c.job.Filename != "IMG_2.jpg" || !sameBytes(c.data, kioskData) {
				t.Errorf("Kiosk-Upload = %+v", c.job)
			}
		default:
			t.Errorf("unerwarteter Auftrag %q", c.job.ID)
		}
	}

	if _, _, acks := f.snapshot(); acks["a"] != 1 || acks["b"] != 1 {
		t.Fatalf("Bestätigungen = %v, erwartet je eine", acks)
	}

	spec.Verified(t, upload.RUploadEingang)
}

// Scheitert die Übernahme am Gerät, bleibt der Auftrag beim Dienst liegen
// und kommt im nächsten Durchlauf wieder. Bestätigt wird erst, was wirklich
// angekommen ist – sonst wäre das Bild eines Gastes verloren.
func TestFailedDeliveryIsRetriedAndNotAcknowledged(t *testing.T) {
	f := newFakeRelay(t)
	f.add(relay.Job{ID: "b", Filename: "IMG_2.jpg"}, []byte("bild"))

	in := &inbox{fail: map[string]int{"b": 2}}
	p := relay.NewPoller(newSettings(relay.Options{URL: f.srv.URL, Token: token, Interval: interval}).load, in.deliver)
	run(t, p)

	waitFor(t, "Auftrag bestätigt", func() bool { pending, _, _ := f.snapshot(); return pending == 0 })
	settle()

	ok, all := countOK(in.snapshot(), "b")
	if ok != 1 || all != 3 {
		t.Fatalf("Übergaben: %d erfolgreich von %d, erwartet 1 von 3", ok, all)
	}

	// Die gescheiterten Versuche wurden nicht bestätigt.
	if _, _, acks := f.snapshot(); acks["b"] != 1 {
		t.Fatalf("%d Bestätigungen, erwartet genau eine nach dem Erfolg", acks["b"])
	}

	// Ein gescheiterter Auftrag macht die Sitzung nicht ungültig.
	if _, sessions, _ := f.snapshot(); sessions != 1 {
		t.Fatalf("%d Sitzungen, erwartet 1", sessions)
	}
}

// Kam das Bild an, scheiterte aber die Bestätigung, steht der Auftrag beim
// nächsten Mal wieder in der Liste. Er darf dann nicht ein zweites Mal im
// Eingang landen – oder, im Kiosk, ein zweites Mal gedruckt werden.
func TestAcknowledgeFailureDoesNotDeliverTwice(t *testing.T) {
	f := newFakeRelay(t)
	f.add(relay.Job{ID: "a", Template: "polaroid"}, []byte("bild"))

	f.mu.Lock()
	f.ackFails["a"] = 2
	f.mu.Unlock()

	in := &inbox{}
	p := relay.NewPoller(newSettings(relay.Options{URL: f.srv.URL, Token: token, Interval: interval}).load, in.deliver)
	run(t, p)

	waitFor(t, "Auftrag bestätigt", func() bool { pending, _, _ := f.snapshot(); return pending == 0 })
	settle()

	if ok, all := countOK(in.snapshot(), "a"); ok != 1 || all != 1 {
		t.Fatalf("Übergaben: %d erfolgreich von %d, erwartet genau eine", ok, all)
	}

	if _, _, acks := f.snapshot(); acks["a"] != 3 {
		t.Fatalf("%d Bestätigungsversuche, erwartet 3", acks["a"])
	}

	// Nach dem gescheiterten Bestätigen gilt die Sitzung weiter.
	if p.State() != relay.StateReady {
		t.Fatalf("State = %d", p.State())
	}
}
