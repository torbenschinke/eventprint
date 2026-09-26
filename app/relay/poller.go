package relay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Deliver übergibt ein abgeholtes Bild an das Gerät. Ein Fehler lässt den
// Upload beim Dienst liegen; er wird beim nächsten Durchlauf erneut geholt.
type Deliver func(ctx context.Context, job Job, data []byte) error

// State beschreibt, woran die Verbindung zum Upload-Dienst gerade ist.
type State int

const (
	// StateOff bedeutet: kein Upload-Dienst eingetragen.
	StateOff State = iota

	// StateMissingToken bedeutet: Adresse eingetragen, Token fehlt.
	StateMissingToken

	// StateConnecting bedeutet: eingerichtet, aber noch keine Sitzung. Das
	// ist entweder der Moment nach dem Start, kein Netz oder ein abgelehntes
	// Token.
	StateConnecting

	// StateReady bedeutet: Sitzung steht, der QR-Code führt zum Dienst.
	StateReady
)

// Poller hält die Verbindung zum Upload-Dienst und holt neue Bilder ab.
//
// Er folgt den Einstellungen: Ändert sich Adresse oder Token, beginnt er mit
// einer neuen Sitzung, ohne dass das Gerät neu starten muss.
type Poller struct {
	load    func() Options
	deliver Deliver

	uploadURL atomic.Pointer[string]
	lastError atomic.Pointer[string]

	// processed merkt sich übernommene Uploads über Sitzungswechsel hinweg.
	// Die Menge ist die einzige Absicherung dagegen, ein Bild ein zweites Mal
	// anzunehmen, falls zuvor nur die Bestätigung scheiterte.
	mu        sync.Mutex
	processed map[string]struct{}
}

// NewPoller erzeugt den Poller; gestartet wird er mit [Poller.Run].
func NewPoller(load func() Options, deliver Deliver) *Poller {
	return &Poller{load: load, deliver: deliver, processed: map[string]struct{}{}}
}

// UploadURL liefert die Adresse der aktuellen Sitzung, leer ohne Sitzung.
func (p *Poller) UploadURL() string {
	if u := p.uploadURL.Load(); u != nil {
		return *u
	}

	return ""
}

// LastError beschreibt den letzten Fehler, leer ohne Fehler.
func (p *Poller) LastError() string {
	if e := p.lastError.Load(); e != nil {
		return *e
	}

	return ""
}

// State liefert den Zustand für die Anzeige.
//
// Ein QR-Code, der ins Leere führt, ist schlimmer als keiner: Er sieht aus wie
// jeder andere, und der Gast merkt den Fehler erst mit dem Handy in der Hand.
// Die Oberfläche fragt deshalb hier nach, bevor sie einen Code zeigt.
func (p *Poller) State() State {
	opts := p.load()

	switch {
	case !opts.Configured():
		return StateOff
	case !opts.Enabled():
		return StateMissingToken
	case p.UploadURL() == "":
		return StateConnecting
	default:
		return StateReady
	}
}

// Run arbeitet, bis ctx endet.
func (p *Poller) Run(ctx context.Context) {
	const tick = 5 * time.Second

	timer := time.NewTimer(0)
	defer timer.Stop()

	var active Options
	var client *Client

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		next := p.load()
		if next.URL != active.URL || next.Token != active.Token {
			active = next
			client = nil
			p.uploadURL.Store(nil)

			if active.Enabled() {
				c, err := NewClient(active)
				if err != nil {
					p.fail(err)
				} else {
					client = c
				}
			}
		}

		if client != nil {
			p.step(ctx, client)
		}

		interval := active.Interval
		if interval <= 0 {
			interval = tick
		}

		timer.Reset(interval)
	}
}

// step öffnet bei Bedarf eine Sitzung und holt sonst ab.
func (p *Poller) step(ctx context.Context, client *Client) {
	if p.UploadURL() == "" {
		u, err := client.OpenSession(ctx)
		if err != nil {
			p.fail(err)
			return
		}

		p.uploadURL.Store(&u)
		p.lastError.Store(nil)
		slog.Info("upload relay session ready", "url", u)

		return
	}

	if err := p.poll(ctx, client); err != nil {
		p.fail(err)

		var httpErr HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
			// Die Sitzung ist hinfällig; im nächsten Durchlauf entsteht eine
			// neue.
			p.uploadURL.Store(nil)
		}

		return
	}

	p.lastError.Store(nil)
}

func (p *Poller) fail(err error) {
	msg := err.Error()
	p.lastError.Store(&msg)
	slog.Error("upload relay", "err", err)
}

func (p *Poller) poll(ctx context.Context, client *Client) error {
	jobs, err := client.Jobs(ctx)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if p.seen(job.ID) {
			if err := client.Ack(ctx, job.ID); err != nil {
				slog.Error("cannot acknowledge processed upload", "job", job.ID, "err", err)
			}

			continue
		}

		if err := p.process(ctx, client, job); err != nil {
			slog.Error("cannot process upload", "job", job.ID, "err", err)
		}
	}

	return nil
}

func (p *Poller) seen(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, ok := p.processed[id]
	return ok
}

func (p *Poller) process(ctx context.Context, client *Client, job Job) error {
	reader, _, err := client.Image(ctx, job.ID)
	if err != nil {
		return err
	}

	raw, readErr := io.ReadAll(io.LimitReader(reader, 64<<20))
	closeErr := reader.Close()
	if readErr != nil {
		return readErr
	}

	if closeErr != nil {
		return closeErr
	}

	if err := p.deliver(ctx, job, raw); err != nil {
		return fmt.Errorf("cannot deliver upload: %w", err)
	}

	p.mu.Lock()
	p.processed[job.ID] = struct{}{}
	p.mu.Unlock()

	if err := client.Ack(ctx, job.ID); err != nil {
		return fmt.Errorf("cannot acknowledge upload: %w", err)
	}

	return nil
}
