package pairing

import (
	"slices"

	"go.wdy.de/nago/auth"
)

// FindMyBoxes listet die Boxen, die der angemeldete Nutzer gekoppelt hat,
// die jüngste zuerst.
type FindMyBoxes func(subject auth.Subject) ([]Box, error)

// NewFindMyBoxes erzeugt den Anwendungsfall.
func NewFindMyBoxes(boxes Boxes) FindMyBoxes {
	return func(subject auth.Subject) ([]Box, error) {
		if err := subject.Audit(PermFindMyBoxes); err != nil {
			return nil, err
		}

		var mine []Box
		for box, err := range boxes.All() {
			if err != nil {
				return nil, err
			}

			if box.Owner == string(subject.ID()) {
				mine = append(mine, box)
			}
		}

		slices.SortFunc(mine, func(a, b Box) int { return b.PairedAt.Compare(a.PairedAt) })

		return mine, nil
	}
}
