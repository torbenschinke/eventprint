package device

import "go.wdy.de/nago/application/permission"

// LoadSettings liefert die Einstellungen des Geräts.
type LoadSettings func(subject permission.Auditable) (Settings, error)

// NewLoadSettings erzeugt den [LoadSettings] Anwendungsfall.
func NewLoadSettings(store SettingsStore) LoadSettings {
	return func(subject permission.Auditable) (Settings, error) {
		if err := subject.Audit(PermLoadSettings); err != nil {
			return Settings{}, err
		}

		s, err := store.Load()
		return s.Normalized(), err
	}
}
