package photoupld

import (
	"io"
	"log/slog"
	"time"

	"go.wdy.de/nago/application"
	"go.wdy.de/nago/application/hapi"
	"go.wdy.de/nago/auth"

	"github.com/torbenschinke/eventprint/app/upld"
)

type authenticated struct{ Subject auth.Subject }
type jobRequest struct {
	authenticated
	ID upld.JobID
}

type SessionResponse struct {
	UploadID  upld.UploadID `json:"uploadId"`
	UploadURL string        `json:"uploadUrl"`
}

type JobResponse struct {
	ID        upld.JobID `json:"id"`
	Template  string     `json:"template"`
	Filename  string     `json:"filename"`
	CreatedAt time.Time  `json:"createdAt"`
}

type AckResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

// ConfigureAPI hängt die Anwendungsfälle des Relais an HTTP-Adressen.
//
// Die Handler übersetzen ausschließlich zwischen Transport und Anwendungsfall:
// Sie prüfen keine Berechtigung, greifen nicht auf die Registry zu und treffen
// keine Entscheidung. Läge die Fachlichkeit hier, gäbe es sie zweimal, sobald
// dieselbe Sache auch über die Oberfläche erreichbar sein soll.
func ConfigureAPI(api *hapi.API, tokens application.TokenManagement, uc upld.UseCases, uploadURL func(upld.UploadID) string) {
	// warnAboutToken macht die haeufigste Fehlbedienung im Protokoll sichtbar.
	//
	// Nago reicht ein unbekanntes oder abgelaufenes Token als anonymen Nutzer
	// durch, statt es abzulehnen. Der Fehler faellt deshalb erst auf, wenn der
	// Anwendungsfall die Berechtigung prueft - und wird dort zu einem nackten
	// 400 ohne Begruendung. Von aussen sehen "Token unbekannt" und "Token ohne
	// Rolle" damit gleich aus, und wer die Fotobox einrichtet, sucht im
	// Dunkeln.
	warnAboutToken := func(subject auth.Subject) {
		if subject.HasPermission(upld.PermOpenSession) {
			return
		}

		if subject.ID() == "" {
			slog.Warn("photoupld: Anfrage ohne brauchbares Token abgelehnt",
				"hinweis", "Token fehlt, ist unbekannt oder abgelaufen")

			return
		}

		slog.Warn("photoupld: Token ohne die noetige Rolle abgelehnt",
			"subject", subject.ID(),
			"hinweis", "dem Token die Rolle Fotobox-Relay zuweisen")
	}

	authenticate := func(dst *authenticated, subject auth.Subject) error {
		dst.Subject = subject
		warnAboutToken(subject)

		return nil
	}
	jobAuth := func(dst *jobRequest, subject auth.Subject) error {
		dst.Subject = subject
		warnAboutToken(subject)

		return nil
	}
	jobID := hapi.StrFromQuery(hapi.StrParam[jobRequest]{Name: "id", Required: true, IntoModel: func(dst *jobRequest, value string) error {
		dst.ID = upld.JobID(value)
		return nil
	}})

	hapi.Post[authenticated](api, hapi.Operation{Path: "/api/v1/session", Summary: "Neue Upload-Sitzung"}).
		Request(hapi.BearerAuth(tokens.UseCases.AuthenticateSubject, authenticate)).
		Response(hapi.ToJSON[authenticated, SessionResponse](func(in authenticated) (SessionResponse, error) {
			id, err := uc.OpenSession(in.Subject)
			if err != nil {
				return SessionResponse{}, err
			}

			return SessionResponse{UploadID: id, UploadURL: uploadURL(id)}, nil
		}))

	hapi.Get[authenticated](api, hapi.Operation{Path: "/api/v1/jobs", Summary: "Wartende Druckaufträge"}).
		Request(hapi.BearerAuth(tokens.UseCases.AuthenticateSubject, authenticate)).
		Response(hapi.ToJSON[authenticated, []JobResponse](func(in authenticated) ([]JobResponse, error) {
			jobs, err := uc.FindPendingJobs(in.Subject)
			if err != nil {
				return nil, err
			}

			out := make([]JobResponse, 0, len(jobs))
			for _, job := range jobs {
				out = append(out, JobResponse{ID: job.ID, Template: string(job.Template), Filename: job.Filename, CreatedAt: job.CreatedAt})
			}

			return out, nil
		}))

	hapi.Get[jobRequest](api, hapi.Operation{Path: "/api/v1/job/image", Summary: "Originalbild eines Druckauftrags"}).
		Request(hapi.BearerAuth(tokens.UseCases.AuthenticateSubject, jobAuth), jobID).
		Response(hapi.ToBinary[jobRequest](func(in jobRequest) (io.Reader, error) {
			return uc.OpenJobImage(in.Subject, in.ID)
		}))

	hapi.Delete[jobRequest](api, hapi.Operation{Path: "/api/v1/job", Summary: "Druckauftrag bestätigen"}).
		Request(hapi.BearerAuth(tokens.UseCases.AuthenticateSubject, jobAuth), jobID).
		Response(hapi.ToJSON[jobRequest, AckResponse](func(in jobRequest) (AckResponse, error) {
			if err := uc.AckJob(in.Subject, in.ID); err != nil {
				return AckResponse{}, err
			}

			return AckResponse{Acknowledged: true}, nil
		}))
}
