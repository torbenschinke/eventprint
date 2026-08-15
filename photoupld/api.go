package photoupld

import (
	"fmt"
	"io"
	"time"

	"go.wdy.de/nago/application"
	"go.wdy.de/nago/application/hapi"
	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/auth"

	"github.com/torbenschinke/eventprint/upld"
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

func ConfigureAPI(api *hapi.API, tokens application.TokenManagement, images image.UseCases, registry *upld.Registry, uploadURL func(upld.UploadID) string) {
	authenticate := func(dst *authenticated, subject auth.Subject) error {
		dst.Subject = subject
		return nil
	}
	jobAuth := func(dst *jobRequest, subject auth.Subject) error {
		dst.Subject = subject
		return nil
	}
	jobID := hapi.StrFromQuery(hapi.StrParam[jobRequest]{Name: "id", Required: true, IntoModel: func(dst *jobRequest, value string) error {
		dst.ID = upld.JobID(value)
		return nil
	}})

	hapi.Post[authenticated](api, hapi.Operation{Path: "/api/v1/session", Summary: "Neue Upload-Sitzung"}).
		Request(hapi.BearerAuth(tokens.UseCases.AuthenticateSubject, authenticate)).
		Response(hapi.ToJSON[authenticated, SessionResponse](func(in authenticated) (SessionResponse, error) {
			if err := in.Subject.Audit(upld.PermOpenSession); err != nil {
				return SessionResponse{}, err
			}
			id, err := registry.Open(upld.TokenID(in.Subject.ID()))
			return SessionResponse{UploadID: id, UploadURL: uploadURL(id)}, err
		}))

	hapi.Get[authenticated](api, hapi.Operation{Path: "/api/v1/jobs", Summary: "Wartende Druckaufträge"}).
		Request(hapi.BearerAuth(tokens.UseCases.AuthenticateSubject, authenticate)).
		Response(hapi.ToJSON[authenticated, []JobResponse](func(in authenticated) ([]JobResponse, error) {
			if err := in.Subject.Audit(upld.PermPollJobs); err != nil {
				return nil, err
			}
			jobs, err := registry.Pending(upld.TokenID(in.Subject.ID()))
			out := make([]JobResponse, 0, len(jobs))
			for _, job := range jobs {
				out = append(out, JobResponse{ID: job.ID, Template: string(job.Template), Filename: job.Filename, CreatedAt: job.CreatedAt})
			}
			return out, err
		}))

	hapi.Get[jobRequest](api, hapi.Operation{Path: "/api/v1/job/image", Summary: "Originalbild eines Druckauftrags"}).
		Request(hapi.BearerAuth(tokens.UseCases.AuthenticateSubject, jobAuth), jobID).
		Response(hapi.ToBinary[jobRequest](func(in jobRequest) (io.Reader, error) {
			if err := in.Subject.Audit(upld.PermFetchImage); err != nil {
				return nil, err
			}
			job, ok := registry.Find(upld.TokenID(in.Subject.ID()), in.ID)
			if !ok {
				return nil, fmt.Errorf("upload job not found")
			}
			reader, err := images.OpenReader(user.SU(), job.Image)
			if err != nil || reader.IsNone() {
				return nil, fmt.Errorf("upload image not found: %w", err)
			}
			return reader.Unwrap(), nil
		}))

	hapi.Delete[jobRequest](api, hapi.Operation{Path: "/api/v1/job", Summary: "Druckauftrag bestätigen"}).
		Request(hapi.BearerAuth(tokens.UseCases.AuthenticateSubject, jobAuth), jobID).
		Response(hapi.ToJSON[jobRequest, AckResponse](func(in jobRequest) (AckResponse, error) {
			if err := in.Subject.Audit(upld.PermAckJob); err != nil {
				return AckResponse{}, err
			}
			ok := registry.Ack(upld.TokenID(in.Subject.ID()), in.ID)
			if !ok {
				return AckResponse{}, fmt.Errorf("upload job not found")
			}
			return AckResponse{Acknowledged: true}, nil
		}))
}
