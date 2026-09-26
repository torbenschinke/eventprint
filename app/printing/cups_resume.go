package printing

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// cupsenableExecutable gibt eine angehaltene Warteschlange wieder frei.
var cupsenableExecutable = "cupsenable"

// resumeTimeout begrenzt den Aufruf, damit ein hängender Druckdienst weder
// die Oberfläche noch die Überwachung festhält.
const resumeTimeout = 10 * time.Second

// ResumeQueue gibt eine angehaltene CUPS-Warteschlange wieder frei.
//
// Angehalten wird sie nicht von der Fotobox, sondern vom Backend: Gutenprint
// beendet sich bei "Ribbon End" und bei einer abgerissenen USB-Verbindung mit
// Status 4 ("stop printer"). Die ErrorPolicy greift dann nicht – sie gilt nur
// für Status 1 –, und die Warteschlange bleibt angehalten, bis jemand
// cupsenable aufruft. Auch über einen Neustart hinweg, denn der Zustand steht
// in printers.conf.
//
// So geschehen am 05.09.2026: Nach jedem Papierwechsel und nach jedem
// USB-Abriss durch die Kamera musste im Terminal "sudo cupsenable CZ01"
// eingegeben werden.
//
// Wie bei lpadmin genügt die Mitgliedschaft in der SystemGroup von CUPS
// ("lpadmin"), root ist nicht nötig.
func ResumeQueue(ctx context.Context, queue string) error {
	if queue == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, resumeTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, cupsenableExecutable, queue).CombinedOutput()
	if err != nil {
		return fmt.Errorf("cannot resume queue %s: %w: %s", queue, err, strings.TrimSpace(string(out)))
	}

	return nil
}

// Resumer wird von Druckern implementiert, die eine angehaltene Warteschlange
// wieder freigeben können.
type Resumer interface {
	Resume(ctx context.Context) error
}

// Resume gibt die Warteschlange wieder frei.
func (p CUPSPrinter) Resume(ctx context.Context) error {
	return ResumeQueue(ctx, p.Queue)
}

// Resume gibt die aktuell eingestellte Warteschlange frei. Im Testbetrieb
// gibt es nichts freizugeben.
func (p settingsPrinter) Resume(ctx context.Context) error {
	target, ok := p.target().(Resumer)
	if !ok {
		return nil
	}

	return target.Resume(ctx)
}
