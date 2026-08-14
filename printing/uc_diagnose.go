package printing

import (
	"context"
	"time"

	"go.wdy.de/nago/auth"
)

// diagnoseTimeout begrenzt die Abfrage, damit ein hängendes lpstat nicht die
// Oberfläche blockiert. Die Druckstatus-Seite zeichnet sich zyklisch neu.
const diagnoseTimeout = 3 * time.Second

// NewDiagnose erzeugt den [Diagnose] Anwendungsfall.
//
// Kann der Drucker keine Auskunft geben, wird das ausdrücklich als solches
// gemeldet und nicht als "alles in Ordnung" ausgegeben – eine falsche
// Entwarnung wäre schlimmer als gar keine Aussage.
func NewDiagnose(ctx context.Context, printer Printer) Diagnose {
	return func(subject auth.Subject) PrinterStatus {
		if err := subject.Audit(PermFindAllJobs); err != nil {
			return PrinterStatus{Queue: printer.Name(), Err: err}
		}

		tracker, ok := printer.(Tracker)
		if !ok {
			return PrinterStatus{Queue: printer.Name(), Exists: true, Enabled: true, Accepting: true}
		}

		ctx, cancel := context.WithTimeout(ctx, diagnoseTimeout)
		defer cancel()

		return tracker.Status(ctx)
	}
}
