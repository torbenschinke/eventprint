package camera

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestDetectedCamera(t *testing.T) {
	model, port, ok := detectedCamera("Modell                         Port\n----------------------------------------------------------\nCanon EOS 80D                  usb:001,004\n")
	if !ok || model != "Canon EOS 80D" || port != "usb:001,004" {
		t.Fatalf("detectedCamera = %q, %q, %v", model, port, ok)
	}
	if _, _, ok := detectedCamera("Modell Port\n------ ----\n"); ok {
		t.Fatal("camera detected in empty output")
	}
}

// TestDetectedCameraTakesFirstOfMany haelt fest, dass bei mehreren Geraeten
// am Bus das erste genommen wird, statt zu raten oder aufzugeben.
func TestDetectedCameraTakesFirstOfMany(t *testing.T) {
	model, port, ok := detectedCamera("Canon EOS 80D usb:001,004\nNikon D750 usb:001,007\n")
	if !ok || model != "Canon EOS 80D" || port != "usb:001,004" {
		t.Fatalf("detectedCamera = %q, %q, %v", model, port, ok)
	}
}

func TestSupervisorStartsTetheringForDetectedCamera(t *testing.T) {
	runner := &recordingRunner{output: []byte("Modell Port\n------ ----\nCanon EOS 80D usb:001,004\n"), runErr: errors.New("disconnected")}
	s := newTestSupervisor(t.TempDir(), runner)
	if s.once(context.Background()) == outcomeNoCamera {
		t.Fatal("detected camera was not started")
	}

	call := runner.calls[len(runner.calls)-1]
	if call.name != "gphoto2" {
		t.Fatalf("command = %q", call.name)
	}
	want := []string{"--camera", "Canon EOS 80D", "--port", "usb:001,004", "--capture-tethered", "--filename"}
	for i, arg := range want {
		if call.args[i] != arg {
			t.Fatalf("arg %d = %q, want %q", i, call.args[i], arg)
		}
	}
	if len(call.args) != len(want)+1 || filepath.Ext(call.args[len(call.args)-1]) != ".%C" {
		t.Fatalf("unexpected filename pattern %q", call.args[len(call.args)-1])
	}
	if got := s.status.get(); got.State != StateError || got.Model != "Canon EOS 80D" {
		t.Fatalf("status = %+v, want error state with model", got)
	}
}

// TestSupervisorReportsSearchingWithoutCamera deckt ab, dass ein leerer Bus
// nicht als Fehler dasteht. Sonst stuende auf dem Startbildschirm eine rote
// Warnung, solange die Kamera noch nicht angeschlossen ist.
func TestSupervisorReportsSearchingWithoutCamera(t *testing.T) {
	runner := &recordingRunner{output: []byte("Modell Port\n------ ----\n")}
	s := newTestSupervisor(t.TempDir(), runner)
	if got := s.once(context.Background()); got != outcomeNoCamera {
		t.Fatalf("outcome = %v, want no camera", got)
	}
	if got := s.status.get().State; got != StateSearching {
		t.Fatalf("state = %q, want searching", got)
	}
}

// TestSupervisorRecoversCardAfterDroppedTethering ist der Kern des verlorenen
// Fotos.
//
// Bricht ein laufendes Tethering ab, wartete der Supervisor vorher das volle
// DetectInterval von zehn Sekunden. In dieser Luecke lauschte niemand am USB,
// und weil --capture-tethered nur NEU entstehende Bilder herunterlaedt, war
// eine in dieser Zeit ausgeloeste Aufnahme endgueltig verloren. Jetzt wird
// sofort neu verbunden und die Karte mit --get-all-files --new nachgelesen.
func TestSupervisorRecoversCardAfterDroppedTethering(t *testing.T) {
	runner := &recordingRunner{
		output: []byte("Canon EOS 80D usb:001,004\n"),
		runErr: errors.New("ptp timeout"),
		runFor: 20 * time.Millisecond,
	}
	s := newTestSupervisor(t.TempDir(), runner)
	s.healthy = 10 * time.Millisecond

	if got := s.once(context.Background()); got != outcomeDropped {
		t.Fatalf("outcome = %v, want dropped", got)
	}
	if !s.recover {
		t.Fatal("dropped tethering did not arm the card recovery")
	}

	runner.runFor = 0
	s.once(context.Background())

	var recovered []string
	for _, call := range runner.calls {
		if len(call.args) > 4 && call.args[4] == "--get-all-files" {
			recovered = call.args
		}
	}
	if recovered == nil {
		t.Fatal("captures left on the card were never fetched")
	}
	if recovered[5] != "--new" {
		t.Fatalf("recovery args = %v, want --new to limit the download", recovered)
	}
	if s.recover {
		t.Fatal("recovery must not repeat on every reconnect")
	}
}

// TestSupervisorDoesNotRecoverOnFirstConnect stellt sicher, dass beim ersten
// Verbinden nicht der gesamte Karteninhalt in den Ordner laeuft.
func TestSupervisorDoesNotRecoverOnFirstConnect(t *testing.T) {
	runner := &recordingRunner{output: []byte("Canon EOS 80D usb:001,004\n"), runErr: errors.New("nope")}
	newTestSupervisor(t.TempDir(), runner).once(context.Background())

	for _, call := range runner.calls {
		for _, arg := range call.args {
			if arg == "--get-all-files" {
				t.Fatal("first connect must not download the whole card")
			}
		}
	}
}

// TestSupervisorBacksOffWhenTetheringNeverStarts deckt den Fall ab, dass
// gphoto2 sofort wieder zurueckkehrt - typisch, wenn der Datei-Manager des
// Systems die Kamera belegt. Ohne Backoff waere das eine Endlosschleife.
func TestSupervisorBacksOffWhenTetheringNeverStarts(t *testing.T) {
	runner := &recordingRunner{output: []byte("Canon EOS 80D usb:001,004\n"), runErr: errors.New("could not claim the usb device")}
	s := newTestSupervisor(t.TempDir(), runner)
	if got := s.once(context.Background()); got != outcomeStartFailed {
		t.Fatalf("outcome = %v, want start failed", got)
	}
	if s.recover {
		t.Fatal("a tethering that never started leaves no gap to recover")
	}
}

// TestWatcherDeliversCaptureAndRemovesFile haelt fest, was beim Geraet
// ankommt: der Dateiname ohne Verzeichnis und genau die Bytes der Aufnahme.
// Danach darf die Datei nicht liegen bleiben, sonst kaeme sie beim naechsten
// Durchlauf ein zweites Mal.
func TestWatcherDeliversCaptureAndRemovesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture.jpg")
	raw := testJPEG(t)
	writeFile(t, path, raw)

	h := newTestHarness(t, dir)
	h.pump()

	if len(h.delivered) != 1 {
		t.Fatalf("deliveries=%d, want 1", len(h.delivered))
	}
	got := h.delivered[0]
	if got.name != "capture.jpg" {
		t.Fatalf("name = %q, want capture.jpg", got.name)
	}
	if !bytes.Equal(got.data, raw) {
		t.Fatal("delivered bytes differ from the capture")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("delivered file remains: %v", err)
	}
}

// TestWatcherDeliversOnFirstScan haelt die Latenz fest.
//
// Vorher musste eine Datei in zwei aufeinanderfolgenden Durchlaeufen
// unveraendert dastehen. Das kostete bei jeder Aufnahme einen zusaetzlichen
// Takt, obwohl die Vollstaendigkeit am Bild selbst ablesbar ist.
func TestWatcherDeliversOnFirstScan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "capture.jpg"), testJPEG(t))

	h := newTestHarness(t, dir)
	h.watcher.scan()
	h.drain()

	if len(h.delivered) != 1 {
		t.Fatal("a complete capture must be delivered without waiting for a second scan")
	}
}

// TestWatcherIgnoresTruncatedCapture deckt den halb uebertragenen Fall ab.
//
// Stockte der USB-Transfer eine Sekunde lang, sahen zwei Durchlaeufe dieselbe
// Groesse und ein abgeschnittenes JPEG ging an das Geraet.
func TestWatcherIgnoresTruncatedCapture(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture.jpg")
	full := testJPEG(t)
	writeFile(t, path, full[:len(full)/2])

	h := newTestHarness(t, dir)
	h.pump()
	if len(h.delivered) != 0 {
		t.Fatal("truncated capture was delivered")
	}

	// Sobald der Rest eingetroffen ist, muss es ohne Zutun weitergehen.
	writeFile(t, path, full)
	h.pump()
	if len(h.delivered) != 1 || !bytes.Equal(h.delivered[0].data, full) {
		t.Fatal("completed capture was not delivered afterwards")
	}
}

// TestWorkerRetriesFailedDelivery deckt ab, dass eine abgelehnte Aufnahme
// nicht verloren geht: Die Datei bleibt liegen, und nach der Wartezeit kommt
// sie erneut dran - genau einmal angenommen, nicht einmal je Versuch.
func TestWorkerRetriesFailedDelivery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture.jpg")
	writeFile(t, path, testJPEG(t))

	h := newTestHarness(t, dir)
	h.deliverErr = errors.New("device busy")
	h.pump()

	if len(h.delivered) != 0 || h.attempts != 1 {
		t.Fatalf("deliveries=%d attempts=%d, want 0/1", len(h.delivered), h.attempts)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("eine abgelehnte Aufnahme muss liegen bleiben: %v", err)
	}

	h.deliverErr = nil
	h.retry()

	if len(h.delivered) != 1 || h.attempts != 2 {
		t.Fatalf("deliveries=%d attempts=%d, want 1/2 - ein Bild darf nur einmal angenommen werden", len(h.delivered), h.attempts)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("delivered file remains: %v", err)
	}
}

// TestWorkerBacksOffBetweenDeliveryAttempts haelt fest, dass ein Geraet, das
// gerade nichts annimmt, nicht mehr im Sekundentakt bestuermt wird.
func TestWorkerBacksOffBetweenDeliveryAttempts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "capture.jpg"), testJPEG(t))

	h := newTestHarness(t, dir)
	h.deliverErr = errors.New("device busy")
	h.pump()

	// Ein sofortiger weiterer Durchlauf darf nichts ausloesen, die Wartezeit
	// laeuft noch.
	h.worker.work(context.Background())
	if h.attempts != 1 {
		t.Fatalf("attempts=%d, want 1 - der Wiederholungsversuch kam zu frueh", h.attempts)
	}
}

// TestWorkerDoesNotRedeliverWhenRemoveFails deckt ab, dass die uebergebene
// Datei nicht geloescht werden kann.
//
// Der Auftrag bleibt dann offen und wird erneut angefasst. Vorher bedeutete
// das einen weiteren Ausdruck je Sekunde, solange die Datei liegen blieb -
// und im Kiosk druckt das Geraet jede angenommene Aufnahme.
func TestWorkerDoesNotRedeliverWhenRemoveFails(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("Verzeichnisrechte halten hier niemanden vom Loeschen ab")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "capture.jpg")
	writeFile(t, path, testJPEG(t))

	// Ein schreibgeschuetztes Verzeichnis laesst das Lesen zu, das Loeschen
	// aber nicht.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	h := newTestHarness(t, dir)
	h.pump()
	h.retry()
	h.retry()

	if h.attempts != 1 {
		t.Fatalf("attempts=%d, want 1 - ein liegengebliebenes Bild darf nicht erneut uebergeben werden", h.attempts)
	}
	if len(h.worker.pending) != 1 {
		t.Fatalf("pending=%d, want 1 - das Loeschen muss weiter versucht werden", len(h.worker.pending))
	}

	// Sobald das Loeschen wieder geht, raeumt der Worker auf, ohne ein
	// weiteres Mal zu uebergeben.
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	h.retry()

	if h.attempts != 1 || len(h.worker.pending) != 0 {
		t.Fatalf("attempts=%d pending=%d, want 1/0", h.attempts, len(h.worker.pending))
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("delivered file remains: %v", err)
	}
}

// TestWorkerGivesUpAfterRetryLimit stellt sicher, dass ein dauerhaft
// unbrauchbares Bild die Warteschlange nicht fuer immer belegt.
func TestWorkerGivesUpAfterRetryLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture.jpg")
	writeFile(t, path, testJPEG(t))

	h := newTestHarness(t, dir)
	h.deliverErr = errors.New("device refuses")
	h.pump()

	for range retryLimit + 2 {
		h.retry()
	}

	if len(h.worker.pending) != 0 {
		t.Fatalf("pending=%d, want 0 - ein hoffnungsloser Auftrag muss aufgegeben werden", len(h.worker.pending))
	}

	// Aufgeben heisst liegen lassen: Der Durchlauf darf die Datei nicht
	// erneut einreihen, sonst begaennen dieselben Fehlversuche von vorn.
	before := h.attempts
	h.pump()
	if h.attempts != before {
		t.Fatalf("attempts=%d, want %d - eine aufgegebene Datei wurde erneut eingereiht", h.attempts, before)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("aufgegebene Datei muss zur Rettung liegen bleiben: %v", err)
	}
}

// TestStatusReportsCaptures prueft, dass der Startbildschirm etwas zu zeigen
// bekommt, sobald ein Bild angekommen ist.
func TestStatusReportsCaptures(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "capture.jpg"), testJPEG(t))

	h := newTestHarness(t, dir)
	h.pump()

	got := h.status.get()
	if got.Captures != 1 || got.LastCapture.IsZero() {
		t.Fatalf("status = %+v, want one capture with a timestamp", got)
	}
}

func TestComplete(t *testing.T) {
	dir := t.TempDir()
	full := testJPEG(t)

	whole := filepath.Join(dir, "whole.jpg")
	writeFile(t, whole, full)
	if !complete(whole) {
		t.Fatal("a whole jpeg was reported as incomplete")
	}

	half := filepath.Join(dir, "half.jpg")
	writeFile(t, half, full[:len(full)/2])
	if complete(half) {
		t.Fatal("a truncated jpeg was reported as complete")
	}

	empty := filepath.Join(dir, "empty.jpg")
	writeFile(t, empty, nil)
	if complete(empty) {
		t.Fatal("an empty file was reported as complete")
	}
}

// harness setzt Watcher und Worker so zusammen, wie sie im Betrieb laufen,
// laesst sie aber schrittweise statt nebenlaeufig arbeiten.
type harness struct {
	watcher *watcher
	worker  *worker
	status  *status
	queue   chan string

	// delivered sind die vom Geraet angenommenen Aufnahmen, attempts alle
	// Uebergabeversuche einschliesslich der abgelehnten.
	delivered  []delivery
	attempts   int
	deliverErr error
}

type delivery struct {
	name string
	data []byte
}

func newTestHarness(t *testing.T, dir string) *harness {
	t.Helper()

	h := &harness{status: &status{}, queue: make(chan string, queueDepth)}
	h.watcher = &watcher{dir: dir, states: map[string]fileState{}, taken: map[string]bool{}, queue: h.queue}
	h.worker = &worker{
		load:   func() Options { return Options{} },
		status: h.status,
		queue:  h.queue,
		done:   h.watcher.release,
		deliver: func(name string, data []byte) error {
			h.attempts++
			if h.deliverErr != nil {
				return h.deliverErr
			}
			h.delivered = append(h.delivered, delivery{name: name, data: data})
			return nil
		},
	}
	return h
}

// pump laesst einen Durchlauf und die daraus entstehende Arbeit ablaufen.
func (h *harness) pump() {
	h.watcher.scan()
	h.drain()
}

func (h *harness) drain() {
	for {
		select {
		case path := <-h.queue:
			h.worker.pending = append(h.worker.pending, &job{path: path})
		default:
			h.worker.work(context.Background())
			return
		}
	}
}

// retry laesst die Wartezeit verstreichen und versucht es erneut.
func (h *harness) retry() {
	for _, j := range h.worker.pending {
		j.nextTry = time.Time{}
	}
	h.worker.work(context.Background())
}

func newTestSupervisor(dir string, runner commandRunner) *supervisor {
	return &supervisor{
		dir: dir, runner: runner, status: &status{},
		load: func() Options { return Options{DetectInterval: time.Millisecond} },
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.Set(x, y, color.Black)
		}
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, nil); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

type recordedCall struct {
	name string
	args []string
}

type recordingRunner struct {
	output []byte
	runErr error

	// runFor ist die Zeit, die ein Lauf vortaeuscht. Daran entscheidet sich,
	// ob der Supervisor einen Startfehler oder einen Abriss im Betrieb sieht.
	runFor time.Duration

	calls []recordedCall
}

func (r *recordingRunner) Output(context.Context, string, ...string) ([]byte, error) {
	return r.output, nil
}

func (r *recordingRunner) Run(_ context.Context, _ io.Writer, name string, args ...string) error {
	r.calls = append(r.calls, recordedCall{name: name, args: append([]string(nil), args...)})
	if r.runFor > 0 {
		time.Sleep(r.runFor)
	}
	return r.runErr
}
