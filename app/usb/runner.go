package usb

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner führt ein externes Programm aus und liefert dessen Ausgabe.
//
// Eine Schnittstelle und kein fest verdrahtetes exec: lsblk und udisksctl
// hängen an echter Hardware. Ohne Austauschmöglichkeit ließe sich weder die
// Auswahl der Laufwerke noch das Einhängen prüfen, ohne am Testrechner einen
// Stick einzustecken.
type Runner interface {
	// Run liefert stdout und stderr zusammen, auch im Fehlerfall: udisksctl
	// erklärt den Grund des Scheiterns nur im Text.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner ist der echte Runner für die Fotobox.
type ExecRunner struct{}

// Run startet das Programm mit C.UTF-8 als Sprache.
//
// Die Ausgabe von udisksctl wird ausgewertet ("Mounted … at …"), und die darf
// nicht von der Spracheinstellung des Systems abhängen. C.UTF-8 statt C, weil
// lsblk im reinen C-Locale Umlaute im Namen des Sticks als \x-Folgen maskiert.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C.UTF-8")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}

	return out, nil
}

// Zeitgrenzen für die externen Programme.
//
// Ein hängendes udisksctl würde die Oberfläche sonst für immer im Ladezustand
// lassen, und vor Ort wüsste niemand, ob noch etwas passiert. Das Aushängen
// bekommt am meisten Zeit: Es schreibt die letzten Puffer auf den Stick, und
// billige Sticks sind dabei quälend langsam.
const (
	lsblkTimeout    = 15 * time.Second
	mountTimeout    = 60 * time.Second
	unmountTimeout  = 2 * time.Minute
	powerOffTimeout = 30 * time.Second
)

// run begrenzt einen Aufruf zeitlich.
func run(ctx context.Context, r Runner, timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return r.Run(ctx, name, args...)
}
