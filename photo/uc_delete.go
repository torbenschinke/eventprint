package photo

import (
	"sync"

	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/events"
)

// NewDelete erzeugt den [Delete] Anwendungsfall.
//
// Es werden nur die Metadaten entfernt. Die Bilddaten selbst verbleiben im
// Image-Subsystem und werden bei einem Backup mitgenommen, was für eine
// Veranstaltung das gewünschte Verhalten ist: ein versehentlich gelöschtes
// Foto ist so wiederherstellbar.
func NewDelete(mutex *sync.Mutex, bus events.Bus, repo Repository) Delete {
	return func(subject auth.Subject, id ID) error {
		if err := subject.Audit(PermDelete); err != nil {
			return err
		}

		mutex.Lock()
		defer mutex.Unlock()

		if err := repo.DeleteByID(id); err != nil {
			return err
		}

		bus.Publish(Deleted{Photo: id})

		return nil
	}
}
