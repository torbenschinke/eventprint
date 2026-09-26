// Package upld contains the transient upload relay domain.
package upld

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"go.wdy.de/nago/application/image"

	"github.com/torbenschinke/eventprint/app/printing"
)

// InboxTemplate kennzeichnet einen Auftrag für den Eingang der Fotobox.
//
// Ein solches Bild wird nicht gedruckt, sondern an der Box abgelegt; Format
// und Design wählt dort jemand am Bildschirm. Das Relais legt dafür kein
// eigenes Feld an, sondern lässt das Layout leer: "Noch nicht entschieden" ist
// genau das, was ein Eingangsbild ausmacht, und das JSON-Feld template gibt
// es bereits auf beiden Seiten des Protokolls.
const InboxTemplate printing.TemplateID = ""

type UploadID string
type JobID string
type TokenID string

type Job struct {
	ID        JobID               `json:"id"`
	Image     image.ID            `json:"-"`
	Template  printing.TemplateID `json:"template"`
	Filename  string              `json:"filename"`
	CreatedAt time.Time           `json:"createdAt"`
}

func randomID[T ~string]() (T, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("cannot create random id: %w", err)
	}
	return T(hex.EncodeToString(raw[:])), nil
}

func NewJobID() (JobID, error) { return randomID[JobID]() }
