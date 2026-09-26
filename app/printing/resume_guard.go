package printing

import (
	"context"
	"log/slog"
	"time"
)

// Grenzen für die selbsttätige Freigabe.
//
// Der erste Versuch kommt nach kurzer Zeit, weil ein Drucker nach einem
// USB-Abriss meist binnen Sekunden wieder am Bus hängt. Danach wächst der
// Abstand: Bei leerem Farbband hält CUPS die Warteschlange nach jedem
// Versuch sofort wieder an, und das soll das Protokoll nicht im
// Sekundentakt füllen. Papier kostet ein solcher Versuch nicht – ohne
// Farbband druckt das Gerät nichts.
const (
	resumeCheckInterval = 5 * time.Second
	resumeFirstDelay    = 15 * time.Second
	resumeMaxDelay      = 2 * time.Minute
)

// resumeGuard gibt eine angehaltene Warteschlange selbsttätig wieder frei.
//
// Ohne ihn braucht jeder Papierwechsel und jeder USB-Abriss jemanden, der
// die Fotobox kennt – und auf einer Feier steht dort meistens niemand, der
// das tut.
type resumeGuard struct {
	printer  Printer
	interval time.Duration
	first    time.Duration
	max      time.Duration
}

func newResumeGuard(printer Printer) *resumeGuard {
	return &resumeGuard{
		printer:  printer,
		interval: resumeCheckInterval,
		first:    resumeFirstDelay,
		max:      resumeMaxDelay,
	}
}

func (g *resumeGuard) run(ctx context.Context) {
	tracker, ok := g.printer.(Tracker)
	if !ok {
		return
	}

	resumer, ok := g.printer.(Resumer)
	if !ok {
		return
	}

	var (
		// stoppedSince ist der Beginn des aktuellen Stillstands, der Nullwert
		// bedeutet "läuft".
		stoppedSince time.Time
		nextAttempt  time.Time
		delay        time.Duration
	)

	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		status := g.status(ctx, tracker)

		// Nur der angehaltene Drucker ist gemeint. Eine fehlende
		// Warteschlange oder eine gestoppte Annahme hat jemand bewusst so
		// eingerichtet, das lässt sich nicht durch Freigeben beheben.
		if status.Err != nil || !status.Exists || status.Enabled {
			if !stoppedSince.IsZero() && status.Err == nil {
				slog.Info("printer running again", "printer", g.printer.Name(), "stopped", time.Since(stoppedSince).Round(time.Second))
			}

			if status.Err == nil {
				stoppedSince = time.Time{}
			}

			continue
		}

		now := time.Now()
		if stoppedSince.IsZero() {
			stoppedSince = now
			delay = g.first
			nextAttempt = now.Add(delay)

			slog.Warn("printer stopped by cups", "printer", g.printer.Name(), "message", status.Message)

			continue
		}

		if now.Before(nextAttempt) {
			continue
		}

		if err := resumer.Resume(ctx); err != nil {
			slog.Error("cannot resume stopped printer", "printer", g.printer.Name(), "err", err)
		} else {
			slog.Info("resuming stopped printer", "printer", g.printer.Name(), "message", status.Message)
		}

		delay = min(delay*2, g.max)
		nextAttempt = now.Add(delay)
	}
}

func (g *resumeGuard) status(ctx context.Context, tracker Tracker) PrinterStatus {
	ctx, cancel := context.WithTimeout(ctx, diagnoseTimeout)
	defer cancel()

	return tracker.Status(ctx)
}
