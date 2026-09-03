package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
)

type Relay struct {
	opts      Options
	client    *Client
	photos    photo.UseCases
	printing  printing.UseCases
	uploadURL atomic.Pointer[string]
	processed map[string]struct{}
}

// Manager keeps the relay aligned with the current Nago settings.
type Manager struct {
	load     func() Options
	photos   photo.UseCases
	printing printing.UseCases
	relay    atomic.Pointer[Relay]
}

func NewManager(load func() Options, photos photo.UseCases, prints printing.UseCases) *Manager {
	return &Manager{load: load, photos: photos, printing: prints}
}

func (m *Manager) UploadURL() string {
	if current := m.relay.Load(); current != nil {
		return current.UploadURL()
	}
	return ""
}

func (m *Manager) Run(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var active Options
	var cancel context.CancelFunc
	for {
		next := m.load()
		if next.URL != active.URL || next.Token != active.Token {
			if cancel != nil {
				cancel()
			}
			m.relay.Store(nil)
			active = next
			if active.Enabled() {
				relay, err := New(active, m.photos, m.printing)
				if err != nil {
					slog.Error("cannot configure photoupld relay", "err", err)
				} else {
					var relayCtx context.Context
					relayCtx, cancel = context.WithCancel(ctx)
					m.relay.Store(relay)
					go relay.Run(relayCtx)
				}
			}
		}

		select {
		case <-ctx.Done():
			if cancel != nil {
				cancel()
			}
			return
		case <-ticker.C:
		}
	}
}

func New(opts Options, photos photo.UseCases, prints printing.UseCases) (*Relay, error) {
	client, err := NewClient(opts)
	if err != nil {
		return nil, err
	}
	if opts.Interval <= 0 {
		opts.Interval = 10 * time.Second
	}
	return &Relay{opts: opts, client: client, photos: photos, printing: prints, processed: map[string]struct{}{}}, nil
}

func (r *Relay) UploadURL() string {
	if current := r.uploadURL.Load(); current != nil {
		return *current
	}
	return ""
}

func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.opts.Interval)
	defer ticker.Stop()
	for {
		if r.UploadURL() == "" {
			if url, err := r.client.OpenSession(ctx); err != nil {
				slog.Error("cannot open photoupld session", "err", err)
			} else {
				r.uploadURL.Store(&url)
				slog.Info("photoupld session ready", "url", url)
			}
		} else if err := r.poll(ctx); err != nil {
			slog.Error("cannot poll photoupld", "err", err)
			var httpErr HTTPError
			if errors.As(err, &httpErr) && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
				// Die Sitzung ist hinfällig, eine neue wird gleich geöffnet.
				//
				// r.processed wird dabei bewusst nicht geleert: Die Menge ist
				// die einzige Absicherung dagegen, ein bereits gedrucktes
				// Bild ein zweites Mal auszugeben, falls dessen Bestätigung
				// zuvor fehlschlug. Sie kostet nur wenige Bytes je Upload und
				// darf deshalb über den Sitzungswechsel hinaus bestehen.
				r.uploadURL.Store(nil)
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Relay) poll(ctx context.Context) error {
	jobs, err := r.client.Jobs(ctx)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if _, ok := r.processed[job.ID]; ok {
			if err := r.client.Ack(ctx, job.ID); err != nil {
				slog.Error("cannot acknowledge processed remote print job", "job", job.ID, "err", err)
			}
			continue
		}
		if err := r.process(ctx, job); err != nil {
			slog.Error("cannot process remote print job", "job", job.ID, "err", err)
		}
	}
	return nil
}

func (r *Relay) process(ctx context.Context, job Job) error {
	reader, mime, err := r.client.Image(ctx, job.ID)
	if err != nil {
		return err
	}
	raw, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if mime == "" || mime == "application/octet-stream" {
		mime = http.DetectContentType(raw)
	}
	p, err := r.photos.Import(user.SU(), photo.Options{Source: photo.SourceRelay}, image.MemFile{Filename: job.Filename, MimeTypeHint: mime, Bytes: raw})
	if err != nil {
		return fmt.Errorf("cannot import remote photo: %w", err)
	}
	if _, err := r.printing.Print(user.SU(), p.ID, job.Template); err != nil {
		return fmt.Errorf("cannot queue remote photo: %w", err)
	}
	r.processed[job.ID] = struct{}{}
	if err := r.client.Ack(ctx, job.ID); err != nil {
		return fmt.Errorf("cannot acknowledge remote photo: %w", err)
	}
	return nil
}
