package device

import (
	"sync"

	"go.wdy.de/nago/application/permission"
)

// SaveSettings ändert die Einstellungen mit fn.
//
// Eine Änderungsfunktion und kein fertiger Datensatz: Zwischen Anzeigen und
// Speichern zählt der Druck-Worker Papier ab und der Kiosk-Start trägt eine
// Feier ein. Wer den ganzen Datensatz von vorhin zurückschriebe, löschte
// diese Änderungen unbemerkt.
//
// PIN und Feierliste sind davon ausgenommen; für sie gibt es eigene Wege.
type SaveSettings func(subject permission.Auditable, fn func(*Settings)) (Settings, error)

// NewSaveSettings erzeugt den [SaveSettings] Anwendungsfall.
func NewSaveSettings(mutex *sync.Mutex, store SettingsStore) SaveSettings {
	return func(subject permission.Auditable, fn func(*Settings)) (Settings, error) {
		if err := subject.Audit(PermSaveSettings); err != nil {
			return Settings{}, err
		}

		return modify(mutex, store, func(s *Settings) {
			pin, events := s.Pin, s.Events
			fn(s)
			s.Pin, s.Events = pin, events
		})
	}
}

// modify liest, ändert und schreibt unter der gemeinsamen Sperre.
func modify(mutex *sync.Mutex, store SettingsStore, fn func(*Settings)) (Settings, error) {
	mutex.Lock()
	defer mutex.Unlock()

	s, err := store.Load()
	if err != nil {
		return Settings{}, err
	}

	s = s.Normalized()
	fn(&s)
	s = s.Normalized()

	if err := store.Save(s); err != nil {
		return Settings{}, err
	}

	return s, nil
}
