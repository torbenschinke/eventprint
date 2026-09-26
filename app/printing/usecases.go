package printing

import (
	"context"
	"sync"
	"time"

	"go.wdy.de/nago/pkg/std"

	"github.com/torbenschinke/eventprint/app/photo"
)

// UseCases bündelt alle Anwendungsfälle rund um das Drucken.
type UseCases struct {
	Print       Print
	PrintSimple PrintSimple
	Preview     Preview
	FindAllJobs FindAllJobs
	FindJobByID FindJobByID
	Retry       Retry
	Cancel      Cancel
	Diagnose    Diagnose
	Resume      Resume

	// Printer ist der konfigurierte Ausgabekanal, damit die Oberfläche das
	// Ziel anzeigen kann.
	Printer Printer
}

// Options sind die Abhängigkeiten der Druck-Anwendungsfälle.
type Options struct {
	Repository Repository
	Printer    Printer

	// Locate liest die Originale der Motive.
	Locate photo.Locate

	// RenderOptions wird bei jedem Rendern erneut ausgewertet, damit eine
	// geänderte Einstellung sofort greift. Nil bedeutet: keine Korrekturen.
	RenderOptions func() RenderOptions

	// MaxKioskCopies begrenzt die Exemplare eines Gastes.
	MaxKioskCopies func() int

	// Observe erfährt jeden abgeschlossenen Auftrag, etwa um Papier zu zählen.
	Observe func(Job)

	// DecodeJPEGScaled dekodiert JPEGs für die Vorschau verkleinert. Nil
	// dekodiert voll und verkleinert danach – richtig, nur langsamer.
	DecodeJPEGScaled ScaledJPEGDecoder
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
// Der Worker endet, sobald ctx abgebrochen wird – also beim Herunterfahren.
func NewUseCases(ctx context.Context, opts Options) UseCases {
	var mutex sync.Mutex

	// Der Puffer entkoppelt die Oberfläche vom Drucker. Ist er voll, wartet
	// der aufrufende Tipp – das ist gewollt, denn dann stimmt etwas nicht.
	queue := make(chan JobID, 256)

	repo, printer := opts.Repository, opts.Printer
	findJobByID := NewFindJobByID(repo)
	renderOptions := orDefaultRenderOptions(opts.RenderOptions)
	motifs := motifLoader(opts.Locate, opts.DecodeJPEGScaled)
	previews := newPreviewSources(previewCacheSize)
	visible := visibleTo(opts.Locate)

	worker := newWorker(&mutex, repo, printer, motifs, renderOptions, opts.Observe)
	recoverStaleJobs(ctx, &mutex, repo, printer, queue)
	go worker.run(ctx, queue)
	go newResumeGuard(printer).run(ctx)

	return UseCases{
		Print:       NewPrint(ctx, &mutex, repo, printer, queue),
		PrintSimple: NewPrintSimple(ctx, &mutex, repo, printer, queue, opts.MaxKioskCopies, visible),
		Preview:     NewPreview(previewLoader(opts.Locate, opts.DecodeJPEGScaled, previews), previewRenderOptions(renderOptions, previews), visible),
		FindAllJobs: NewFindAllJobs(repo),
		FindJobByID: findJobByID,
		Retry:       NewRetry(ctx, &mutex, repo, printer, findJobByID, queue),
		Cancel:      NewCancel(ctx, &mutex, repo, printer),
		Diagnose:    NewDiagnose(ctx, printer),
		Resume:      NewResume(ctx, printer),
		Printer:     printer,
	}
}
