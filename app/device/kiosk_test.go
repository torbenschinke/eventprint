package device

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/app/printing"
	"github.com/torbenschinke/eventprint/requirements/fun/modus"
)

// TestHomeAfterBoot prüft die Unterscheidung zwischen Neustart der Anwendung
// und Neustart des Geräts. Beides sieht für den Prozess gleich aus; der
// Unterschied liegt allein darin, ob das Laufzeitverzeichnis noch da ist.
func TestHomeAfterBoot(t *testing.T) {
	f := newFixture(t)

	// Frisch eingeschaltet: Das tmpfs ist leer, also Heimbetrieb.
	k, err := NewCurrentKiosk(f.kiosk)(owner())
	if err != nil {
		t.Fatal(err)
	}

	if k.Active() {
		t.Fatalf("empty runtime dir must mean home mode, got %+v", k)
	}

	started, err := f.startKiosk()(owner(), StartKioskCmd{Title: "Sommerfest"})
	if err != nil {
		t.Fatal(err)
	}

	// Der Dienst stürzt ab und systemd startet ihn neu: neue Instanz, gleiche
	// Datei. Gäste dürfen danach nicht plötzlich in der Mediathek stehen.
	restarted := NewFileKiosk(filepath.Join(f.dir, "run", "kiosk.json"))
	k, err = NewCurrentKiosk(restarted)(guest())
	if err != nil {
		t.Fatal(err)
	}

	if !k.Active() || k.Event != started.Event || k.Title != "Sommerfest" {
		t.Fatalf("service restart must keep the kiosk, got %+v want %+v", k, started)
	}

	// Stecker gezogen: Das tmpfs ist leer. Die Einstellungen auf der Karte
	// sind noch da, der Kiosk nicht – zurück im Heimbetrieb.
	rebooted := NewFileKiosk(filepath.Join(t.TempDir(), "run", "kiosk.json"))
	k, err = NewCurrentKiosk(rebooted)(owner())
	if err != nil {
		t.Fatal(err)
	}

	if k.Active() {
		t.Fatalf("reboot must end the kiosk, got %+v", k)
	}

	// Clear auf einer fehlenden Datei ist kein Fehler: Wer den Kiosk nach
	// einem Neustart beendet, beendet einen Kiosk, der schon weg ist.
	if err := rebooted.Clear(); err != nil {
		t.Fatalf("clear on missing file: %v", err)
	}

	spec.Verified(t, modus.RModusHeim)
}

// TestStartKiosk prüft, was beim Start einer Feier festgehalten wird.
func TestStartKiosk(t *testing.T) {
	f := newFixture(t)

	save := NewSaveSettings(f.mutex, f.settings)
	if _, err := save(owner(), func(s *Settings) {
		s.Accent = "#112233"
		s.KioskLayouts = []printing.TemplateID{printing.TemplatePolaroid}
		s.MaxCopies = 3
	}); err != nil {
		t.Fatal(err)
	}

	// Ein Gast darf keinen Kiosk starten, auch wenn die Oberfläche den Knopf
	// versehentlich zeigte.
	var denied PermissionDeniedError
	if _, err := f.startKiosk()(guest(), StartKioskCmd{Title: "Party"}); !errors.As(err, &denied) {
		t.Fatalf("guest must be denied, got %v", err)
	}

	// Ohne Titel heißt die Feier "Fotobox" – leere Überschriften sehen im
	// Kiosk nach einem Fehler aus.
	k, err := f.startKiosk()(owner(), StartKioskCmd{Title: "   "})
	if err != nil {
		t.Fatal(err)
	}

	if k.Title != "Fotobox" {
		t.Fatalf("blank title must default to Fotobox, got %q", k.Title)
	}

	if want := "20260926-163000.000"; string(k.Event) != want {
		t.Fatalf("event id = %q, want %q (UTC timestamp)", k.Event, want)
	}

	if !k.Since.Equal(f.clock.Now()) {
		t.Fatalf("since = %v, want %v", k.Since, f.clock.Now())
	}

	// Gäste dürfen die Einstellungen nicht lesen, deshalb nimmt der Kiosk die
	// Gestaltung als Momentaufnahme mit.
	if k.Accent != "#112233" || k.MaxCopies != 3 || !slices.Equal(k.Layouts, []printing.TemplateID{printing.TemplatePolaroid}) {
		t.Fatalf("kiosk must snapshot the design, got %+v", k)
	}

	s, err := f.settings.Load()
	if err != nil {
		t.Fatal(err)
	}

	if len(s.Events) != 1 || s.Events[0].ID != k.Event || s.Events[0].Title != "Fotobox" {
		t.Fatalf("event must be recorded in settings, got %+v", s.Events)
	}

	if e, ok := s.Event(k.Event); !ok || !e.StartedAt.Equal(k.Since) {
		t.Fatalf("Settings.Event(%q) = %+v, %v", k.Event, e, ok)
	}

	// Ein zweiter Start während der Feier würde die Fotos auf zwei Feiern
	// verteilen.
	f.clock.Advance(1)
	again, err := f.startKiosk()(owner(), StartKioskCmd{Title: "Nochmal"})
	if err == nil {
		t.Fatal("second start while active must fail")
	}

	if again.Event != k.Event {
		t.Fatalf("second start must report the running kiosk, got %+v", again)
	}

	s, _ = f.settings.Load()
	if len(s.Events) != 1 {
		t.Fatalf("refused start must not record an event, got %+v", s.Events)
	}

	spec.Verified(t, modus.RModusKiosk)
}

// TestPreflight prüft die Vorabprüfung: Sie ruft alle Proben der Verdrahtung
// und fügt die PIN-Prüfung selbst an, weil nur dieser Kontext die PIN kennt.
func TestPreflight(t *testing.T) {
	f := newFixture(t)

	var seen []Settings
	probe := func(level Level) Probe {
		return func(_ context.Context, s Settings) Check {
			seen = append(seen, s)
			return Check{Title: "probe", Level: level}
		}
	}

	pf := NewPreflight(f.settings, []Probe{probe(LevelOK), probe(LevelWarning)})

	var denied PermissionDeniedError
	if _, err := pf(guest(), context.Background()); !errors.As(err, &denied) {
		t.Fatalf("guest must be denied, got %v", err)
	}

	r, err := pf(owner(), context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(seen) != 2 {
		t.Fatalf("both probes must run, ran %d", len(seen))
	}

	// Die Proben sehen normalisierte Einstellungen, damit etwa der Papier-
	// Check nie gegen eine Kapazität von null rechnet.
	if seen[0].PaperCapacity <= 0 || seen[0].KioskDefault == "" {
		t.Fatalf("probes must see normalized settings, got %+v", seen[0])
	}

	if len(r.Checks) != 3 {
		t.Fatalf("want 2 probes + PIN check, got %+v", r.Checks)
	}

	// Ohne PIN lässt sich der Kiosk nur durch Ausschalten beenden. Das ist
	// ein Hinweis, kein Hindernis.
	if pin := r.Checks[2]; pin.Level != LevelWarning {
		t.Fatalf("missing PIN must warn, got %+v", pin)
	}

	if r.Blocking() {
		t.Fatal("warnings must not block")
	}

	if err := NewSetPin(f.mutex, f.settings)(owner(), "135790"); err != nil {
		t.Fatal(err)
	}

	r, err = pf(owner(), context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if pin := r.Checks[len(r.Checks)-1]; pin.Level != LevelOK {
		t.Fatalf("configured PIN must be ok, got %+v", pin)
	}

	// Ein einziger Fehler – etwa kein Drucker – blockiert.
	r, err = NewPreflight(f.settings, []Probe{probe(LevelOK), probe(LevelError)})(owner(), context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if !r.Blocking() {
		t.Fatalf("an error must block, got %+v", r.Checks)
	}

	if (Report{}).Blocking() {
		t.Fatal("empty report must not block")
	}

	spec.Verified(t, modus.RModusKiosk)
}
