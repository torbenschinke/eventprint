package upld

import (
	"fmt"
	"io"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/std"
)

// OpenSession legt eine neue, kurzlebige Upload-Identität für die anfragende
// Fotobox an und liefert deren Kennung.
//
// Eine bereits bestehende Sitzung derselben Fotobox wird dabei verworfen. Zwei
// gültige Adressen für dieselbe Box wären eine zu viel: Der QR-Code auf dem
// Startbildschirm zeigt nur eine davon.
type OpenSession func(subject auth.Subject) (UploadID, error)

// FindPendingJobs liefert die Aufträge, die für die anfragende Fotobox
// bereitliegen.
type FindPendingJobs func(subject auth.Subject) ([]Job, error)

// OpenJobImage öffnet das Originalbild eines wartenden Auftrags.
//
// Der Aufrufer schließt den Reader.
type OpenJobImage func(subject auth.Subject, id JobID) (io.ReadCloser, error)

// AckJob bestätigt einen übernommenen Auftrag und entfernt ihn samt Bild.
//
// Die Bestätigung kommt bewusst von der Fotobox und nicht vom Abruf: Erst wenn
// das Bild dort angekommen ist, darf es hier verschwinden.
type AckJob func(subject auth.Subject, id JobID) error

// UseCases bündelt alle Anwendungsfälle des Upload-Relais.
type UseCases struct {
	OpenSession     OpenSession
	FindPendingJobs FindPendingJobs
	OpenJobImage    OpenJobImage
	AckJob          AckJob
}

// NewUseCases verdrahtet die Anwendungsfälle mit der Registry.
//
// uploadURL bildet die von außen erreichbare Adresse einer Sitzung. Sie steckt
// nicht in der Domäne, weil sie vom Betrieb abhängt und nicht von der Sache.
func NewUseCases(registry *Registry, images image.UseCases) UseCases {
	return UseCases{
		OpenSession:     NewOpenSession(registry),
		FindPendingJobs: NewFindPendingJobs(registry),
		OpenJobImage:    NewOpenJobImage(registry, images.OpenReader),
		AckJob:          NewAckJob(registry),
	}
}

// tokenOf leitet die Kennung der Fotobox aus dem angemeldeten Subjekt ab.
//
// Die Sitzung hängt am Zugangstoken, nicht am Menschen: Eine Fotobox meldet
// sich mit ihrem eigenen Token an, und genau dessen Sitzung darf sie sehen.
func tokenOf(subject auth.Subject) TokenID { return TokenID(subject.ID()) }

// NewOpenSession erzeugt den [OpenSession] Anwendungsfall.
func NewOpenSession(registry *Registry) OpenSession {
	return func(subject auth.Subject) (UploadID, error) {
		if err := subject.Audit(PermOpenSession); err != nil {
			return "", err
		}

		return registry.Open(tokenOf(subject))
	}
}

// NewFindPendingJobs erzeugt den [FindPendingJobs] Anwendungsfall.
func NewFindPendingJobs(registry *Registry) FindPendingJobs {
	return func(subject auth.Subject) ([]Job, error) {
		if err := subject.Audit(PermPollJobs); err != nil {
			return nil, err
		}

		return registry.Pending(tokenOf(subject))
	}
}

// NewOpenJobImage erzeugt den [OpenJobImage] Anwendungsfall.
func NewOpenJobImage(registry *Registry, openReader image.OpenReader) OpenJobImage {
	return func(subject auth.Subject, id JobID) (io.ReadCloser, error) {
		if err := subject.Audit(PermFetchImage); err != nil {
			return nil, err
		}

		job, ok := registry.Find(tokenOf(subject), id)
		if !ok {
			return nil, ErrUnknownJob
		}

		// Das Bild liegt im Image-Subsystem und gehört keinem Nutzer, deshalb
		// als Systemnutzer. Die Berechtigung wurde oben bereits geprüft.
		reader, err := openReader(user.SU(), job.Image)
		if err != nil {
			return nil, fmt.Errorf("cannot open upload image: %w", err)
		}

		if reader.IsNone() {
			return nil, ErrUnknownJob
		}

		return reader.Unwrap(), nil
	}
}

// NewAckJob erzeugt den [AckJob] Anwendungsfall.
func NewAckJob(registry *Registry) AckJob {
	return func(subject auth.Subject, id JobID) error {
		if err := subject.Audit(PermAckJob); err != nil {
			return err
		}

		if !registry.Ack(tokenOf(subject), id) {
			return ErrUnknownJob
		}

		return nil
	}
}

// ErrUnknownJob meldet einen Auftrag, den diese Sitzung nicht kennt.
//
// Ein fremder und ein verschwundener Auftrag sind hier bewusst nicht zu
// unterscheiden: Der Unterschied verriete, welche Kennungen existieren.
var ErrUnknownJob = std.NewLocalizedError("Unbekannter Auftrag", "Dieser Druckauftrag gehört nicht zu dieser Sitzung oder wurde bereits übernommen.")
