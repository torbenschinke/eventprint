package printing

import (
	"context"
	"time"

	"go.wdy.de/nago/auth"
)

// diagnoseTimeout begrenzt die Abfrage, damit ein hängendes lpstat nicht die
// Oberfläche blockiert. Die Druckstatus-Seite zeichnet sich zyklisch neu.
const diagnoseTimeout = 3 * time.Second

// Diagnose beschreibt den Zustand des Druckers.
//
// Die Druckstatus-Seite zeigt ihn an, damit die häufigsten Ursachen ohne
// Terminal erkennbar sind: Warteschlange gelöscht, Drucker angehalten, kein
// Papier.
type Diagnose func(subject auth.Subject) (PrinterStatus, error)

// NewDiagnose erzeugt den [Diagnose] Anwendungsfall.
//
// Kann der Drucker keine Auskunft geben, wird das ausdrücklich als solches
// gemeldet und nicht als "alles in Ordnung" ausgegeben – eine falsche
// Entwarnung wäre schlimmer als gar keine Aussage.
func NewDiagnose(ctx context.Context, printer Printer) Diagnose {
	return func(subject auth.Subject) (PrinterStatus, error) {
		// Zwei verschiedene Dinge, deshalb zwei Wege: "Du darfst nicht fragen"
		// ist ein Fehler des Aufrufs und kommt als error zurück. "Der Drucker
		// hat nicht geantwortet" ist ein Befund über den Drucker und steht in
		// PrinterStatus.Err, weil die Oberfläche ihn anzeigen soll.
		if err := subject.Audit(PermDiagnose); err != nil {
			return PrinterStatus{Queue: printer.Name()}, err
		}

		tracker, ok := printer.(Tracker)
		if !ok {
			return PrinterStatus{Queue: printer.Name(), Exists: true, Enabled: true, Accepting: true}, nil
		}

		ctx, cancel := context.WithTimeout(ctx, diagnoseTimeout)
		defer cancel()

		return tracker.Status(ctx), nil
	}
}
