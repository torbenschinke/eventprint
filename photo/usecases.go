package photo

import (
	"io"
	"iter"
	"sync"

	"github.com/worldiety/option"
	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/events"
)

// Options steuern den Import eines Fotos.
type Options struct {
	// Source gibt an, woher das Bild stammt. Leer bedeutet [SourceUpload].
	Source Source
}

// Import legt ein neues Foto an. Die Bilddaten werden dabei in das
// Image-Subsystem übernommen, welches automatisch skalierte Varianten
// erzeugt. Das Original bleibt unverändert erhalten und wird für den Druck
// verwendet.
type Import func(subject auth.Subject, opts Options, file image.File) (Photo, error)

// FindByID liefert ein einzelnes Foto anhand seiner ID.
type FindByID func(subject auth.Subject, id ID) (option.Opt[Photo], error)

// FindAll liefert alle Fotos, beginnend mit dem neuesten.
type FindAll func(subject auth.Subject) iter.Seq2[Photo, error]

// FindLatest liefert die neuesten max Fotos, beginnend mit dem neuesten.
type FindLatest func(subject auth.Subject, max int) ([]Photo, error)

// Delete entfernt ein Foto aus der Historie.
type Delete func(subject auth.Subject, id ID) error

// OpenOriginal öffnet die unveränderten Originaldaten eines Fotos, so wie sie
// von Kamera oder Smartphone geliefert wurden.
type OpenOriginal func(subject auth.Subject, id ID) (option.Opt[io.ReadCloser], error)

// UseCases bündelt alle Anwendungsfälle rund um Fotos.
type UseCases struct {
	Import       Import
	FindByID     FindByID
	FindAll      FindAll
	FindLatest   FindLatest
	Delete       Delete
	OpenOriginal OpenOriginal
}

// NewUseCases verdrahtet die Anwendungsfälle mit ihren Abhängigkeiten.
func NewUseCases(bus events.Bus, repo Repository, images image.UseCases) UseCases {
	var mutex sync.Mutex

	findByID := NewFindByID(repo)
	findAll := NewFindAll(repo)

	return UseCases{
		Import:       NewImport(&mutex, bus, repo, images.CreateSrcSet),
		FindByID:     findByID,
		FindAll:      findAll,
		FindLatest:   NewFindLatest(findAll),
		Delete:       NewDelete(&mutex, bus, repo),
		OpenOriginal: NewOpenOriginal(findByID, images.OpenReader),
	}
}
