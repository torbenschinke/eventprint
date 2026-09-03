package upld

import (
	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/std"
)

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

// errUnknownJob meldet einen Auftrag, den diese Sitzung nicht kennt.
//
// Ein fremder und ein verschwundener Auftrag sind hier bewusst nicht zu
// unterscheiden: Der Unterschied verriete, welche Kennungen existieren.
//
// Eine Funktion und keine Paketvariable: Ein Anwendungsfall, der auf Zustand
// außerhalb seiner selbst zugreift, lässt sich nicht mehr für sich allein
// betrachten.
func errUnknownJob() error {
	return std.NewLocalizedError("Unbekannter Auftrag", "Dieser Druckauftrag gehört nicht zu dieser Sitzung oder wurde bereits übernommen.")
}
