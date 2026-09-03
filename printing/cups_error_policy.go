package printing

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

// lpadminExecutable verwaltet Warteschlangen im Druckdienst.
var lpadminExecutable = "lpadmin"

const (
	// AbortPolicy weist CUPS an, einen gescheiterten Auftrag zu verwerfen.
	//
	// Die Vorgabe von CUPS ist "retry-job": Meldet das Backend einen Fehler,
	// stellt CUPS den Auftrag zurück in die Warteschlange und versucht es
	// später erneut. Für einen Fotodrucker ist das die falsche Annahme. Ein
	// Dye-Sublimation-Gerät hat das Blatt beim Fehlerfall meist längst
	// ausgeworfen; der Wiederholversuch verbraucht ein zweites.
	//
	// Genau das ist am 31.08.2026 passiert: Der Backend verlor beim
	// Wiederanstecken mitten in der Übertragung das USB-Gerät, CUPS wiederholte
	// den Auftrag von sich aus, und derselbe Ausdruck kam mehrfach.
	AbortPolicy = "abort-job"

	// policyTimeout begrenzt den Aufruf, damit ein hängender Druckdienst weder
	// den Start noch den ersten Druck aufhält.
	policyTimeout = 10 * time.Second
)

// EnforceAbortPolicy stellt die Fehlerbehandlung einer Warteschlange auf
// [AbortPolicy] um.
//
// Der Aufruf ist wiederholbar und ändert nichts, wenn der Wert bereits stimmt.
// Er benötigt keine root-Rechte, wohl aber die Mitgliedschaft in der
// SystemGroup von CUPS – auf Debian und Raspberry Pi OS ist das die Gruppe
// "lpadmin".
//
// Zuerst wird geprüft, ob es die Warteschlange überhaupt gibt. Das ist keine
// Höflichkeit, sondern notwendig: "lpadmin -p <name> -o …" **legt eine
// Warteschlange an**, wenn sie fehlt. Bei einem Tippfehler in den
// Einstellungen entstünde also ein Phantomdrucker – und ausgerechnet die
// Meldung "Warteschlange nicht eingerichtet", die den Tippfehler aufdecken
// soll, verschwände damit.
func EnforceAbortPolicy(ctx context.Context, queue string) error {
	if queue == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, policyTimeout)
	defer cancel()

	if !queueExists(ctx, queue) {
		return fmt.Errorf("cannot set error policy: queue %s does not exist", queue)
	}

	out, err := exec.CommandContext(ctx, lpadminExecutable, "-p", queue, "-o", "printer-error-policy="+AbortPolicy).CombinedOutput()
	if err != nil {
		return fmt.Errorf("cannot set error policy for %s: %w: %s", queue, err, strings.TrimSpace(string(out)))
	}

	return nil
}

// queueExists meldet, ob CUPS die Warteschlange kennt.
//
// Kann lpstat keine Auskunft geben, gilt die Warteschlange als vorhanden: Eine
// kaputte lpstat-Installation darf nicht dazu führen, dass die Absicherung
// eines gültigen Druckers unterbleibt. Der Aufruf von lpadmin scheitert dann
// eben selbst und wird protokolliert.
func queueExists(ctx context.Context, queue string) bool {
	queues, err := ListQueues(ctx)
	if err != nil {
		return true
	}

	return slices.Contains(queues, queue)
}

// policyGuard sorgt dafür, dass jede Warteschlange höchstens einmal je
// Programmlauf umgestellt wird.
//
// Die Einstellung überlebt in CUPS jeden Neustart, ein zweiter Aufruf wäre
// also nur Aufwand. Gewechselt werden kann die Warteschlange trotzdem
// jederzeit im laufenden Betrieb – deshalb eine Menge und kein einzelner Name.
type policyGuard struct {
	mutex sync.Mutex

	// done sind die Warteschlangen, deren Umstellung geglückt ist.
	done map[string]struct{}

	// warned verhindert, dass ein dauerhaft misslingender Versuch das
	// Protokoll flutet.
	warned map[string]struct{}
}

// ensure stellt die Warteschlange um, sofern das in diesem Programmlauf noch
// nicht geglückt ist.
//
// Vermerkt wird nur der Erfolg. Ein Fehlschlag darf sich nicht festschreiben:
// Wer den Drucker erst nach dem Start der Fotobox in CUPS einrichtet, bekäme
// die Absicherung sonst bis zum nächsten Neustart nicht mehr.
//
// Ein Fehlschlag bricht nichts ab: Ohne die Berechtigung dazu muss die Fotobox
// trotzdem drucken können. Sie kann seit dem Storno beim Timeout auch selbst
// verhindern, dass ein aufgegebener Auftrag später nachgedruckt wird – die
// Umstellung ist die zweite Sicherung, nicht die einzige.
func (g *policyGuard) ensure(ctx context.Context, queue string) {
	if queue == "" {
		return
	}

	g.mutex.Lock()
	_, settled := g.done[queue]
	g.mutex.Unlock()

	if settled {
		return
	}

	if err := EnforceAbortPolicy(ctx, queue); err != nil {
		g.mutex.Lock()
		_, complained := g.warned[queue]
		if !complained {
			if g.warned == nil {
				g.warned = map[string]struct{}{}
			}

			g.warned[queue] = struct{}{}
		}
		g.mutex.Unlock()

		if !complained {
			slog.Warn("cannot enforce abort-job error policy",
				"queue", queue,
				"err", err,
				"hint", "Ohne diese Einstellung wiederholt CUPS gescheiterte Aufträge von sich aus. Manuell: sudo lpadmin -p "+queue+" -o printer-error-policy="+AbortPolicy,
			)
		}

		return
	}

	g.mutex.Lock()
	if g.done == nil {
		g.done = map[string]struct{}{}
	}

	g.done[queue] = struct{}{}
	g.mutex.Unlock()

	slog.Info("error policy enforced", "queue", queue, "policy", AbortPolicy)
}
