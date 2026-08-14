package printing

import (
	"context"
	"os/exec"
	"strings"
)

// PrinterStatus beschreibt den Zustand einer CUPS-Warteschlange.
//
// Die Druckstatus-Seite zeigt ihn an, damit die häufigsten Ursachen ohne
// Terminal erkennbar sind: Warteschlange gelöscht, Drucker angehalten, oder
// Annahme gestoppt.
type PrinterStatus struct {
	// Queue ist der abgefragte Name.
	Queue string

	// Exists ist falsch, wenn es die Warteschlange nicht gibt – etwa nach
	// einem Tippfehler oder wenn CUPS nicht läuft.
	Exists bool

	// Enabled ist falsch, wenn der Drucker angehalten wurde. Aufträge bleiben
	// dann in der Warteschlange liegen, statt zu drucken.
	Enabled bool

	// Accepting ist falsch, wenn die Warteschlange keine neuen Aufträge
	// annimmt.
	Accepting bool

	// Message ist die Zustandsmeldung des Geräts, z. B. "Out of paper".
	Message string

	// Err beschreibt, warum der Zustand nicht ermittelt werden konnte.
	Err error
}

// OK meldet, ob aus Sicht von CUPS alles bereit ist.
func (s PrinterStatus) OK() bool {
	return s.Err == nil && s.Exists && s.Enabled && s.Accepting
}

// Problem beschreibt in einem Satz, was der Aufstellung im Weg steht.
// Ein leeres Ergebnis bedeutet, dass alles bereit ist.
func (s PrinterStatus) Problem() string {
	switch {
	case s.Err != nil:
		return "Der Zustand des Druckers konnte nicht ermittelt werden: " + s.Err.Error()
	case !s.Exists:
		return "Die Warteschlange " + s.Queue + " ist in CUPS nicht eingerichtet."
	case !s.Enabled:
		return "Der Drucker ist angehalten. Aufträge bleiben liegen, bis er freigegeben wird."
	case !s.Accepting:
		return "Die Warteschlange nimmt keine neuen Aufträge an."
	default:
		return ""
	}
}

// QueryPrinter ermittelt den Zustand einer Warteschlange.
func QueryPrinter(ctx context.Context, queue string) PrinterStatus {
	status := PrinterStatus{Queue: queue}

	// "lpstat -p" meldet Freigabe und Gerätemeldung, "-a" die Annahme. Beide
	// Ausgaben sind übersetzt, weshalb ausschließlich der Rückgabewert und
	// die maschinenlesbaren Anteile ausgewertet werden.
	printerOut, printerErr := exec.CommandContext(ctx, "lpstat", "-p", queue).CombinedOutput()
	acceptOut, acceptErr := exec.CommandContext(ctx, "lpstat", "-a", queue).CombinedOutput()

	if printerErr != nil && len(printerOut) == 0 {
		status.Err = printerErr
		return status
	}

	// Eine unbekannte Warteschlange quittiert lpstat mit einem Fehlercode.
	status.Exists = printerErr == nil
	status.Enabled = status.Exists && !isDisabled(string(printerOut))
	status.Accepting = acceptErr == nil && !isRejecting(string(acceptOut))
	status.Message = deviceMessage(string(printerOut))

	return status
}

// isDisabled erkennt den angehaltenen Zustand.
//
// lpstat übersetzt den Satz, das Schlüsselwort "disabled" bzw. "deaktiviert"
// bleibt aber erhalten; zusätzlich wird die englische Fassung berücksichtigt,
// falls die Fotobox ohne deutsche Locale läuft.
func isDisabled(out string) bool {
	lower := strings.ToLower(out)

	return strings.Contains(lower, "disabled") || strings.Contains(lower, "deaktiviert")
}

// isRejecting erkennt eine Warteschlange, die keine Aufträge annimmt.
func isRejecting(out string) bool {
	lower := strings.ToLower(out)

	return strings.Contains(lower, "not accepting") || strings.Contains(lower, "nicht an")
}

// deviceMessage schneidet die Zustandsmeldung des Geräts heraus. Sie steht
// eingerückt unter der Kopfzeile, z. B.
//
//	Drucker CZ01 ist im Leerlauf.  Aktiviert seit ...
//		Printing started (1 copies)
func deviceMessage(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
			continue
		}

		if msg := strings.TrimSpace(line); msg != "" {
			return msg
		}
	}

	return ""
}
