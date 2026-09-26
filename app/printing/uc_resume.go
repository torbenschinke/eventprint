package printing

import (
	"context"
	"log/slog"

	"go.wdy.de/nago/application/permission"
	"go.wdy.de/nago/pkg/std"
)

// Resume gibt einen angehaltenen Drucker wieder frei.
//
// Der typische Fall auf einer Feier: Papier und Farbband sind leer, CUPS hält
// die Warteschlange an, und nach dem Wechsel geht nichts mehr, bis sie
// freigegeben wird. Dieser Anwendungsfall ersetzt das "sudo cupsenable" im
// Terminal durch eine Schaltfläche.
type Resume func(subject permission.Auditable) error

// NewResume erzeugt den [Resume] Anwendungsfall.
//
// Ein Auftrag, den CUPS beim Anhalten zurückbehalten hat, wird dabei
// weitergedruckt. Das ist kein ungewollter Ausdruck: Die Fotobox verfolgt ihn
// noch und hätte ihn nach Ablauf ihrer Frist selbst storniert.
func NewResume(ctx context.Context, printer Printer) Resume {
	return func(subject permission.Auditable) error {
		if err := subject.Audit(PermResume); err != nil {
			return err
		}

		resumer, ok := printer.(Resumer)
		if !ok {
			return nil
		}

		if err := resumer.Resume(ctx); err != nil {
			slog.Error("cannot resume printer", "printer", printer.Name(), "err", err)

			return std.NewLocalizedError("Drucker nicht freigegeben",
				"CUPS hat die Freigabe abgelehnt. Im Terminal hilft 'sudo cupsenable "+printer.Name()+"'.")
		}

		slog.Info("printer resumed by operator", "printer", printer.Name())

		return nil
	}
}
