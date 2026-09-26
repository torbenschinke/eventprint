package camera

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"
)

// guardBackoff ist die Pause nach einem abgefangenen Absturz. Variable statt
// Konstante, damit die Tests nicht minutenlang warten.
var guardBackoff = 2 * time.Second

// guardLimit begrenzt die Neustarts einer Schleife. Danach bleibt die
// Kamerafunktion aus, statt in einer Absturzschleife Strom zu verbrennen.
const guardLimit = 5

// guard hält eine Dauerschleife am Leben und fängt Abstürze ab.
//
// Ohne diese Klammer riss eine Panik in einer der Kamera-Goroutinen den
// gesamten Prozess mit: Die Fotobox startete neu, und mit ihr verschwanden
// Galerie, Upload und Druckwarteschlange für die Dauer des Starts. Auf einer
// Veranstaltung ohne Fernzugang ist das der Unterschied zwischen "die Kamera
// klemmt" und "die Box ist tot".
//
// Der Fehler wird nicht verschluckt: Er steht im Log und im Status, den der
// Startbildschirm unter dem QR-Code anzeigt.
func guard(ctx context.Context, name string, st *status, fn func(context.Context)) {
	for attempt := 1; ; attempt++ {
		if ctx.Err() != nil {
			return
		}

		crashed := runGuarded(ctx, name, st, fn)
		if !crashed {
			// Ordentlich zurückgekehrt, also ist Schluss.
			return
		}

		if attempt >= guardLimit {
			slog.Error("giving up on camera loop", "loop", name, "attempts", attempt)
			st.failed("", "", fmt.Sprintf(
				"Die Kameraanbindung (%s) ist wiederholt abgestürzt und bleibt aus. "+
					"Die Box läuft weiter, Aufnahmen kommen aber nicht mehr an.", name))
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(guardBackoff):
		}
	}
}

// runGuarded führt fn aus und meldet, ob es abgestürzt ist.
func runGuarded(ctx context.Context, name string, st *status, fn func(context.Context)) (crashed bool) {
	defer func() {
		if r := recover(); r != nil {
			crashed = true
			slog.Error("camera loop panicked", "loop", name, "panic", r,
				"stack", string(debug.Stack()))
			st.failed("", "", "Die Kameraanbindung ist abgestürzt und wird neu gestartet.")
		}
	}()

	fn(ctx)
	return false
}
