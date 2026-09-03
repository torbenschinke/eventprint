package upld

import "go.wdy.de/nago/auth"

// AckJob bestätigt einen übernommenen Auftrag und entfernt ihn samt Bild.
//
// Die Bestätigung kommt bewusst von der Fotobox und nicht vom Abruf: Erst wenn
// das Bild dort angekommen ist, darf es hier verschwinden.
type AckJob func(subject auth.Subject, id JobID) error

// NewAckJob erzeugt den [AckJob] Anwendungsfall.
func NewAckJob(registry *Registry) AckJob {
	return func(subject auth.Subject, id JobID) error {
		if err := subject.Audit(PermAckJob); err != nil {
			return err
		}

		if !registry.Ack(tokenOf(subject), id) {
			return errUnknownJob()
		}

		return nil
	}
}
