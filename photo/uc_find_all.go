package photo

import (
	"cmp"
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
// [ID] zeitlich sortierbar aufgebaut ist (siehe [NewID]).
//
// Zwei Bilder derselben Millisekunde stehen in beliebiger, aber fester
// Reihenfolge zueinander: Feiner löst die Kennung nicht auf, und für zwei
// Aufnahmen im selben Augenblick gibt es auch keine richtige Antwort. Es werden zunächst
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

		// Absteigend sortieren, nicht bloß umdrehen: Die Reihenfolge, in der
		// eine Ablage ihre Schlüssel hergibt, ist ihre Sache und bei einem
		// Speicher über einer Map schlicht zufällig. Weil die [ID] mit einem
		// nullgefüllten Zeitstempel beginnt, ist die lexikographische
		// Ordnung zugleich die zeitliche.
		slices.SortFunc(ids, func(a, b ID) int { return cmp.Compare(b, a) })

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
