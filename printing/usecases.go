package printing

import (
	"context"
	"iter"
	"sync"

	"github.com/worldiety/option"
	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/events"

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

// NewUseCases verdrahtet die Anwendungsfälle und startet den Druck-Worker.
//
// Der Worker endet, sobald ctx abgebrochen wird – also beim Herunterfahren
// der Anwendung.
func NewUseCases(ctx context.Context, bus events.Bus, repo Repository, printer Printer, openOriginal photo.OpenOriginal) UseCases {
	var mutex sync.Mutex

	// Der Puffer entkoppelt die Oberfläche vom Drucker. Ist er voll, wartet
	// der aufrufende Klick – das ist gewollt, denn dann stimmt etwas nicht.
	queue := make(chan JobID, 64)

	findJobByID := NewFindJobByID(repo)

	worker := newWorker(&mutex, bus, repo, printer, openOriginal)
	recoverStaleJobs(&mutex, repo, queue)
	go worker.run(ctx, queue)

	return UseCases{
		Print:       NewPrint(&mutex, bus, repo, printer, queue),
		Preview:     NewPreview(openOriginal),
		FindAllJobs: NewFindAllJobs(repo),
		FindJobByID: findJobByID,
		Retry:       NewRetry(&mutex, repo, findJobByID, queue),
		Diagnose:    NewDiagnose(ctx, printer),
		Printer:     printer,
	}
}
