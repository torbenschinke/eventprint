package printing

import (
	"context"
	"iter"
	"sync"
	"time"

	"github.com/worldiety/option"
	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/events"
	"go.wdy.de/nago/pkg/std"

	"github.com/torbenschinke/eventprint/photo"
)

// Print stellt ein Foto mit dem gewählten Layout in die Druckwarteschlange
// und liefert die ID des angelegten Auftrags zurück. Der Aufruf kehrt sofort
// zurück, gedruckt wird asynchron.
type Print func(subject auth.Subject, id photo.ID, tpl TemplateID) (JobID, error)

// FindAllJobs liefert alle Druckaufträge, beginnend mit dem neuesten.
type FindAllJobs func(subject auth.Subject) iter.Seq2[Job, error]

// FindJobByID liefert einen einzelnen Druckauftrag.
type FindJobByID func(subject auth.Subject, id JobID) (option.Opt[Job], error)

// Retry stellt einen fehlgeschlagenen Auftrag erneut in die Warteschlange.
type Retry func(subject auth.Subject, id JobID) error

// Preview rendert ein Foto mit dem gewählten Layout, ohne es zu drucken.
// Damit kann die Oberfläche zeigen, wie der Ausdruck aussehen wird.
type Preview func(subject auth.Subject, id photo.ID, tpl TemplateID) ([]byte, error)

// Diagnose beschreibt den Zustand des Druckers.
//
// Die Druckstatus-Seite zeigt ihn an, damit die häufigsten Ursachen ohne
// Terminal erkennbar sind: Warteschlange gelöscht, Drucker angehalten, kein
// Papier.
type Diagnose func(subject auth.Subject) PrinterStatus

// UseCases bündelt alle Anwendungsfälle rund um das Drucken.
type UseCases struct {
	Print       Print
	Preview     Preview
	FindAllJobs FindAllJobs
	FindJobByID FindJobByID
	Retry       Retry
	Diagnose    Diagnose

	// Printer ist der konfigurierte Ausgabekanal, damit die Oberfläche das
	// Ziel anzeigen kann.
	Printer Printer
}

// enqueueTimeout begrenzt das Warten auf einen freien Platz in der
// Warteschlange.
//
// Ein unbegrenztes Warten wäre gefährlich: Am Kanal hängen nicht nur Klicks
// der Oberfläche, sondern auch die Schleifen für Kamera und Fern-Uploads. Ein
// festsitzender Worker würde sie alle stillstehen lassen, ohne dass irgendwo
// eine Meldung erschiene.
const enqueueTimeout = 5 * time.Second

// enqueue reiht eine Auftragskennung ein und gibt nach [enqueueTimeout] auf.
func enqueue(ctx context.Context, queue chan<- JobID, id JobID) error {
	timer := time.NewTimer(enqueueTimeout)
	defer timer.Stop()

	select {
	case queue <- id:
		return nil

	case <-ctx.Done():
		return std.NewLocalizedError("Fotobox wird beendet", "Der Druckauftrag wurde nicht mehr angenommen.")

	case <-timer.C:
		return std.NewLocalizedError("Warteschlange voll",
			"Der Drucker kommt nicht hinterher. Bitte prüfe den Druckstatus, bevor weitere Aufträge gestartet werden.")
	}
}

// NewUseCases verdrahtet die Anwendungsfälle und startet den Druck-Worker.
//
// renderOptions wird bei jedem Rendern erneut ausgewertet, damit eine
// geänderte Einstellung sofort greift. nil bedeutet: keine Bildkorrekturen.
//
// Der Worker endet, sobald ctx abgebrochen wird – also beim Herunterfahren
// der Anwendung.
func NewUseCases(ctx context.Context, bus events.Bus, repo Repository, printer Printer, openOriginal photo.OpenOriginal, renderOptions func() RenderOptions) UseCases {
	var mutex sync.Mutex

	// Der Puffer entkoppelt die Oberfläche vom Drucker. Ist er voll, wartet
	// der aufrufende Klick – das ist gewollt, denn dann stimmt etwas nicht.
	queue := make(chan JobID, 64)

	findJobByID := NewFindJobByID(repo)

	renderOptions = orDefaultRenderOptions(renderOptions)

	worker := newWorker(&mutex, bus, repo, printer, openOriginal, renderOptions)
	recoverStaleJobs(ctx, &mutex, repo, printer, queue)
	go worker.run(ctx, queue)

	return UseCases{
		Print:       NewPrint(ctx, &mutex, bus, repo, printer, queue),
		Preview:     NewPreview(openOriginal, renderOptions),
		FindAllJobs: NewFindAllJobs(repo),
		FindJobByID: findJobByID,
		Retry:       NewRetry(ctx, &mutex, repo, printer, findJobByID, queue),
		Diagnose:    NewDiagnose(ctx, printer),
		Printer:     printer,
	}
}
