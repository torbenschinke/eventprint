package photo

import "go.wdy.de/nago/auth"

// FindLatest liefert die neuesten max Fotos, beginnend mit dem neuesten.
type FindLatest func(subject auth.Subject, max int) ([]Photo, error)

// NewFindLatest erzeugt den [FindLatest] Anwendungsfall auf Basis von
// [FindAll]. Da [FindAll] bereits absteigend sortiert und faul lädt, bricht
// die Iteration nach max Elementen ab.
func NewFindLatest(findAll FindAll) FindLatest {
	return func(subject auth.Subject, max int) ([]Photo, error) {
		// Eigene Prüfung, obwohl FindAll gleich darauf noch einmal prüft: Die
		// Auswahl der jüngsten Bilder ist der Startbildschirm, den ein Gast
		// sieht, und die vollständige Historie ist es nicht. Beides getrennt
		// zuteilbar zu halten ist der Zweck einer Berechtigung je
		// Anwendungsfall.
		if err := subject.Audit(PermFindLatest); err != nil {
			return nil, err
		}

		if max <= 0 {
			return nil, nil
		}

		all, err := findAll(subject)
		if err != nil {
			return nil, err
		}

		var res []Photo
		for photo, err := range all {
			if err != nil {
				return nil, err
			}

			res = append(res, photo)
			if len(res) >= max {
				break
			}
		}

		return res, nil
	}
}
