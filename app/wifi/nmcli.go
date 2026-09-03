package wifi

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// nmcliExecutable ist auswechselbar, damit die Anbindung prüfbar bleibt, ohne
// die Funkverbindung des Testrechners anzufassen.
var nmcliExecutable = "nmcli"

// SetNmcliExecutableForTest tauscht das Programm aus und liefert die Rücknahme.
func SetNmcliExecutableForTest(path string) (restore func()) {
	prev := nmcliExecutable
	nmcliExecutable = path

	return func() { nmcliExecutable = prev }
}

// scanTimeout begrenzt die Suche.
//
// Ein hängendes nmcli würde sonst die Oberfläche für immer im Ladezustand
// stehen lassen, und niemand vor Ort wüsste, ob sie noch sucht.
const scanTimeout = 30 * time.Second

// connectTimeout begrenzt den Verbindungsversuch. NetworkManager gibt bei
// falschem Kennwort nicht sofort auf.
const connectTimeout = 45 * time.Second

// runNmcli ruft nmcli auf und liefert die Ausgabe.
func runNmcli(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, nmcliExecutable, args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("nmcli %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}

	return string(out), nil
}
