package printing

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// cancelExecutable ist das CUPS-Kommando, mit dem ein übergebener Auftrag
// wieder aus der Warteschlange genommen wird.
var cancelExecutable = "cancel"

// cancelTimeout begrenzt das Storno. Es läuft auch beim Herunterfahren noch,
// darf die Anwendung dabei aber nicht festhalten.
const cancelTimeout = 10 * time.Second

// CancelJob entfernt einen Auftrag samt Spool-Datei aus CUPS.
//
// Das ist die Gegenrichtung zu lp und der einzige Weg, die Kontrolle über
// einen Auftrag zurückzugewinnen. Ohne sie bleibt ein Auftrag, den die
// Fotobox aufgegeben hat, in der CUPS-Warteschlange liegen: Sobald der
// Drucker wieder erreichbar ist, druckt CUPS ihn eigenständig nach – unter
// Umständen Stunden später und mehrfach, denn CUPS wiederholt einen Auftrag
// nach ErrorPolicy=retry-job so lange, bis er als abgeschlossen gilt.
func CancelJob(ctx context.Context, jobID string) error {
	if jobID == "" {
		return nil
	}

	// Der Aufrufer bricht seinen Kontext beim Herunterfahren gerade ab –
	// genau dann muss das Storno aber noch stattfinden. Deshalb wird die
	// Abbruchkette bewusst durchtrennt und nur die Frist übernommen.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cancelTimeout)
	defer cancel()

	// "-x" löscht zusätzlich die Spool-Datei. Ohne das bliebe der Auftrag als
	// abgebrochen erhalten und ließe sich über die CUPS-Oberfläche erneut
	// starten – genau die Überraschung, die vermieden werden soll.
	out, err := exec.CommandContext(ctx, cancelExecutable, "-x", jobID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("cannot cancel cups job %s: %w: %s", jobID, err, strings.TrimSpace(string(out)))
	}

	return nil
}

// PendingJobs liefert die Kennungen aller Aufträge, die CUPS noch nicht
// abgeschlossen hat.
//
// Diese Auskunft fehlte bisher vollständig: Die Fotobox fragte ausschließlich
// die fertigen Aufträge ab und war damit blind für ihren eigenen Rückstau.
// Ein wartender Auftrag ist etwas grundlegend anderes als ein verschwundener,
// und nur diese Liste unterscheidet die beiden Fälle.
func PendingJobs(ctx context.Context, queue string) ([]string, error) {
	out, err := exec.CommandContext(ctx, "lpstat", "-W", "not-completed", "-o", queue).Output()
	if err != nil {
		return nil, err
	}

	return ParsePendingJobs(string(out)), nil
}

// ParsePendingJobs liest die Auftragskennungen aus der Ausgabe von
// "lpstat -W not-completed -o <queue>". Ausgewertet wird nur das erste Feld
// der nicht eingerückten Zeilen, damit die Übersetzung keine Rolle spielt.
func ParsePendingJobs(out string) []string {
	var ids []string

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}

		name, _, _ := strings.Cut(strings.TrimSpace(line), " ")
		if name != "" {
			ids = append(ids, name)
		}
	}

	return ids
}

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
//
// Wird die Grenze erreicht oder die Fotobox beendet, wird der Auftrag in CUPS
// storniert. Das ist zwingend: Ein aufgegebener, aber nicht stornierter
// Auftrag ist für die Fotobox unsichtbar, für CUPS aber weiterhin gültig und
// wird später ohne jedes Zutun gedruckt.
func AwaitJob(ctx context.Context, queue, jobID string, timeout, interval time.Duration) Outcome {
	if interval <= 0 {
		interval = time.Second
	}

	deadline := time.Now().Add(timeout)

	// missing zählt, wie oft der Auftrag weder als abgeschlossen noch als
	// wartend auftauchte. Erst nach mehreren Runden gilt er als verschwunden,
	// denn zwischen den beiden lpstat-Aufrufen kann ein Zustandswechsel
	// liegen.
	var missing int

	for {
		outcome, err := JobStatus(ctx, queue, jobID)
		if err == nil && outcome.Done {
			return outcome
		}

		if err == nil {
			switch pending, perr := PendingJobs(ctx, queue); {
			case perr != nil:
				// Keine Auskunft ist kein Befund – weiter warten.
			case slices.Contains(pending, jobID):
				missing = 0
			default:
				missing++
				if missing >= vanishedThreshold {
					// Weder fertig noch wartend: Jemand hat den Auftrag von
					// außen entfernt. Weiteres Warten wäre sinnlos.
					return Outcome{
						Done:    true,
						Success: false,
						Reason:  "vanished",
						Message: "CUPS kennt den Auftrag nicht mehr. Er wurde vermutlich außerhalb der Fotobox abgebrochen.",
					}
				}
			}
		}

		if time.Now().After(deadline) {
			return abandonJob(ctx, jobID, "timeout",
				"CUPS hat den Auftrag auch nach "+timeout.String()+" nicht als abgeschlossen gemeldet. "+
					"Er wurde aus der Warteschlange entfernt, damit der Drucker ihn nicht später von sich aus nachdruckt. "+
					"Möglicherweise ist das Blatt dennoch entstanden.")
		}

		select {
		case <-ctx.Done():
			return abandonJob(ctx, jobID, "canceled",
				"Die Fotobox wurde beendet. Der Auftrag wurde aus der CUPS-Warteschlange entfernt.")
		case <-time.After(interval):
		}
	}
}

// vanishedThreshold ist die Anzahl aufeinanderfolgender Abfragen, nach denen
// ein weder fertiger noch wartender Auftrag als verschwunden gilt.
const vanishedThreshold = 3

// abandonJob gibt einen Auftrag auf und räumt ihn in CUPS weg.
//
// Scheitert das Storno, wird das ausdrücklich in der Meldung genannt: Dann
// droht genau der Nachdruck, den diese Funktion verhindern soll, und die
// Bedienung muss eingreifen.
func abandonJob(ctx context.Context, jobID, reason, message string) Outcome {
	if err := CancelJob(ctx, jobID); err != nil {
		slog.Error("cannot cancel abandoned cups job", "printerJob", jobID, "reason", reason, "err", err)

		message += " Achtung: Der Auftrag ließ sich nicht aus der CUPS-Warteschlange entfernen. " +
			"Solange er dort liegt, kann der Drucker ihn erneut ausgeben – im Terminal hilft 'cancel -a'."
	}

	return Outcome{Done: true, Success: false, Reason: reason, Message: message}
}
