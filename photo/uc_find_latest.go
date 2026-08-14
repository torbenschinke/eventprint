package photo

import "go.wdy.de/nago/auth"

// NewFindLatest erzeugt den [FindLatest] Anwendungsfall auf Basis von
// [FindAll]. Da [FindAll] bereits absteigend sortiert und faul lädt, bricht
// die Iteration nach max Elementen ab.
func NewFindLatest(findAll FindAll) FindLatest {
	return func(subject auth.Subject, max int) ([]Photo, error) {
		if max <= 0 {
			return nil, nil
		}

		var res []Photo
		for photo, err := range findAll(subject) {
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
