package printing

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

// ListQueues liefert die Namen aller eingerichteten CUPS-Warteschlangen.
//
// Die Abfrage läuft über lpstat statt über eine Bibliothek: Das Kommando ist
// auf jedem Linux-System mit CUPS vorhanden und liefert genau die Namen, die
// auch lp erwartet.
func ListQueues(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, lpstatExecutable, "-a").Output()
	if err != nil {
		return nil, err
	}

	return ParseQueues(string(out)), nil
}

// ParseQueues liest die Warteschlangennamen aus der Ausgabe von "lpstat -a".
//
// Jede Zeile beginnt mit dem Namen, gefolgt von lokalisiertem Fließtext:
//
//	CZ01 akzeptiert Anfragen seit Di 11 Aug 2026 19:09:42 CEST
//
// Nur das erste Feld ist stabil, alles dahinter hängt an der Spracheinstellung
// und darf nicht ausgewertet werden.
func ParseQueues(out string) []string {
	var queues []string

	for _, line := range strings.Split(out, "\n") {
		name, _, found := strings.Cut(strings.TrimSpace(line), " ")
		if !found || name == "" {
			continue
		}

		queues = append(queues, name)
	}

	slices.Sort(queues)

	return slices.Compact(queues)
}

// QueueLister liefert die verfügbaren Warteschlangen und hält das Ergebnis
// kurz vor.
//
// Der Zwischenspeicher ist nötig, weil das Einstellungsformular die Liste bei
// jedem Neuzeichnen anfordert – ohne ihn würde pro Tastendruck ein Prozess
// gestartet.
type QueueLister struct {
	// TTL ist die Haltbarkeit des Zwischenspeichers. Null bedeutet 30
	// Sekunden; das genügt, um einen frisch eingerichteten Drucker ohne
	// Neustart der Fotobox zu finden.
	TTL time.Duration

	mutex   sync.Mutex
	queues  []string
	fetched time.Time
}

// List liefert die Warteschlangen, notfalls aus dem Zwischenspeicher.
func (l *QueueLister) List(ctx context.Context) ([]string, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	ttl := l.TTL
	if ttl <= 0 {
		ttl = 30 * time.Second
	}

	if time.Since(l.fetched) < ttl {
		return l.queues, nil
	}

	queues, err := ListQueues(ctx)
	if err != nil {
		// Ein fehlendes lpstat darf die Oberfläche nicht lahmlegen; dann ist
		// die Auswahlliste eben leer und der Name lässt sich eintippen.
		return l.queues, err
	}

	l.queues = queues
	l.fetched = time.Now()

	return queues, nil
}

// Exists meldet, ob die genannte Warteschlange eingerichtet ist. Bei einem
// Fehler wird true angenommen, damit eine kaputte lpstat-Installation keinen
// gültigen Drucker aussperrt.
func (l *QueueLister) Exists(ctx context.Context, queue string) bool {
	queues, err := l.List(ctx)
	if err != nil {
		return true
	}

	return slices.Contains(queues, queue)
}
