package printing

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	lpExecutable                 = "lp"
	rastertogutenprintExecutable = "/usr/lib/cups/filter/rastertogutenprint.5.3"
	cupsPPDDirectory             = "/etc/cups/ppd"
)

// Result ist die Quittung der Übergabe an den Drucker.
type Result struct {
	// JobID ist die Kennung in der Druckerwarteschlange, z. B. "CZ01-7".
	// Sie erlaubt es, den weiteren Verlauf zu verfolgen und den Auftrag
	// notfalls im Terminal wiederzufinden.
	JobID string

	// Message ist die Rückmeldung für die Oberfläche.
	Message string
}

// Printer ist der Ausgabekanal für ein fertig gerendertes Bild.
//
// Die Abstraktion erlaubt es, die Domäne ohne echten Drucker zu testen und
// später weitere Backends (z. B. direkt via IPP) zu ergänzen.
type Printer interface {
	// Print übergibt die JPEG-Daten an den Drucker.
	//
	// Ein fehlerfreier Rückgabewert bedeutet ausdrücklich nur, dass der
	// Auftrag angenommen wurde – nicht, dass gedruckt wurde. Ob tatsächlich
	// Papier entstanden ist, beantwortet erst [Tracker.Await].
	Print(ctx context.Context, jpeg []byte, name string) (Result, error)

	// Name der Zielwarteschlange, für die Anzeige.
	Name() string
}

// Tracker wird von Druckern implementiert, die Auskunft über ihren Zustand
// und über den Verbleib übergebener Aufträge geben können.
//
// Ohne diese Auskunft bliebe die Fotobox blind: lp bestätigt lediglich die
// Annahme, während Filter, Treiber oder Gerät den Auftrag danach noch
// verwerfen können.
type Tracker interface {
	// Await wartet, bis der Auftrag abgeschlossen ist, und meldet den Ausgang.
	Await(ctx context.Context, jobID string) Outcome

	// Status beschreibt den aktuellen Zustand des Druckers.
	Status(ctx context.Context) PrinterStatus
}

// Canceller wird von Druckern implementiert, die einen bereits übergebenen
// Auftrag wieder zurücknehmen können.
//
// Ohne diese Fähigkeit kann die Fotobox einen Auftrag nur hinzufügen, nie
// entfernen. Jeder aufgegebene Auftrag bliebe dann in der Warteschlange des
// Druckdienstes zurück und käme dort irgendwann von selbst zum Ausdruck.
type Canceller interface {
	// Cancel nimmt den Auftrag aus der Warteschlange. Ein unbekannter oder
	// bereits abgeschlossener Auftrag ist kein Fehler.
	Cancel(ctx context.Context, jobID string) error
}

// CUPSPrinter druckt über das lp-Kommando des lokalen CUPS.
//
// Bewusst wird lp und nicht libcups verwendet: das Kommando ist auf jedem
// Linux-Desktop vorhanden, benötigt keine cgo-Abhängigkeit und ist im
// Fehlerfall über die üblichen CUPS-Werkzeuge nachvollziehbar.
type CUPSPrinter struct {
	// Queue ist der Name der CUPS-Warteschlange, z. B. "CZ01".
	Queue string

	// PageSize ist die PPD-Bezeichnung des Papierformats.
	// Leer bedeutet [CupsPageSize].
	PageSize string

	// Laminate ist das Oberflächenfinish. Leer überlässt dem Treiber die
	// Vorgabe, beim CZ-01 also glänzend.
	Laminate string

	// PrintSpeed steuert das Tempo des Thermokopfes. Leer bedeutet
	// [SpeedLow]: Langsam gedruckt fällt die Farbe kräftiger aus, was bei
	// diesem Gerät den Unterschied zwischen satten und verwaschenen Farben
	// ausmacht.
	PrintSpeed string
}

func (p CUPSPrinter) Name() string { return p.Queue }

func (p CUPSPrinter) Print(ctx context.Context, jpg []byte, name string) (Result, error) {
	f, err := os.CreateTemp("", "eventprint-*.jpg")
	if err != nil {
		return Result{}, fmt.Errorf("cannot create temp file: %w", err)
	}

	tmp := f.Name()
	// Die Datei ist nur die kontrollierte Eingabe für unseren Rasterencoder.
	defer func() {
		_ = os.Remove(tmp)
	}()

	if _, err := f.Write(jpg); err != nil {
		_ = f.Close()
		return Result{}, fmt.Errorf("cannot write temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		return Result{}, fmt.Errorf("cannot close temp file: %w", err)
	}

	title := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if title == "" {
		title = "eventprint"
	}

	stream, err := p.filterJPEG(ctx, tmp, title)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = os.Remove(stream) }()

	cmd := exec.CommandContext(ctx, lpExecutable, "-d", p.Queue, "-t", title, "-o", "raw", stream)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return Result{}, fmt.Errorf("lp failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	message := strings.TrimSpace(string(out))

	return Result{JobID: ParseRequestID(message, p.Queue), Message: message}, nil
}

func (p CUPSPrinter) filterJPEG(ctx context.Context, jpegFile, title string) (string, error) {
	pageSize := p.PageSize
	if pageSize == "" {
		pageSize = CupsPageSize
	}
	if pageSize != CupsPageSize {
		return "", fmt.Errorf("unsupported CZ-01 page size %q: only %s has a verified raster", pageSize, CupsPageSize)
	}
	if filepath.Base(p.Queue) != p.Queue {
		return "", fmt.Errorf("invalid CUPS queue name %q", p.Queue)
	}

	ppd := filepath.Join(cupsPPDDirectory, p.Queue+".ppd")
	if err := validateCZ01PPD(ppd); err != nil {
		return "", err
	}

	jpegData, err := os.ReadFile(jpegFile)
	if err != nil {
		return "", fmt.Errorf("cannot read print JPEG: %w", err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(jpegData))
	if err != nil {
		return "", fmt.Errorf("cannot read print JPEG dimensions: %w", err)
	}
	if !NativeRaster4x6.Matches(cfg.Width, cfg.Height) {
		return "", fmt.Errorf("refusing to print %dx%d JPEG: CZ-01 requires %dx%d or %dx%d",
			cfg.Width, cfg.Height, NativeRaster4x6.Width, NativeRaster4x6.Height,
			NativeRaster4x6.Height, NativeRaster4x6.Width)
	}

	raster, err := os.CreateTemp("", "eventprint-*.raster")
	if err != nil {
		return "", fmt.Errorf("cannot create CUPS raster: %w", err)
	}
	rasterName := raster.Name()
	defer func() {
		_ = raster.Close()
		_ = os.Remove(rasterName)
	}()
	if err := writeCZ01Raster(raster, jpegData); err != nil {
		return "", err
	}
	if err := raster.Close(); err != nil {
		return "", fmt.Errorf("cannot close CUPS raster: %w", err)
	}
	if err := validateCZ01Raster(rasterName); err != nil {
		return "", fmt.Errorf("refusing invalid raster before Gutenprint: %w", err)
	}

	stream, err := os.CreateTemp("", "eventprint-*.prn")
	if err != nil {
		return "", fmt.Errorf("cannot create print stream: %w", err)
	}
	streamName := stream.Name()
	options := strings.Join(p.driverOptions(), " ")
	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, rastertogutenprintExecutable, "1", "eventprint", title, "1", options, rasterName)
	cmd.Env = append(os.Environ(), "PPD="+ppd)
	cmd.Stdout = stream
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = stream.Close()
		_ = os.Remove(streamName)
		return "", fmt.Errorf("system Gutenprint filter failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if err := stream.Close(); err != nil {
		_ = os.Remove(streamName)
		return "", fmt.Errorf("cannot close print stream: %w", err)
	}
	if err := validateCZ01PrintStream(streamName); err != nil {
		_ = os.Remove(streamName)
		return "", fmt.Errorf("refusing invalid stream before printer: %w", err)
	}
	return streamName, nil
}

func (p CUPSPrinter) driverOptions() []string {
	pageSize := p.PageSize
	if pageSize == "" {
		pageSize = CupsPageSize
	}
	speed := p.PrintSpeed
	if speed == "" {
		speed = SpeedLow
	}

	options := []string{
		"PageSize=" + pageSize,
		"StpImageType=Photo",
		"StpPrintSpeed=" + speed,
	}
	if p.Laminate != "" {
		options = append(options, "StpLaminate="+p.Laminate)
	}
	return options
}

// ParseRequestID liest die Auftragskennung aus der Bestätigung von lp.
//
// Die Meldung ist übersetzt – "Anfrage-ID ist CZ01-7 (1 Datei(en))" bzw.
// "request id is CZ01-7 (1 file(s))" –, die Kennung selbst folgt aber immer
// dem Muster <warteschlange>-<nummer>. Danach wird gesucht, statt sich auf
// die Wortstellung zu verlassen.
func ParseRequestID(out, queue string) string {
	prefix := queue + "-"

	for _, field := range strings.Fields(out) {
		if !strings.HasPrefix(field, prefix) {
			continue
		}

		number := strings.TrimPrefix(field, prefix)
		if number == "" || strings.TrimLeft(number, "0123456789") != "" {
			continue
		}

		return field
	}

	return ""
}

// Await verfolgt den Auftrag, bis CUPS ihn abgeschlossen hat.
func (p CUPSPrinter) Await(ctx context.Context, jobID string) Outcome {
	if jobID == "" {
		// Ohne Kennung lässt sich nichts nachverfolgen. Das als Erfolg zu
		// werten wäre die alte, falsche Annahme – deshalb ehrlich benennen.
		return Outcome{
			Done:    true,
			Success: false,
			Reason:  "unknown",
			Message: "lp hat keine Auftragskennung gemeldet, der Verbleib des Auftrags ist unbekannt.",
		}
	}

	return AwaitJob(ctx, p.Queue, jobID, AwaitTimeout, AwaitInterval)
}

// Status beschreibt den Zustand der Warteschlange.
func (p CUPSPrinter) Status(ctx context.Context) PrinterStatus {
	return QueryPrinter(ctx, p.Queue)
}

// Cancel nimmt einen übergebenen Auftrag aus der CUPS-Warteschlange.
func (p CUPSPrinter) Cancel(ctx context.Context, jobID string) error {
	return CancelJob(ctx, jobID)
}

// Zeitgrenzen für die Nachverfolgung. Ein 10x15-Ausdruck braucht am CZ-01
// etwa 15 Sekunden; die großzügige Grenze deckt eine gefüllte Warteschlange
// ab, ohne die Fotobox bei einem hängenden Auftrag dauerhaft zu blockieren.
const (
	AwaitTimeout  = 5 * time.Minute
	AwaitInterval = 2 * time.Second
)

// DiscardPrinter verwirft alle Aufträge und protokolliert sie lediglich.
// Damit lässt sich die Fotobox ohne angeschlossenen Drucker aufbauen und
// vorführen.
type DiscardPrinter struct{}

func (DiscardPrinter) Name() string { return TestModeName }

func (DiscardPrinter) Print(_ context.Context, jpg []byte, name string) (Result, error) {
	slog.Info("discarding print job", "name", name, "bytes", len(jpg))

	return Result{Message: fmt.Sprintf("Testmodus: %d Bytes verworfen", len(jpg))}, nil
}

// Await meldet den Testbetrieb sofort als abgeschlossen.
func (DiscardPrinter) Await(context.Context, string) Outcome {
	return Outcome{Done: true, Success: true, Reason: "test-mode"}
}

// Status meldet den Testbetrieb als bereit, damit die Oberfläche keinen
// Druckerfehler anzeigt, wo gar kein Drucker konfiguriert ist.
func (DiscardPrinter) Status(context.Context) PrinterStatus {
	return PrinterStatus{Queue: TestModeName, Exists: true, Enabled: true, Accepting: true}
}

// Cancel hat im Testbetrieb nichts zurückzunehmen.
func (DiscardPrinter) Cancel(context.Context, string) error { return nil }
