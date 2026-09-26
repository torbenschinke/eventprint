package photo

import (
	"fmt"
	"sync"
)

// update ändert ein Foto unter der gemeinsamen Sperre. Ein verschwundenes
// Foto ist kein Fehler: Es wurde inzwischen gelöscht, und dann gibt es nichts
// mehr zu vermerken.
func update(mutex *sync.Mutex, repo Repository, id ID, fn func(*Photo)) error {
	mutex.Lock()
	defer mutex.Unlock()

	opt, err := repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("cannot load photo: %w", err)
	}

	if opt.IsNone() {
		return nil
	}

	p := opt.Unwrap()
	fn(&p)

	if err := repo.Save(p); err != nil {
		return fmt.Errorf("cannot save photo: %w", err)
	}

	return nil
}
