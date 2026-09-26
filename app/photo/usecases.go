package photo

import "sync"

// UseCases bündelt alle Anwendungsfälle rund um Fotos.
type UseCases struct {
	Import         Import
	FindAll        FindAll
	FindEvent      FindEvent
	FindByID       FindByID
	Delete         Delete
	SetFavorite    SetFavorite
	MarkSeen       MarkSeen
	MarkPrinted    MarkPrinted
	OpenOriginal   OpenOriginal
	Locate         Locate
	InspectStorage InspectStorage
	PurgeEvent     PurgeEvent
}

// NewUseCases verdrahtet die Anwendungsfälle mit ihren Abhängigkeiten.
func NewUseCases(repo Repository, originals Originals) UseCases {
	var mutex sync.Mutex

	return UseCases{
		Import:         NewImport(&mutex, repo, originals),
		FindAll:        NewFindAll(repo),
		FindEvent:      NewFindEvent(repo),
		FindByID:       NewFindByID(repo),
		Delete:         NewDelete(&mutex, repo, originals),
		SetFavorite:    NewSetFavorite(&mutex, repo),
		MarkSeen:       NewMarkSeen(&mutex, repo),
		MarkPrinted:    NewMarkPrinted(&mutex, repo),
		OpenOriginal:   NewOpenOriginal(repo, originals),
		Locate:         NewLocate(repo, originals),
		InspectStorage: NewInspectStorage(originals),
		PurgeEvent:     NewPurgeEvent(&mutex, repo, originals),
	}
}
