package printing

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Outcome ist das tatsächliche Ergebnis eines Druckauftrags, so wie CUPS es
// nach der Verarbeitung meldet.
//
// Die Unterscheidung ist notwendig, weil lp lediglich die Annahme des
// Auftrags bestätigt. Was danach passiert – Filter, Treiber, Backend, Gerät –
// bleibt der Fotobox sonst verborgen. Genau daran scheiterte die erste
// Fassung: lp meldete Erfolg, CUPS verwarf den Auftrag anschließend still,
// und die Oberfläche behauptete "Fertig".
type Outcome struct {
	// Done ist wahr, sobald CUPS den Auftrag nicht mehr bearbeitet.
	Done bool

	// Success unterscheidet den erfolgreichen Abschluss vom Abbruch.
	Success bool

	// Reason ist der IPP-Grund, z. B. "job-completed-successfully" oder
	// "canceled-at-device". Diese Kennungen sind Teil des Protokolls und
	// deshalb nicht übersetzt – im Gegensatz zu den Beschriftungen von
	// lpstat.
	Reason string

	// Message ist die Klartextmeldung von CUPS, z. B.
	// "The print file could not be opened."
	Message string
}

// reasonCompleted ist der einzige IPP-Grund, der einen echten Ausdruck
// bedeutet. Alle anderen Gründe – abgebrochen, verworfen, Formatfehler –
// gelten als Fehlschlag, damit auch bisher unbekannte Ursachen auffallen
// statt stillschweigend als Erfolg durchzugehen.
const reasonCompleted = "job-completed-successfully"

// JobStatus fragt den Ausgang eines bereits übergebenen Auftrags ab.
//
// Ausgewertet wird "lpstat -l -W completed", weil dessen Ausgabe sowohl den
// IPP-Grund als auch die Klartextmeldung des Druckers enthält. Solange der
// Auftrag noch läuft, taucht er dort nicht auf; dann ist Done falsch.
func JobStatus(ctx context.Context, queue, jobID string) (Outcome, error) {
	out, err := exec.CommandContext(ctx, "lpstat", "-l", "-W", "completed", "-o", queue).Output()
	if err != nil {
		return Outcome{}, err
	}

	return ParseJobStatus(string(out), jobID), nil
}

// ParseJobStatus liest den Ausgang eines Auftrags aus der Ausgabe von
// "lpstat -l -W completed -o <queue>".
//
// Die Ausgabe besteht aus einem Block je Auftrag: Die erste Zeile beginnt mit
// der Auftragskennung, die Folgezeilen sind eingerückt und tragen übersetzte
// Beschriftungen:
//
//	CZ01-10   tschinke   711680   Di 11 Aug 2026 22:23:47 CEST
//		Status: The print file could not be opened.
//		Alarme: canceled-at-device
//		in Warteschlange eingereiht für CZ01
//
// Ausgewertet werden deshalb nur die Werte hinter dem Doppelpunkt, nicht die
// Beschriftungen: Ein Wert ohne Leerzeichen ist eine IPP-Kennung, ein Wert
// mit Leerzeichen die Klartextmeldung. Das funktioniert unabhängig von der
// Spracheinstellung des Systems.
func ParseJobStatus(out, jobID string) Outcome {
	block, ok := jobBlock(out, jobID)
	if !ok {
		return Outcome{}
	}

	res := Outcome{Done: true}

	for _, line := range block {
		_, value, found := strings.Cut(strings.TrimSpace(line), ":")
		if !found {
			continue
		}

		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		if strings.ContainsAny(value, " \t") {
			res.Message = value
			continue
		}

		res.Reason = value
	}

	res.Success = res.Reason == reasonCompleted

	return res
}

// jobBlock schneidet die Zeilen heraus, die zu genau einem Auftrag gehören.
func jobBlock(out, jobID string) ([]string, bool) {
	var (
		block []string
		found bool
	)

	for _, line := range strings.Split(out, "\n") {
		indented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")

		if !indented {
			// Eine neue, nicht eingerückte Zeile beendet den gesuchten Block.
			if found {
				break
			}

			name, _, _ := strings.Cut(strings.TrimSpace(line), " ")
			found = name == jobID

			continue
		}

		if found {
			block = append(block, line)
		}
	}

	return block, found
}

// AwaitJob wartet, bis CUPS den Auftrag abgeschlossen hat.
//
// Die Zeitgrenze verhindert, dass ein hängender Auftrag die Warteschlange der
// Fotobox blockiert: Der Worker arbeitet bewusst seriell, weil es nur einen
// Drucker gibt.
func AwaitJob(ctx context.Context, queue, jobID string, timeout, interval time.Duration) Outcome {
	if interval <= 0 {
		interval = time.Second
	}

	deadline := time.Now().Add(timeout)

	for {
		outcome, err := JobStatus(ctx, queue, jobID)
		if err == nil && outcome.Done {
			return outcome
		}

		if time.Now().After(deadline) {
			return Outcome{
				Done:    true,
				Success: false,
				Reason:  "timeout",
				Message: "CUPS meldet den Auftrag auch nach " + timeout.String() + " nicht als abgeschlossen. Prüfe, ob der Drucker angeschlossen und eingeschaltet ist.",
			}
		}

		select {
		case <-ctx.Done():
			return Outcome{Done: true, Reason: "canceled", Message: "Die Fotobox wurde beendet."}
		case <-time.After(interval):
		}
	}
}
