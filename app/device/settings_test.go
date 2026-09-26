package device

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/app/printing"
	"github.com/torbenschinke/eventprint/requirements/fun/druck"
	"github.com/torbenschinke/eventprint/requirements/fun/modus"
)

// TestLoadSettingsDefaults prüft das frisch eingerichtete Gerät: Ohne Datei
// gibt es Standardwerte statt eines Fehlers, sonst stünde beim ersten
// Einschalten ein leerer Bildschirm da.
func TestLoadSettingsDefaults(t *testing.T) {
	f := newFixture(t)

	s, err := NewLoadSettings(f.settings)(owner())
	if err != nil {
		t.Fatal(err)
	}

	d := DefaultSettings()
	if s.Accent != d.Accent || s.KioskDefault != d.KioskDefault || s.PaperLeft != d.PaperLeft || !slices.Equal(s.KioskLayouts, d.KioskLayouts) {
		t.Fatalf("missing file must load defaults, got %+v", s)
	}

	if _, err := os.Stat(f.settings.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("loading must not create the file, stat: %v", err)
	}

	// Gäste dürfen die Einstellungen nicht lesen: Darin stehen Zugangstoken
	// und PIN.
	var denied PermissionDeniedError
	if _, err := NewLoadSettings(f.settings)(guest()); !errors.As(err, &denied) {
		t.Fatalf("guest must not load settings, got %v", err)
	}

	spec.Verified(t, modus.RModusEinstellungen)
}

// TestNormalized prüft, dass eine halb ausgefüllte oder ältere Datei nie zu
// einem unbenutzbaren Gerät führt.
func TestNormalized(t *testing.T) {
	d := DefaultSettings()

	n := Settings{MaxCopies: -1, PaperCapacity: 0}.Normalized()
	if n.Accent != d.Accent || n.KioskDefault != d.KioskDefault || n.MaxCopies != d.MaxCopies ||
		n.PaperCapacity != d.PaperCapacity || !slices.Equal(n.KioskLayouts, d.KioskLayouts) {
		t.Fatalf("defaults not filled: %+v", n)
	}

	// Was gesetzt ist, bleibt – auch wenn es vom Standard abweicht.
	custom := Settings{
		Accent: "#000000", KioskLayouts: []printing.TemplateID{printing.TemplateFull},
		KioskDefault: printing.TemplateFull, MaxCopies: 5, PaperCapacity: 18,
	}

	n = custom.Normalized()
	if n.Accent != "#000000" || n.KioskDefault != printing.TemplateFull || n.MaxCopies != 5 || n.PaperCapacity != 18 ||
		!slices.Equal(n.KioskLayouts, custom.KioskLayouts) {
		t.Fatalf("set values must survive: %+v", n)
	}

	spec.Verified(t, modus.RModusEinstellungen)
}

// TestSaveSettings prüft das Speichern über eine Änderungsfunktion: Was fn
// ändert, landet in der Datei; PIN und Feierliste bleiben unangetastet, auch
// wenn eine Oberfläche sie mit einem alten Stand überschreiben wollte.
func TestSaveSettings(t *testing.T) {
	f := newFixture(t)

	if err := NewSetPin(f.mutex, f.settings)(owner(), "135790"); err != nil {
		t.Fatal(err)
	}

	k, err := f.startKiosk()(owner(), StartKioskCmd{Title: "Geburtstag"})
	if err != nil {
		t.Fatal(err)
	}

	save := NewSaveSettings(f.mutex, f.settings)

	var denied PermissionDeniedError
	if _, err := save(guest(), func(s *Settings) { s.Accent = "#FF0000" }); !errors.As(err, &denied) {
		t.Fatalf("guest must not save settings, got %v", err)
	}

	got, err := save(owner(), func(s *Settings) {
		s.Accent = "#123456"
		s.PrintCamera = false
		s.RelayURL = "https://relay.example"
		s.KioskLayouts = nil // halb ausgefülltes Formular
		s.Pin = PinHash{}
		s.Events = nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.Accent != "#123456" || got.PrintCamera || got.RelayURL != "https://relay.example" {
		t.Fatalf("changes not applied: %+v", got)
	}

	if len(got.KioskLayouts) == 0 {
		t.Fatal("saved settings must be normalized")
	}

	if !got.Pin.Matches("135790") {
		t.Fatal("SaveSettings must keep the PIN")
	}

	if len(got.Events) != 1 || got.Events[0].ID != k.Event {
		t.Fatalf("SaveSettings must keep the events, got %+v", got.Events)
	}

	// Ein neuer Speicher auf derselben Datei – wie nach einem Neustart – sieht
	// denselben Stand.
	reloaded, err := NewLoadSettings(NewFileSettings(f.settings.path))(owner())
	if err != nil {
		t.Fatal(err)
	}

	if reloaded.Accent != "#123456" || reloaded.PrintCamera || !reloaded.Pin.Matches("135790") || len(reloaded.Events) != 1 {
		t.Fatalf("settings not persisted: %+v", reloaded)
	}

	// Die Datei ist gültiges JSON, und die Zwischendatei des atomaren
	// Schreibens bleibt nicht liegen.
	buf, err := os.ReadFile(f.settings.path)
	if err != nil {
		t.Fatal(err)
	}

	if !json.Valid(buf) {
		t.Fatalf("settings file is not JSON:\n%s", buf)
	}

	if _, err := os.Stat(f.settings.path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp file must be renamed away, stat: %v", err)
	}

	// Eine kaputte Datei ist ein Fehler und wird nicht still durch
	// Standardwerte ersetzt – sonst wäre beim nächsten Speichern alles weg.
	if err := os.WriteFile(f.settings.path, []byte("{kaputt"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := save(owner(), func(s *Settings) { s.Accent = "#000000" }); err == nil {
		t.Fatal("a corrupt settings file must not be overwritten")
	}

	spec.Verified(t, modus.RModusEinstellungen)
}

// TestPaper prüft den selbst gezählten Papiervorrat.
func TestPaper(t *testing.T) {
	f := newFixture(t)

	if _, err := NewSaveSettings(f.mutex, f.settings)(owner(), func(s *Settings) { s.PaperCapacity = 18 }); err != nil {
		t.Fatal(err)
	}

	consume := NewConsumePaper(f.mutex, f.settings)
	refill := NewRefillPaper(f.mutex, f.settings)

	s, err := refill(owner())
	if err != nil {
		t.Fatal(err)
	}

	if s.PaperLeft != 18 {
		t.Fatalf("refill must set PaperLeft to the capacity, got %d", s.PaperLeft)
	}

	if err := consume(owner(), 5); err != nil {
		t.Fatal(err)
	}

	s, _ = f.settings.Load()
	if s.PaperLeft != 13 {
		t.Fatalf("after 5 sheets want 13, got %d", s.PaperLeft)
	}

	// Wer das Nachlegen nicht meldet, druckt weiter. Eine negative Zahl sähe
	// nach einem Defekt aus.
	if err := consume(owner(), 20); err != nil {
		t.Fatal(err)
	}

	s, _ = f.settings.Load()
	if s.PaperLeft != 0 {
		t.Fatalf("paper must not go below 0, got %d", s.PaperLeft)
	}

	s, err = refill(owner())
	if err != nil {
		t.Fatal(err)
	}

	if s.PaperLeft != 18 {
		t.Fatalf("refill after empty want 18, got %d", s.PaperLeft)
	}

	var denied PermissionDeniedError
	if _, err := refill(guest()); !errors.As(err, &denied) {
		t.Fatalf("guest must not refill, got %v", err)
	}

	spec.Verified(t, druck.RDruckPapier)
}
