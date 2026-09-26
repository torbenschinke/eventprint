package printing

import (
	"context"
	"sync"

	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/photo"
)

// SimpleCmd beschreibt einen Druck im Kiosk: ein Foto, eines der drei
// Kiosk-Layouts, wenige Exemplare.
type SimpleCmd struct {
	Photo    photo.ID
	Template TemplateID
	Copies   int
}

// PrintSimple druckt ein Foto in einem Kiosk-Layout.
//
// Gäste haben nur diese Berechtigung und nicht [Print]. Das Druck-Studio mit
// seinen Formaten ist dem Besitzer vorbehalten; vor allem aber begrenzt
// dieser Weg die Exemplare, damit ein Gast nicht das Papier der ganzen Feier
// verbraucht.
type PrintSimple func(subject permission.Auditable, cmd SimpleCmd) (Batch, error)

// NewPrintSimple erzeugt den [PrintSimple] Anwendungsfall.
//
// maxCopies wird bei jedem Aufruf gelesen, damit eine geänderte Einstellung
// sofort gilt.
func NewPrintSimple(ctx context.Context, mutex *sync.Mutex, repo Repository, printer Printer, queue chan<- JobID, maxCopies func() int, visible func(permission.Auditable, []photo.ID) error) PrintSimple {
	return func(subject permission.Auditable, cmd SimpleCmd) (Batch, error) {
		if err := subject.Audit(PermPrintSimple); err != nil {
			return Batch{}, err
		}

		if err := visible(subject, []photo.ID{cmd.Photo}); err != nil {
			return Batch{}, err
		}

		limit := 1
		if maxCopies != nil {
			limit = max(1, maxCopies())
		}

		return enqueueBatch(ctx, mutex, repo, printer, queue, PrintCmd{
			Photos: []photo.ID{cmd.Photo},
			Layout: TemplateByID(cmd.Template).ID.Layout(),
			Copies: min(max(cmd.Copies, 1), limit),
		})
	}
}
