package photo

import (
	"fmt"
	"iter"
	"slices"

	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/xiter"
)

// NewFindAll erzeugt den [FindAll] Anwendungsfall.
//
// Die Fotos werden absteigend nach Aufnahmezeitpunkt geliefert, das neueste
// Bild also zuerst. Das funktioniert ohne zusätzlichen Index, weil die
// [ID] zeitlich sortierbar aufgebaut ist (siehe [NewID]). Es werden zunächst
// nur die Identifier gelesen und die Aggregate anschließend faul nachgeladen,
// damit der Startbildschirm nicht die gesamte Historie deserialisieren muss.
func NewFindAll(repo Repository) FindAll {
	return func(subject auth.Subject) iter.Seq2[Photo, error] {
		if err := subject.Audit(PermFindAll); err != nil {
			return xiter.WithError[Photo](err)
		}

		var ids []ID
		for id, err := range repo.Identifiers() {
			if err != nil {
				return xiter.WithError[Photo](fmt.Errorf("cannot read photo identifiers: %w", err))
			}

			ids = append(ids, id)
		}

		slices.Reverse(ids)

		return func(yield func(Photo, error) bool) {
			for _, id := range ids {
				optPhoto, err := repo.FindByID(id)
				if err != nil {
					if !yield(Photo{}, fmt.Errorf("cannot find photo %s: %w", id, err)) {
						return
					}

					continue
				}

				// zwischenzeitlich gelöscht, das ist kein Fehler
				if optPhoto.IsNone() {
					continue
				}

				if !yield(optPhoto.Unwrap(), nil) {
					return
				}
			}
		}
	}
}
