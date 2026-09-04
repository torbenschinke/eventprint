package camera

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type commandRunner interface {
	Output(context.Context, string, ...string) ([]byte, error)
	Run(context.Context, io.Writer, string, ...string) error
}

type execRunner struct{}

func (execRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func (execRunner) Run(ctx context.Context, output io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = output
	cmd.Stderr = output
	return cmd.Run()
}

const (
	// minBackoff ist die Wartezeit nach einem sofort gescheiterten Start.
	minBackoff = 250 * time.Millisecond

	// maxBackoff deckelt die Wartezeit, damit eine dauerhaft klemmende Kamera
	// den Bus nicht im Millisekundentakt belegt.
	maxBackoff = 5 * time.Second

	// healthyTethering ist die Laufzeit, ab der ein Tethering als "hat
	// funktioniert" gilt. Bricht es danach ab, war es kein Startfehler,
	// sondern eine Störung im Betrieb – und dann zählt jede Sekunde.
	healthyTethering = 5 * time.Second
)

// outcome unterscheidet die Ausgänge eines Supervisor-Durchlaufs, weil sich
// daraus die Wartezeit bis zum nächsten ergibt.
type outcome int

const (
	// outcomeNoCamera: nichts am Bus. In Ruhe weitersuchen.
	outcomeNoCamera outcome = iota

	// outcomeStartFailed: Kamera erkannt, Tethering ging sofort wieder aus.
	// Meist greift ein anderer Prozess auf das Gerät zu.
	outcomeStartFailed

	// outcomeDropped: Tethering lief und brach ab. Sofort wieder aufbauen.
	outcomeDropped
)

// supervisor hält das Tethering am Leben.
type supervisor struct {
	dir    string
	load   LoadOptions
	runner commandRunner
	status *status

	// healthy ist die Laufzeit, ab der ein Tethering als geglueckt gilt.
	// Null bedeutet healthyTethering.
	healthy time.Duration

	// recover merkt sich, dass eine Lücke im Tethering war. Beim nächsten
	// Verbindungsaufbau werden die in der Lücke entstandenen Bilder von der
	// Speicherkarte nachgeholt.
	recover bool
}

// run baut das Tethering auf und nach jedem Abbruch wieder neu auf.
//
// Der frühere Aufbau wartete nach JEDEM Ausgang das volle DetectInterval von
// zehn Sekunden – auch dann, wenn ein laufendes Tethering nur kurz gestolpert
// war. In diesen zehn Sekunden lauschte niemand am USB. Eine in dieser Zeit
// ausgelöste Aufnahme landete allein auf der Speicherkarte, und weil
// --capture-tethered beim Neustart nur NEU entstehende Bilder herunterlädt,
// war sie damit endgültig verloren. Genau so verschwindet ein einzelnes Foto
// mitten aus einer Serie.
func (s *supervisor) run(ctx context.Context) {
	backoff := minBackoff
	for {
		if ctx.Err() != nil {
			return
		}

		started := time.Now()
		result := s.once(ctx)
		if ctx.Err() != nil {
			return
		}

		var wait time.Duration
		switch result {
		case outcomeDropped:
			// Der Betrieb lief bereits. Kein Warten: die nächste Aufnahme
			// kann jederzeit kommen.
			backoff = minBackoff
			wait = 0
		case outcomeStartFailed:
			wait = backoff
			if backoff *= 2; backoff > maxBackoff {
				backoff = maxBackoff
			}
		default:
			backoff = minBackoff
			wait = s.detectInterval()
		}

		// Ein Durchlauf, der ohne jede Wartezeit sofort zurückkehrt, würde
		// eine Endlosschleife auf einem Kern bedeuten.
		if wait <= 0 && time.Since(started) < minBackoff {
			wait = minBackoff
		}
		if wait <= 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

func (s *supervisor) detectInterval() time.Duration {
	if interval := s.load().DetectInterval; interval > 0 {
		return interval
	}
	return 10 * time.Second
}

// once sucht die Kamera und hält das Tethering bis zum Abbruch.
func (s *supervisor) once(ctx context.Context) outcome {
	out, err := s.runner.Output(ctx, "gphoto2", "--auto-detect")
	if err != nil {
		if ctx.Err() != nil {
			return outcomeNoCamera
		}
		slog.Error("cannot detect camera with gphoto2", "err", err)
		s.status.failed("", "", "gphoto2 antwortet nicht. Ist es installiert?")
		return outcomeStartFailed
	}

	model, port, ok := detectedCamera(string(out))
	if !ok {
		s.status.searching()
		return outcomeNoCamera
	}

	// Bilder, die während einer Tethering-Lücke entstanden sind, liegen noch
	// auf der Karte. Sie werden vor dem neuen Tethering geholt, sonst bleiben
	// sie für immer dort.
	if s.recover {
		s.recover = false
		s.recoverFromCard(ctx, model, port)
	}

	slog.Info("camera connected", "model", model, "port", port)
	s.status.connected(model, port)

	started := time.Now()
	err = s.runner.Run(ctx, os.Stderr, "gphoto2", s.tetherArgs(model, port)...)
	if ctx.Err() != nil {
		return outcomeNoCamera
	}

	lived := time.Since(started)
	if lived >= s.healthyAfter() {
		// Es lief und ist gestorben: ab jetzt klafft eine Lücke.
		s.recover = true
		slog.Warn("camera tethering dropped, reconnecting at once",
			"model", model, "lived", lived, "err", err)
		s.status.failed(model, port, "Verbindung abgerissen, wird sofort neu aufgebaut.")
		return outcomeDropped
	}

	slog.Warn("camera tethering did not start", "model", model, "lived", lived, "err", err)
	s.status.failed(model, port, "gphoto2 bekommt die Kamera nicht. "+
		"Meist greift der Datei-Manager des Systems darauf zu.")
	return outcomeStartFailed
}

func (s *supervisor) healthyAfter() time.Duration {
	if s.healthy > 0 {
		return s.healthy
	}
	return healthyTethering
}

func (s *supervisor) tetherArgs(model, port string) []string {
	prefix := fmt.Sprintf("capture-%d-%%Y%%m%%d-%%H%%M%%S-%%03n.%%C", time.Now().UnixNano())
	return []string{
		"--camera", model, "--port", port,
		"--capture-tethered", "--filename", filepath.Join(s.dir, prefix),
	}
}

// recoverFromCard holt die Aufnahmen, die während einer Tethering-Lücke nur
// auf der Speicherkarte gelandet sind.
//
// --new beschränkt das auf Dateien, die die Kamera selbst noch nicht als
// heruntergeladen führt. Ohne diese Einschränkung würde bei jedem Neuaufbau
// der gesamte Karteninhalt in den Ordner laufen.
func (s *supervisor) recoverFromCard(ctx context.Context, model, port string) {
	prefix := fmt.Sprintf("recover-%d-%%Y%%m%%d-%%H%%M%%S-%%03n.%%C", time.Now().UnixNano())
	args := []string{
		"--camera", model, "--port", port,
		"--get-all-files", "--new", "--filename", filepath.Join(s.dir, prefix),
	}
	if err := s.runner.Run(ctx, os.Stderr, "gphoto2", args...); err != nil && ctx.Err() == nil {
		// Kein Grund abzubrechen: das Tethering ist wichtiger als die
		// Nachlese, und nicht jede Kamera kennt --new.
		slog.Warn("cannot recover captures from camera card", "model", model, "err", err)
	}
}

// detectedCamera liest Modell und Port aus der Ausgabe von --auto-detect.
//
// Hängt mehr als ein PTP-Gerät am Bus, wird das erste genommen und der Rest
// protokolliert – stumm das Falsche zu tethern wäre schlimmer.
func detectedCamera(output string) (model, port string, ok bool) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasPrefix(fields[len(fields)-1], "usb:") {
			continue
		}
		found := strings.Join(fields[:len(fields)-1], " ")
		foundPort := fields[len(fields)-1]
		if ok {
			slog.Warn("ignoring additional camera on the bus", "model", found, "port", foundPort)
			continue
		}
		model, port, ok = found, foundPort, true
	}
	return model, port, ok
}
