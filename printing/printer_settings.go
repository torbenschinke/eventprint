package printing

import (
	"context"
)

// TestModeName ist der Anzeigename, solange keine Warteschlange gewählt ist.
const TestModeName = "Testmodus (kein Drucker)"

// LoadSettings liefert die aktuell gültigen Druckereinstellungen.
type LoadSettings func() Settings

// settingsPrinter leitet jeden Auftrag an das Ziel weiter, das gerade in den
// Einstellungen steht.
//
// Die Einstellung wird bewusst pro Auftrag gelesen und nicht beim Start
// zwischengespeichert: Wer die Fotobox während der Veranstaltung umkonfiguriert
// – etwa weil der Drucker gewechselt wurde – soll das Ergebnis sofort sehen,
// ohne die laufende Anwendung neu starten zu müssen.
type settingsPrinter struct {
	load LoadSettings
}

// NewSettingsPrinter erzeugt einen Drucker, der seiner Konfiguration folgt.
func NewSettingsPrinter(load LoadSettings) Printer {
	return settingsPrinter{load: load}
}

func (p settingsPrinter) Name() string {
	return p.target().Name()
}

func (p settingsPrinter) Print(ctx context.Context, jpg []byte, name string) (Result, error) {
	return p.target().Print(ctx, jpg, name)
}

// Await verfolgt den Auftrag über denselben Kanal, an den er übergeben wurde.
func (p settingsPrinter) Await(ctx context.Context, jobID string) Outcome {
	target, ok := p.target().(Tracker)
	if !ok {
		return Outcome{Done: true, Success: true}
	}

	return target.Await(ctx, jobID)
}

// Status beschreibt den Zustand des aktuell eingestellten Druckers.
func (p settingsPrinter) Status(ctx context.Context) PrinterStatus {
	target, ok := p.target().(Tracker)
	if !ok {
		return PrinterStatus{Queue: p.Name(), Exists: true, Enabled: true, Accepting: true}
	}

	return target.Status(ctx)
}

// target wählt anhand der Einstellungen den konkreten Ausgabekanal.
func (p settingsPrinter) target() Printer {
	if p.load == nil {
		return DiscardPrinter{}
	}

	cfg := p.load()
	if cfg.TestMode() {
		return DiscardPrinter{}
	}

	return CUPSPrinter{
		Queue:      cfg.Queue,
		PageSize:   cfg.PageSize,
		Laminate:   cfg.Laminate,
		PrintSpeed: cfg.Speed(),
	}
}

// TestPrinter wird von Ausgabekanälen implementiert, die nicht wirklich
// drucken. Die Oberfläche kann so ohne Namensvergleich erkennen, dass die
// Fotobox noch eingerichtet werden muss.
type TestPrinter interface {
	TestMode() bool
}

// TestMode meldet der Oberfläche, dass nur simuliert wird.
func (DiscardPrinter) TestMode() bool { return true }

// TestMode folgt der aktuellen Einstellung.
func (p settingsPrinter) TestMode() bool {
	t, ok := p.target().(TestPrinter)
	return ok && t.TestMode()
}

// IsTestMode meldet, ob der übergebene Drucker nur simuliert.
func IsTestMode(p Printer) bool {
	t, ok := p.(TestPrinter)
	return ok && t.TestMode()
}
