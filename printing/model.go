package printing

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"go.wdy.de/nago/pkg/data"

	"github.com/torbenschinke/eventprint/photo"
)

// JobID identifiziert einen Druckauftrag. Wie bei [photo.ID] ist die ID
// zeitlich sortierbar, damit die Druckstatus-Seite ohne Index chronologisch
// sortieren kann.
type JobID string

// NewJobID erzeugt eine neue, zeitlich sortierbare Auftrags-ID.
func NewJobID(t time.Time) JobID {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return JobID(fmt.Sprintf("%013d-%s", t.UTC().UnixMilli(), hex.EncodeToString(buf[:])))
}

// State beschreibt den Lebenszyklus eines Druckauftrags.
type State string

const (
	// StateQueued bedeutet, der Auftrag wartet auf den Worker.
	StateQueued State = "queued"

	// StatePrinting bedeutet, das Bild wird gerade gerendert und übertragen.
	StatePrinting State = "printing"

	// StateDone bedeutet, der Auftrag wurde erfolgreich an CUPS übergeben.
	StateDone State = "done"

	// StateFailed bedeutet, der Auftrag ist fehlgeschlagen. Message enthält
	// dann den Grund.
	StateFailed State = "failed"
)

func (s State) String() string {
	switch s {
	case StateQueued:
		return "Wartet"
	case StatePrinting:
		return "Druckt"
	case StateDone:
		return "Fertig"
	case StateFailed:
		return "Fehler"
	default:
		return string(s)
	}
}

// Done meldet, ob der Auftrag abgeschlossen ist – egal ob erfolgreich.
func (s State) Done() bool { return s == StateDone || s == StateFailed }

// Job ist ein einzelner Druckauftrag.
type Job struct {
	ID JobID `json:"id,omitempty"`

	// Photo verweist auf das zu druckende Foto.
	Photo photo.ID `json:"photo,omitempty"`

	// Template ist das gewählte Layout.
	Template TemplateID `json:"tpl,omitempty"`

	// Printer ist der Name der CUPS-Warteschlange.
	Printer string `json:"printer,omitempty"`

	State State `json:"state,omitempty"`

	// Message enthält bei [StateFailed] die Fehlerursache, sonst die
	// Rückmeldung von CUPS.
	Message string `json:"msg,omitempty"`

	// PrinterJob ist die Kennung in der Druckerwarteschlange, z. B. "CZ01-7".
	// Damit lässt sich ein Auftrag im Zweifel per lpstat wiederfinden.
	PrinterJob string `json:"printerJob,omitempty"`

	// Reason ist der IPP-Grund des Abschlusses, z. B.
	// "job-completed-successfully" oder "canceled-at-device". Er ist nicht
	// übersetzt und deshalb für die Fehlersuche belastbarer als die Meldung.
	Reason string `json:"reason,omitempty"`

	// RequestedBy ist der Anzeigename dessen, der den Druck ausgelöst hat.
	RequestedBy string `json:"by,omitempty"`

	CreatedAt  time.Time `json:"createdAt"`
	FinishedAt time.Time `json:"finishedAt,omitzero"`
}

func (j Job) Identity() JobID { return j.ID }

func (j Job) WithIdentity(id JobID) Job {
	j.ID = id
	return j
}

// Duration liefert die Bearbeitungsdauer, solange der Auftrag läuft die
// bisher verstrichene Zeit.
func (j Job) Duration() time.Duration {
	if j.FinishedAt.IsZero() {
		return time.Since(j.CreatedAt).Truncate(time.Second)
	}

	return j.FinishedAt.Sub(j.CreatedAt).Truncate(time.Second)
}

// Repository speichert alle Druckaufträge.
type Repository = data.Repository[Job, JobID]
