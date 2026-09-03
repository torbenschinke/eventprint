package photo

import (
	"sync"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/pkg/events"
)

// Options steuern den Import eines Fotos.
type Options struct {
	// Source gibt an, woher das Bild stammt. Leer bedeutet [SourceUpload].
	Source Source
}

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
//
// archive sichert jedes eingehende Bild zusätzlich unverändert als Datei.
// nil schaltet die Sicherung ab.
func NewUseCases(bus events.Bus, repo Repository, images image.UseCases, archive Archive) UseCases {
	var mutex sync.Mutex

	findByID := NewFindByID(repo)
	findAll := NewFindAll(repo)

	return UseCases{
		Import:       NewImport(&mutex, bus, repo, images.CreateSrcSet, archive),
		FindByID:     findByID,
		FindAll:      findAll,
		FindLatest:   NewFindLatest(findAll),
		Delete:       NewDelete(&mutex, bus, repo),
		OpenOriginal: NewOpenOriginal(findByID, images.OpenReader),
	}
}
