package device

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
	"github.com/torbenschinke/eventprint/requirements/fun/druck"
	"github.com/torbenschinke/eventprint/requirements/fun/foto"
)

// intakeSpy ersetzt Ablage und Drucker. Intake entscheidet nur, was mit einem
// Bild geschieht; ob Import und Druck selbst funktionieren, prüfen ihre
// eigenen Kontexte.
type intakeSpy struct {
	mu      sync.Mutex
	imports []photo.ImportCmd
	prints  []printing.SimpleCmd
}

func (s *intakeSpy) importPhoto(_ permission.Auditable, cmd photo.ImportCmd) (photo.Photo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.imports = append(s.imports, cmd)

	return photo.Photo{
		ID: photo.ID(fmt.Sprintf("p%d", len(s.imports))), Name: cmd.Name, Source: cmd.Source,
		Event: cmd.Event, Unseen: cmd.Unseen,
	}, nil
}

func (s *intakeSpy) print(_ permission.Auditable, cmd printing.SimpleCmd) (printing.Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.prints = append(s.prints, cmd)

	return printing.Batch{ID: "b"}, nil
}

func (s *intakeSpy) printed() []printing.SimpleCmd {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]printing.SimpleCmd(nil), s.prints...)
}

func (s *intakeSpy) lastImport() photo.ImportCmd {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.imports[len(s.imports)-1]
}

// TestIntakeHome prüft den Heimbetrieb: Alles landet im Eingang, nichts wird
// von allein gedruckt – auch nicht, wenn die Kiosk-Einstellungen das
// sofortige Drucken erlauben.
func TestIntakeHome(t *testing.T) {
	f := newFixture(t)
	spy := &intakeSpy{}
	intake := NewIntake(f.settings, f.kiosk, spy.importPhoto, spy.print)

	for _, src := range []photo.Source{photo.SourceCamera, photo.SourceRelay} {
		res, err := intake(permission.SU(), IntakeCmd{Name: "a.jpg", Source: src, Data: []byte("jpeg"), Template: printing.TemplateFull})
		if err != nil {
			t.Fatal(err)
		}

		cmd := spy.lastImport()
		if !cmd.Unseen || cmd.Event != "" || cmd.Source != src || cmd.Name != "a.jpg" || string(cmd.Data) != "jpeg" {
			t.Fatalf("%s: home intake must import unseen and private, got %+v", src, cmd)
		}

		if res.Printed || !res.Photo.Unseen {
			t.Fatalf("%s: home intake must not print, got %+v", src, res)
		}
	}

	if p := spy.printed(); len(p) != 0 {
		t.Fatalf("home mode must never print, got %+v", p)
	}

	// Ohne Berechtigung nimmt niemand Bilder an.
	var denied PermissionDeniedError
	if _, err := intake(guest(), IntakeCmd{Source: photo.SourceRelay}); !errors.As(err, &denied) {
		t.Fatalf("intake without permission must be denied, got %v", err)
	}

	spec.Verified(t, foto.RFotoEingang)
}

// TestIntakeKiosk prüft den Kiosk: Bilder gehören zur laufenden Feier, und
// Kamera und Handy folgen je ihrer eigenen Einstellung.
func TestIntakeKiosk(t *testing.T) {
	f := newFixture(t)
	save := NewSaveSettings(f.mutex, f.settings)

	if _, err := save(owner(), func(s *Settings) {
		s.KioskLayouts = []printing.TemplateID{printing.TemplateFull, printing.TemplatePassepartout}
		s.KioskDefault = printing.TemplatePassepartout
		s.PrintCamera = true
		s.PrintUploads = true
	}); err != nil {
		t.Fatal(err)
	}

	k, err := f.startKiosk()(owner(), StartKioskCmd{Title: "Sommerfest"})
	if err != nil {
		t.Fatal(err)
	}

	spy := &intakeSpy{}
	intake := NewIntake(f.settings, f.kiosk, spy.importPhoto, spy.print)

	cases := []struct {
		name string
		cmd  IntakeCmd
		want printing.TemplateID
	}{
		// Der Gast hat auf dem Handy ein freigegebenes Layout gewählt.
		{"chosen layout", IntakeCmd{Source: photo.SourceRelay, Template: printing.TemplateFull}, printing.TemplateFull},
		// Ein Layout, das die Betreuung nicht freigegeben hat, wird nicht
		// gedruckt, nur weil jemand die Anfrage von Hand baut.
		{"layout not allowed", IntakeCmd{Source: photo.SourceRelay, Template: printing.TemplatePolaroid}, printing.TemplatePassepartout},
		// Die Kamera wählt nie; sie bekommt den Standard.
		{"camera default", IntakeCmd{Source: photo.SourceCamera}, printing.TemplatePassepartout},
	}

	for _, tc := range cases {
		before := len(spy.printed())

		res, err := intake(permission.SU(), tc.cmd)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		cmd := spy.lastImport()
		if cmd.Event != k.Event || cmd.Unseen {
			t.Fatalf("%s: kiosk intake must file into the event, got %+v", tc.name, cmd)
		}

		p := spy.printed()
		if !res.Printed || len(p) != before+1 {
			t.Fatalf("%s: must print immediately, res %+v, prints %+v", tc.name, res, p)
		}

		if got := p[len(p)-1]; got.Template != tc.want || got.Photo != res.Photo.ID || got.Copies != 1 {
			t.Fatalf("%s: printed %+v, want template %s for %s", tc.name, got, tc.want, res.Photo.ID)
		}
	}

	// Handy-Uploads aus, Kamera an: nur die Kamera druckt.
	if _, err := save(owner(), func(s *Settings) { s.PrintUploads = false; s.PrintCamera = true }); err != nil {
		t.Fatal(err)
	}

	assertIntake(t, intake, spy, k.Event, photo.SourceRelay, false)
	assertIntake(t, intake, spy, k.Event, photo.SourceCamera, true)

	// Umgekehrt: nur das Handy druckt.
	if _, err := save(owner(), func(s *Settings) { s.PrintUploads = true; s.PrintCamera = false }); err != nil {
		t.Fatal(err)
	}

	assertIntake(t, intake, spy, k.Event, photo.SourceRelay, true)
	assertIntake(t, intake, spy, k.Event, photo.SourceCamera, false)

	// Beides aus: Die Bilder gehören trotzdem zur Feier, gedruckt wird nichts.
	if _, err := save(owner(), func(s *Settings) { s.PrintUploads = false; s.PrintCamera = false }); err != nil {
		t.Fatal(err)
	}

	assertIntake(t, intake, spy, k.Event, photo.SourceRelay, false)
	assertIntake(t, intake, spy, k.Event, photo.SourceCamera, false)

	spec.Verified(t, druck.RDruckKiosk)
}

func assertIntake(t *testing.T, intake Intake, spy *intakeSpy, event photo.EventID, src photo.Source, wantPrint bool) {
	t.Helper()

	before := len(spy.printed())

	res, err := intake(permission.SU(), IntakeCmd{Source: src})
	if err != nil {
		t.Fatal(err)
	}

	if cmd := spy.lastImport(); cmd.Event != event || cmd.Unseen {
		t.Fatalf("%s: must be filed into event %s, got %+v", src, event, cmd)
	}

	printed := len(spy.printed()) - before
	if res.Printed != wantPrint || printed != map[bool]int{false: 0, true: 1}[wantPrint] {
		t.Fatalf("%s: printed=%v (%d jobs), want %v", src, res.Printed, printed, wantPrint)
	}
}

// TestIntakePrintFailure prüft, dass ein Druckfehler das Foto nicht verliert:
// Es ist schon in der Feier abgelegt, und der Gast kann es von Hand drucken.
func TestIntakePrintFailure(t *testing.T) {
	f := newFixture(t)

	if _, err := f.startKiosk()(owner(), StartKioskCmd{}); err != nil {
		t.Fatal(err)
	}

	spy := &intakeSpy{}
	broken := func(permission.Auditable, printing.SimpleCmd) (printing.Batch, error) {
		return printing.Batch{}, errors.New("kein Drucker")
	}

	res, err := NewIntake(f.settings, f.kiosk, spy.importPhoto, broken)(permission.SU(), IntakeCmd{Source: photo.SourceCamera})
	if err == nil {
		t.Fatal("a print failure must be reported")
	}

	if res.Printed || res.Photo.ID == "" {
		t.Fatalf("photo must be returned unprinted, got %+v", res)
	}

	spec.Verified(t, druck.RDruckKiosk)
}
