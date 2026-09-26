package printing_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/permission"
	"go.wdy.de/nago/pkg/blob/mem"
	"go.wdy.de/nago/pkg/data/json"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
	"github.com/torbenschinke/eventprint/requirements/fun/druck"
)

// fakePrinter ersetzt CUPS. Er nimmt jeden Auftrag an und meldet den zuvor
// festgelegten Ausgang – so lässt sich der Fall nachstellen, der uns die
// Ausdrucke gekostet hat: lp bestätigt, der Drucker verwirft.
//
// Der Worker ruft ihn aus seiner eigenen Goroutine auf, der Test liest und
// ändert ihn aus einer anderen. Deshalb läuft jeder Zugriff über den Mutex,
// sonst meldet -race zu Recht einen Wettlauf, den es im Betrieb nicht gibt.
type fakePrinter struct {
	mutex    sync.Mutex
	outcome  printing.Outcome
	printErr error
	printed  int

	// canceled hält fest, welche Aufträge zurückgenommen wurden. Genau daran
	// entscheidet sich, ob eine Wiederholung ein zweites Blatt erzeugt.
	canceled []string

	// status ist der Zustand, den der Drucker meldet. Der Nullwert bedeutet
	// "bereit".
	status *printing.PrinterStatus

	// hold hält Await fest, bis der Test ihn schließt. So bleibt ein
	// Auftrag im Zustand "Druckt" stehen, und alles dahinter wartet – der
	// Moment, in dem jemand am Kiosk auf "Abbrechen" tippt.
	hold chan struct{}

	// awaiting meldet die Druckerkennung, sobald Await betreten wurde.
	awaiting chan string
}

func (p *fakePrinter) Name() string { return "Fake" }

func (p *fakePrinter) Print(context.Context, []byte, string) (printing.Result, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.printErr != nil {
		return printing.Result{}, p.printErr
	}

	p.printed++

	// Fortlaufend wie bei CUPS, damit bei mehreren Blättern erkennbar ist,
	// welcher Auftrag storniert wurde.
	return printing.Result{JobID: fmt.Sprintf("Fake-%d", p.printed), Message: "angenommen"}, nil
}

func (p *fakePrinter) Await(ctx context.Context, jobID string) printing.Outcome {
	p.mutex.Lock()
	hold, awaiting := p.hold, p.awaiting
	p.mutex.Unlock()

	if awaiting != nil {
		awaiting <- jobID
	}

	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
		}
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.outcome
}

func (p *fakePrinter) Status(context.Context) printing.PrinterStatus {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.status != nil {
		return *p.status
	}

	return printing.PrinterStatus{Queue: "Fake", Exists: true, Enabled: true, Accepting: true}
}

func (p *fakePrinter) Cancel(_ context.Context, jobID string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.canceled = append(p.canceled, jobID)

	return nil
}

func (p *fakePrinter) setOutcome(o printing.Outcome) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.outcome = o
}

func (p *fakePrinter) setStatus(s printing.PrinterStatus) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.status = &s
}

func (p *fakePrinter) printedCount() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.printed
}

func (p *fakePrinter) canceledJobs() []string {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return slices.Clone(p.canceled)
}

// holding lässt jeden Auftrag in Await warten, bis release aufgerufen wird.
func (p *fakePrinter) holding() (release func()) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.hold = make(chan struct{})
	p.awaiting = make(chan string, 64)
	hold := p.hold

	var once sync.Once

	return func() { once.Do(func() { close(hold) }) }
}

// waitAwaiting wartet, bis der Worker einen Auftrag beim Drucker abgegeben
// hat und auf dessen Ausgang wartet.
func (p *fakePrinter) waitAwaiting(t *testing.T) string {
	t.Helper()

	select {
	case id := <-p.awaiting:
		return id
	case <-time.After(30 * time.Second):
		t.Fatal("der Worker hat keinen Auftrag an den Drucker übergeben")
		return ""
	}
}

var succeeded = printing.Outcome{Done: true, Success: true, Reason: "job-completed-successfully"}

// library ist eine Handvoll echter Originale auf der Platte, so wie der
// Foto-Kontext sie über [photo.Locate] herausgibt.
//
// Die Bilder sind klein und einfarbig: Der Worker rendert trotzdem jedes
// Blatt in voller Druckauflösung, und mit dem 16-Megapixel-Beispielbild
// dauerten die Tests mit mehreren Blättern unnötig lange.
type library struct {
	mutex sync.Mutex
	files map[photo.ID]photo.Location
	ids   []photo.ID
}

func newLibrary(t *testing.T, n int, w, h int) *library {
	t.Helper()

	dir := t.TempDir()
	lib := &library{files: map[photo.ID]photo.Location{}}

	for i := range n {
		id := photo.ID(fmt.Sprintf("foto-%d", i+1))
		path := filepath.Join(dir, string(id)+".jpg")

		c := color.RGBA{R: uint8(40 * i), G: uint8(200 - 30*i), B: 120, A: 0xFF}
		if err := os.WriteFile(path, encodeJPEG(t, solid(w, h, c)), 0o600); err != nil {
			t.Fatal(err)
		}

		lib.files[id] = photo.Location{
			Photo: photo.Photo{ID: id, CreatedAt: time.Date(2026, 9, 5, 20, 15, 0, 0, time.Local)},
			Path:  path,
		}
		lib.ids = append(lib.ids, id)
	}

	return lib
}

// locate verhält sich wie der echte Anwendungsfall: Verschwundene Fotos
// werden übergangen, nicht als Fehler gemeldet.
func (l *library) locate(subject permission.Auditable, ids ...photo.ID) ([]photo.Location, error) {
	if err := subject.Audit(photo.PermLocate); err != nil {
		return nil, err
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	var out []photo.Location
	for _, id := range ids {
		if loc, ok := l.files[id]; ok {
			out = append(out, loc)
		}
	}

	return out, nil
}

func (l *library) remove(id photo.ID) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	delete(l.files, id)
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

// observer sammelt, was [printing.Options.Observe] erfährt.
type observer struct {
	mutex sync.Mutex
	jobs  []printing.Job
}

func (o *observer) observe(job printing.Job) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.jobs = append(o.jobs, job)
}

func (o *observer) seen() []printing.Job {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	return slices.Clone(o.jobs)
}

// fixture bündelt alles, was ein Test über den laufenden Druck wissen muss.
type fixture struct {
	uc       printing.UseCases
	repo     printing.Repository
	printer  *fakePrinter
	photos   *library
	observed *observer
}

// newRepo liefert ein Repository über einen Speicher-Blobstore: dieselbe
// Serialisierung wie im Betrieb, nur ohne Datei auf der Platte. Gerade die
// Serialisierung ist wichtig – ein Layout, das nicht durch JSON kommt, würde
// nach einem Neustart anders gedruckt als bestellt.
func newRepo() printing.Repository {
	return json.NewSloppyJSONRepository[printing.Job, printing.JobID](mem.NewBlobStore("printjob"))
}

func newFixture(t *testing.T, printer *fakePrinter, configure ...func(*printing.Options)) fixture {
	t.Helper()

	f := fixture{
		repo:     newRepo(),
		printer:  printer,
		photos:   newLibrary(t, 6, 300, 450),
		observed: &observer{},
	}

	// Erst nach dem Verzeichnis der Fotos registriert, damit der Worker
	// beendet ist, bevor die Dateien verschwinden.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	opts := printing.Options{
		Repository:     f.repo,
		Printer:        printer,
		Locate:         f.photos.locate,
		MaxKioskCopies: func() int { return 3 },
		Observe:        f.observed.observe,
	}

	for _, c := range configure {
		c(&opts)
	}

	f.repo = opts.Repository
	f.uc = printing.NewUseCases(ctx, opts)

	return f
}

var su = permission.SU()

// printOne druckt ein Foto als Einzelbild und liefert die Auftragskennung.
func (f fixture) printOne(t *testing.T, idx int) printing.JobID {
	t.Helper()

	batch, err := f.uc.Print(su, printing.PrintCmd{
		Photos: []photo.ID{f.photos.ids[idx]},
		Layout: printing.DefaultLayout(),
		Copies: 1,
	})
	if err != nil {
		t.Fatalf("Print: %v", err)
	}

	if len(batch.Jobs) != 1 {
		t.Fatalf("ein Foto als Einzelbild ergab %d Aufträge, erwartet 1", len(batch.Jobs))
	}

	return batch.Jobs[0]
}

func (f fixture) job(t *testing.T, id printing.JobID) printing.Job {
	t.Helper()

	opt, err := f.uc.FindJobByID(su, id)
	if err != nil {
		t.Fatalf("FindJobByID: %v", err)
	}

	if opt.IsNone() {
		t.Fatalf("Auftrag %s fehlt", id)
	}

	return opt.Unwrap()
}

// awaitJob wartet, bis der Auftrag abgeschlossen ist.
func (f fixture) awaitJob(t *testing.T, id printing.JobID) printing.Job {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		if job := f.job(t, id); job.State.Done() {
			return job
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("der Auftrag wurde nicht abgeschlossen")

	return printing.Job{}
}

// TestWorkerReportsRejectionByPrinter ist der Kern dieser Absicherung.
//
// Vorher galt ein Auftrag als "Fertig", sobald lp ihn angenommen hatte. Ein
// vom Backend verworfener Auftrag – etwa weil CUPS den Dateityp nicht
// erkennt – erschien dadurch als Erfolg, während nichts gedruckt wurde.
func TestWorkerReportsRejectionByPrinter(t *testing.T) {
	printer := &fakePrinter{outcome: printing.Outcome{
		Done:    true,
		Success: false,
		Reason:  "canceled-at-device",
		Message: "The print file could not be opened.",
	}}

	f := newFixture(t, printer)
	job := f.awaitJob(t, f.printOne(t, 0))

	if job.State != printing.StateFailed {
		t.Errorf("Zustand = %s, erwartet %s – ein verworfener Auftrag darf nicht als Erfolg gelten",
			job.State, printing.StateFailed)
	}

	if job.Reason != "canceled-at-device" {
		t.Errorf("Reason = %q, erwartet %q", job.Reason, "canceled-at-device")
	}

	if job.Message != "The print file could not be opened." {
		t.Errorf("Message = %q – die Ursache von CUPS muss durchgereicht werden", job.Message)
	}

	if job.PrinterJob != "Fake-1" {
		t.Errorf("PrinterJob = %q, erwartet %q – ohne Kennung ist der Auftrag nicht auffindbar", job.PrinterJob, "Fake-1")
	}

	if job.FinishedAt.IsZero() {
		t.Error("ein abgeschlossener Auftrag braucht einen Endzeitpunkt, sonst läuft die Dauer ewig weiter")
	}
}

// TestWorkerExplainsRejectionWithoutMessage: CUPS nennt nicht immer einen
// Klartext. Ein Fehler ohne jede Erklärung ließe die Betreuung raten.
func TestWorkerExplainsRejectionWithoutMessage(t *testing.T) {
	printer := &fakePrinter{outcome: printing.Outcome{Done: true, Reason: "aborted-by-system"}}

	f := newFixture(t, printer)
	job := f.awaitJob(t, f.printOne(t, 0))

	if job.State != printing.StateFailed || job.Message == "" {
		t.Fatalf("Zustand %s, Meldung %q – erwartet Fehler mit Erklärung", job.State, job.Message)
	}
}

// TestWorkerReportsSuccess ist die Gegenprobe.
func TestWorkerReportsSuccess(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}

	f := newFixture(t, printer)
	id := f.printOne(t, 0)
	job := f.awaitJob(t, id)

	if job.State != printing.StateDone {
		t.Errorf("Zustand = %s, erwartet %s (%s)", job.State, printing.StateDone, job.Message)
	}

	if printer.printedCount() != 1 {
		t.Errorf("es wurden %d Aufträge übergeben, erwartet 1", printer.printedCount())
	}

	// Was gedruckt wurde, muss am Auftrag stehen. Nur so lässt sich ein
	// Blatt aus der Druckstatus-Seite wiederholen, ohne die Fotos neu zu
	// suchen.
	if !slices.Equal(job.Photos, []photo.ID{f.photos.ids[0]}) {
		t.Errorf("Photos = %v, erwartet [%s]", job.Photos, f.photos.ids[0])
	}

	if job.Printer != "Fake" || job.Sheet != 1 || job.Sheets != 1 || job.Batch == "" {
		t.Errorf("Drucker %q, Blatt %d/%d, Batch %q – unvollständig", job.Printer, job.Sheet, job.Sheets, job.Batch)
	}

	spec.Verified(t, druck.RDruckAuftrag)
}

// TestWorkerReportsSubmissionFailure deckt den Fall ab, dass schon die
// Übergabe scheitert – etwa weil lp fehlt.
func TestWorkerReportsSubmissionFailure(t *testing.T) {
	printer := &fakePrinter{printErr: errors.New("lp failed: exec: \"lp\": executable file not found in $PATH")}

	f := newFixture(t, printer)
	job := f.awaitJob(t, f.printOne(t, 0))

	if job.State != printing.StateFailed {
		t.Errorf("Zustand = %s, erwartet %s", job.State, printing.StateFailed)
	}

	if job.Message == "" {
		t.Error("die Ursache muss an der Oberfläche ankommen")
	}
}

// TestWorkerReportsVanishedPhoto: Zwischen Tipp und Druck kann ein Foto
// gelöscht werden. Der Auftrag muss dann scheitern, statt ein leeres oder
// graues Blatt auszugeben.
func TestWorkerReportsVanishedPhoto(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}
	release := printer.holding()
	defer release()

	f := newFixture(t, printer)

	// Der erste Auftrag belegt den Worker, damit das Foto des zweiten
	// verschwinden kann, bevor er an der Reihe ist.
	first := f.printOne(t, 0)
	printer.waitAwaiting(t)

	second := f.printOne(t, 1)
	f.photos.remove(f.photos.ids[1])
	release()

	f.awaitJob(t, first)
	job := f.awaitJob(t, second)

	if job.State != printing.StateFailed || job.Message == "" {
		t.Fatalf("Zustand %s, Meldung %q – erwartet Fehler mit Erklärung", job.State, job.Message)
	}

	if printer.printedCount() != 1 {
		t.Fatalf("es wurden %d Blätter übergeben, erwartet 1", printer.printedCount())
	}
}

// TestRetryRequeuesFailedJob prüft den Ablauf nach einem Papierwechsel.
func TestRetryRequeuesFailedJob(t *testing.T) {
	printer := &fakePrinter{outcome: printing.Outcome{Done: true, Reason: "canceled-at-device"}}

	f := newFixture(t, printer)
	id := f.printOne(t, 0)
	f.awaitJob(t, id)

	// Papier nachgelegt: ab jetzt gelingt der Druck.
	printer.setOutcome(succeeded)

	if err := f.uc.Retry(su, id); err != nil {
		t.Fatalf("Retry: %v", err)
	}

	job := f.awaitJob(t, id)

	if job.State != printing.StateDone {
		t.Errorf("Zustand nach Wiederholung = %s, erwartet %s", job.State, printing.StateDone)
	}

	if printer.printedCount() != 2 {
		t.Errorf("es wurden %d Aufträge übergeben, erwartet 2", printer.printedCount())
	}

	// Die Wiederholung druckt dasselbe Blatt, nicht irgendeines.
	if !slices.Equal(job.Photos, []photo.ID{f.photos.ids[0]}) {
		t.Errorf("Photos nach Wiederholung = %v", job.Photos)
	}
}

// TestRetryCancelsPreviousPrinterJob sichert die zweite Hälfte des
// Doppeldrucks ab.
//
// Ein fehlgeschlagener Auftrag kann im Druckdienst weiterhin anhängig sein –
// nach einem Timeout ist das sogar der Regelfall. Wird er vor der
// Wiederholung nicht zurückgenommen, existieren zwei Aufträge für dasselbe
// Bild, und der Drucker gibt es zweimal aus.
func TestRetryCancelsPreviousPrinterJob(t *testing.T) {
	printer := &fakePrinter{outcome: printing.Outcome{Done: true, Reason: "timeout"}}

	f := newFixture(t, printer)
	id := f.printOne(t, 0)
	f.awaitJob(t, id)

	printer.setOutcome(succeeded)

	if err := f.uc.Retry(su, id); err != nil {
		t.Fatalf("Retry: %v", err)
	}

	f.awaitJob(t, id)

	if got := printer.canceledJobs(); len(got) != 1 || got[0] != "Fake-1" {
		t.Fatalf("stornierte Aufträge = %v, erwartet [Fake-1]", got)
	}

	spec.Verified(t, druck.RDruckWiederholung)
}

// TestRetryRefusesRunningJob: Ein Doppeltipp auf "Wiederholen" während des
// Drucks darf kein zweites Blatt ergeben.
func TestRetryRefusesRunningJob(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}
	release := printer.holding()
	defer release()

	f := newFixture(t, printer)
	id := f.printOne(t, 0)
	printer.waitAwaiting(t)

	if err := f.uc.Retry(su, id); err == nil {
		t.Fatal("ein laufender Auftrag ließ sich wiederholen")
	}

	if err := f.uc.Retry(su, "gibt-es-nicht"); err == nil {
		t.Fatal("ein unbekannter Auftrag ließ sich wiederholen")
	}

	release()
	f.awaitJob(t, id)

	if printer.printedCount() != 1 {
		t.Fatalf("es wurden %d Blätter übergeben, erwartet 1", printer.printedCount())
	}
}

// TestJobsAreListedNewestFirst deckt den Zustand der Warteschlange ab, wie ihn
// die Druckstatus-Seite zeigt.
func TestJobsAreListedNewestFirst(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}

	f := newFixture(t, printer)

	first := f.printOne(t, 0)
	f.awaitJob(t, first)

	// Die Kennung beginnt mit Millisekunden. Ohne diese Pause könnten beide
	// Aufträge in derselben Millisekunde entstehen, und dann entscheidet der
	// Zufallsteil über die Reihenfolge.
	time.Sleep(5 * time.Millisecond)

	second := f.printOne(t, 1)
	f.awaitJob(t, second)

	seq, err := f.uc.FindAllJobs(su)
	if err != nil {
		t.Fatalf("FindAllJobs: %v", err)
	}

	var ids []printing.JobID
	for job, err := range seq {
		if err != nil {
			t.Fatalf("FindAllJobs: %v", err)
		}

		ids = append(ids, job.ID)
	}

	if len(ids) != 2 {
		t.Fatalf("FindAllJobs lieferte %d Aufträge, erwartet 2", len(ids))
	}

	if ids[0] != second || ids[1] != first {
		t.Fatal("die Aufträge stehen nicht mit dem neuesten zuerst")
	}

	// Der einzelne Auftrag trägt den Grund, den die Oberfläche anzeigt.
	if got := f.job(t, first); got.State != printing.StateDone || got.Reason != "job-completed-successfully" {
		t.Fatalf("Zustand %q, Grund %q – erwartet fertig mit IPP-Grund", got.State, got.Reason)
	}

	missing, err := f.uc.FindJobByID(su, "gibt-es-nicht")
	if err != nil {
		t.Fatalf("FindJobByID(unbekannt): %v", err)
	}

	if missing.IsSome() {
		t.Fatal("eine unbekannte Kennung lieferte einen Auftrag")
	}

	spec.Verified(t, druck.RDruckStatus)
}

// TestRecoverStaleJobsOnRestart ist der Neustart mitten auf der Feier: Was
// beim Herunterfahren offen war, darf weder verloren gehen noch doppelt oder
// am nächsten Morgen überraschend aus dem Drucker kommen.
func TestRecoverStaleJobsOnRestart(t *testing.T) {
	repo := newRepo()
	now := time.Now()

	jobs := map[string]printing.Job{
		// Mitten im Druck unterbrochen: Ob Papier verbraucht wurde, weiß
		// niemand. Automatisch nachdrucken hieße im Zweifel doppelt drucken.
		"printing": {State: printing.StatePrinting, PrinterJob: "CZ01-1", CreatedAt: now.Add(-time.Minute)},

		// Wartet seit einer Stunde: Der Gast ist längst gegangen.
		"old": {State: printing.StateQueued, PrinterJob: "CZ01-2", CreatedAt: now.Add(-time.Hour)},

		// Wartet seit einer Minute: Das soll noch kommen, aber nur einmal.
		"young": {State: printing.StateQueued, PrinterJob: "CZ01-3", CreatedAt: now.Add(-time.Minute)},

		// Fertig ist fertig, daran ändert ein Neustart nichts.
		"done": {State: printing.StateDone, PrinterJob: "CZ01-4", Reason: "job-completed-successfully", CreatedAt: now.Add(-time.Minute), FinishedAt: now},
	}

	printer := &fakePrinter{outcome: succeeded}

	// Das Repository muss vor dem Start gefüllt sein, denn die
	// Wiederherstellung läuft genau einmal: beim Verdrahten.
	for name, job := range jobs {
		job.ID = printing.JobID(fmt.Sprintf("%013d-%s", job.CreatedAt.UnixMilli(), name))
		job.Photos = []photo.ID{"foto-1"}
		job.Layout = printing.DefaultLayout()
		job.Printer = "Fake"
		if err := repo.Save(job); err != nil {
			t.Fatal(err)
		}

		jobs[name] = job
	}

	f := newFixture(t, printer, func(o *printing.Options) { o.Repository = repo })

	for _, name := range []string{"printing", "old"} {
		// Diese beiden sind sofort nach dem Start entschieden, der Worker
		// wird für sie nicht gebraucht.
		job := f.job(t, jobs[name].ID)
		if job.State != printing.StateFailed || job.Message == "" || job.FinishedAt.IsZero() {
			t.Errorf("%s: Zustand %s, Meldung %q – erwartet Fehler mit Erklärung", name, job.State, job.Message)
		}

		if job.PrinterJob != "" {
			t.Errorf("%s: PrinterJob %q blieb stehen, obwohl er storniert ist", name, job.PrinterJob)
		}
	}

	young := f.awaitJob(t, jobs["young"].ID)
	if young.State != printing.StateDone {
		t.Errorf("der junge Auftrag wurde nicht nachgedruckt: %s (%s)", young.State, young.Message)
	}

	if done := f.job(t, jobs["done"].ID); done.State != printing.StateDone || done.PrinterJob != "CZ01-4" {
		t.Errorf("der fertige Auftrag wurde angefasst: %+v", done)
	}

	if printer.printedCount() != 1 {
		t.Errorf("es wurden %d Blätter übergeben, erwartet genau 1 (nur der junge Auftrag)", printer.printedCount())
	}

	// Alle drei offenen Aufträge gehören zu einem Lauf, dessen Ausgang
	// niemand mehr kennt. Blieben sie in CUPS, kämen sie beim nächsten
	// Erreichen des Druckers zusätzlich heraus.
	canceled := printer.canceledJobs()
	slices.Sort(canceled)
	if !slices.Equal(canceled, []string{"CZ01-1", "CZ01-2", "CZ01-3"}) {
		t.Errorf("stornierte Aufträge = %v, erwartet [CZ01-1 CZ01-2 CZ01-3]", canceled)
	}

	spec.Verified(t, druck.RDruckKeinNachdruck)
}

// TestPrintSplitsBatchIntoSheets: Fünf Fotos als Collage mit drei Feldern
// passen auf zwei Blätter, zweimal bestellt sind das vier Aufträge. Jedes
// Blatt ist ein eigener Auftrag, damit ein Papierwechsel mitten im Stapel
// genau das fehlende Blatt wiederholt.
func TestPrintSplitsBatchIntoSheets(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}
	f := newFixture(t, printer)

	layout := printing.DefaultLayout()
	layout.Format = printing.FormatCollage
	layout.Design = printing.DesignMatte
	layout.Caption = "Anna & Ben"

	ids := f.photos.ids[:5]
	batch, err := f.uc.Print(su, printing.PrintCmd{Photos: ids, Layout: layout, Copies: 2})
	if err != nil {
		t.Fatalf("Print: %v", err)
	}

	if batch.ID == "" || len(batch.Jobs) != 4 {
		t.Fatalf("Batch %q mit %d Aufträgen, erwartet 4", batch.ID, len(batch.Jobs))
	}

	want := [][]photo.ID{ids[:3], ids[:3], ids[3:], ids[3:]}
	for i, id := range batch.Jobs {
		job := f.awaitJob(t, id)

		if job.State != printing.StateDone {
			t.Errorf("Blatt %d: Zustand %s (%s)", i+1, job.State, job.Message)
		}

		if !slices.Equal(job.Photos, want[i]) {
			t.Errorf("Blatt %d: Photos = %v, erwartet %v", i+1, job.Photos, want[i])
		}

		if job.Batch != batch.ID || job.Sheet != i+1 || job.Sheets != 4 {
			t.Errorf("Blatt %d: Batch %q, Blatt %d/%d – erwartet %q, %d/4", i+1, job.Batch, job.Sheet, job.Sheets, batch.ID, i+1)
		}

		// Das Layout muss den Weg durch JSON überstehen, sonst druckt
		// die Wiederholung nach einem Neustart etwas anderes.
		if job.Layout != layout.Normalized() {
			t.Errorf("Blatt %d: Layout %+v, erwartet %+v", i+1, job.Layout, layout.Normalized())
		}
	}

	if printer.printedCount() != 4 {
		t.Errorf("es wurden %d Blätter übergeben, erwartet 4", printer.printedCount())
	}
}

// TestPrintSheetsFollowTheFormat prüft die Aufteilung der übrigen Formate:
// Passfotos zeigen viermal dasselbe Gesicht, also ein Foto je Blatt; ein
// Fotostreifen nimmt drei.
func TestPrintSheetsFollowTheFormat(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}
	release := printer.holding()
	defer release()

	f := newFixture(t, printer)

	cases := []struct {
		format printing.Format
		photos int
		want   [][]int
	}{
		{printing.FormatPassport, 3, [][]int{{0}, {1}, {2}}},
		{printing.FormatStrip, 4, [][]int{{0, 1, 2}, {3}}},
		{printing.FormatDuo, 3, [][]int{{0, 1}, {2}}},
		{printing.FormatSingle, 2, [][]int{{0}, {1}}},
	}

	for _, c := range cases {
		layout := printing.DefaultLayout()
		layout.Format = c.format

		batch, err := f.uc.Print(su, printing.PrintCmd{Photos: f.photos.ids[:c.photos], Layout: layout, Copies: 1})
		if err != nil {
			t.Fatalf("%s: Print: %v", c.format, err)
		}

		if len(batch.Jobs) != len(c.want) || layout.Sheets(c.photos) != len(c.want) {
			t.Fatalf("%s: %d Aufträge, Sheets() = %d, erwartet %d", c.format, len(batch.Jobs), layout.Sheets(c.photos), len(c.want))
		}

		for i, id := range batch.Jobs {
			var want []photo.ID
			for _, idx := range c.want[i] {
				want = append(want, f.photos.ids[idx])
			}

			if got := f.job(t, id).Photos; !slices.Equal(got, want) {
				t.Errorf("%s Blatt %d: Photos = %v, erwartet %v", c.format, i+1, got, want)
			}
		}
	}
}

// TestPrintLimitsCopies: Ein Tippfehler im Zähler soll kein ganzes
// Papierset kosten, und null Exemplare heißt nicht "gar nicht drucken".
func TestPrintLimitsCopies(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}
	release := printer.holding()
	defer release()

	f := newFixture(t, printer)

	for copies, want := range map[int]int{0: 1, -3: 1, 2: 2, 100: printing.MaxCopies} {
		batch, err := f.uc.Print(su, printing.PrintCmd{Photos: f.photos.ids[:1], Layout: printing.DefaultLayout(), Copies: copies})
		if err != nil {
			t.Fatalf("Print(%d Exemplare): %v", copies, err)
		}

		if len(batch.Jobs) != want {
			t.Errorf("%d Exemplare ergaben %d Aufträge, erwartet %d", copies, len(batch.Jobs), want)
		}
	}

	if _, err := f.uc.Print(su, printing.PrintCmd{Layout: printing.DefaultLayout(), Copies: 1}); err == nil {
		t.Error("ein Druck ohne Foto wurde angenommen")
	}
}

// TestPrintSimpleClampsCopies ist die Grenze für Gäste: Sie drucken ein
// Foto in einem Kiosk-Layout, und nie mehr Exemplare als eingestellt.
func TestPrintSimpleClampsCopies(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}
	release := printer.holding()
	defer release()

	var limit sync.Mutex
	maxCopies := 2

	f := newFixture(t, printer, func(o *printing.Options) {
		o.MaxKioskCopies = func() int {
			limit.Lock()
			defer limit.Unlock()

			return maxCopies
		}
	})

	simple := func(copies int, tpl printing.TemplateID) printing.Batch {
		t.Helper()

		batch, err := f.uc.PrintSimple(su, printing.SimpleCmd{Photo: f.photos.ids[0], Template: tpl, Copies: copies})
		if err != nil {
			t.Fatalf("PrintSimple: %v", err)
		}

		return batch
	}

	if n := len(simple(5, printing.TemplateFull).Jobs); n != 2 {
		t.Errorf("5 Exemplare bei Grenze 2 ergaben %d Aufträge", n)
	}

	if n := len(simple(0, printing.TemplateFull).Jobs); n != 1 {
		t.Errorf("0 Exemplare ergaben %d Aufträge, erwartet 1", n)
	}

	// Die Einstellung gilt ab dem nächsten Tipp, ohne Neustart.
	limit.Lock()
	maxCopies = 0
	limit.Unlock()

	if n := len(simple(5, printing.TemplateFull).Jobs); n != 1 {
		t.Errorf("eine Grenze von 0 ergab %d Aufträge – mindestens eines muss gehen, mehr nicht", n)
	}

	// Ein veralteter Wert aus einem offenen Browserfenster druckt
	// formatfüllend, statt zu scheitern.
	for tpl, want := range map[printing.TemplateID]printing.Layout{
		"gibt-es-nicht":               printing.TemplateFull.Layout(),
		printing.TemplateFull:         printing.TemplateFull.Layout(),
		printing.TemplatePassepartout: printing.TemplatePassepartout.Layout(),
		printing.TemplatePolaroid:     printing.TemplatePolaroid.Layout(),
	} {
		batch := simple(1, tpl)
		job := f.job(t, batch.Jobs[0])

		if job.Layout != want.Normalized() {
			t.Errorf("Vorlage %q: Layout %+v, erwartet %+v", tpl, job.Layout, want)
		}

		if !slices.Equal(job.Photos, f.photos.ids[:1]) {
			t.Errorf("Vorlage %q: Photos = %v", tpl, job.Photos)
		}
	}
}

// TestPrintSimpleWithoutLimitPrintsOnce: Fehlt die Einstellung ganz, ist
// ein Exemplar die sichere Annahme.
func TestPrintSimpleWithoutLimitPrintsOnce(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}
	release := printer.holding()
	defer release()

	f := newFixture(t, printer, func(o *printing.Options) { o.MaxKioskCopies = nil })

	batch, err := f.uc.PrintSimple(su, printing.SimpleCmd{Photo: f.photos.ids[0], Template: printing.TemplatePolaroid, Copies: 4})
	if err != nil {
		t.Fatalf("PrintSimple: %v", err)
	}

	if len(batch.Jobs) != 1 {
		t.Fatalf("ohne Grenze %d Aufträge, erwartet 1", len(batch.Jobs))
	}
}

// TestCancelQueuedJobIsNeverPrinted: Wer sich vertippt hat, bricht ab,
// bevor das Blatt an der Reihe ist. Der Worker darf es danach nicht doch
// noch drucken, nur weil die Kennung schon in seiner Schlange lag.
func TestCancelQueuedJobIsNeverPrinted(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}
	release := printer.holding()
	defer release()

	f := newFixture(t, printer)

	batch, err := f.uc.Print(su, printing.PrintCmd{Photos: f.photos.ids[:2], Layout: printing.DefaultLayout(), Copies: 1})
	if err != nil {
		t.Fatalf("Print: %v", err)
	}

	first, second := batch.Jobs[0], batch.Jobs[1]
	printer.waitAwaiting(t)

	if err := f.uc.Cancel(su, second); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	canceled := f.job(t, second)
	if canceled.State != printing.StateFailed || canceled.Reason != "canceled-by-user" || canceled.FinishedAt.IsZero() {
		t.Fatalf("abgebrochener Auftrag: Zustand %s, Grund %q", canceled.State, canceled.Reason)
	}

	release()
	f.awaitJob(t, first)

	// Ein dritter Auftrag hinter dem abgebrochenen: Ist er fertig, hat der
	// Worker den abgebrochenen sicher schon aus der Schlange genommen.
	third := f.printOne(t, 2)
	f.awaitJob(t, third)

	if got := f.job(t, second); got.State != printing.StateFailed || got.Reason != "canceled-by-user" || got.PrinterJob != "" {
		t.Fatalf("der Worker hat den Abbruch überschrieben: Zustand %s, Grund %q, PrinterJob %q", got.State, got.Reason, got.PrinterJob)
	}

	if printer.printedCount() != 2 {
		t.Fatalf("es wurden %d Blätter übergeben, erwartet 2 – der abgebrochene wurde gedruckt", printer.printedCount())
	}

	// Ein Abbruch ist abgeschlossen; ihn zu wiederholen ist dagegen
	// ausdrücklich erlaubt.
	if err := f.uc.Retry(su, second); err != nil {
		t.Fatalf("Retry nach Abbruch: %v", err)
	}

	if got := f.awaitJob(t, second); got.State != printing.StateDone {
		t.Fatalf("Wiederholung nach Abbruch: Zustand %s (%s)", got.State, got.Message)
	}

	spec.Verified(t, druck.RDruckKeinNachdruck)
}

// TestCancelWhilePrintingWithdrawsPrinterJob: Liegt das Blatt schon beim
// Drucker, wird es dort zurückgenommen. Der Worker, der noch auf CUPS wartet,
// darf danach nicht "Fertig" darüberschreiben.
func TestCancelWhilePrintingWithdrawsPrinterJob(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}
	release := printer.holding()
	defer release()

	f := newFixture(t, printer)

	id := f.printOne(t, 0)
	printerJob := printer.waitAwaiting(t)

	if err := f.uc.Cancel(su, id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	if got := printer.canceledJobs(); !slices.Equal(got, []string{printerJob}) {
		t.Fatalf("stornierte Aufträge = %v, erwartet [%s]", got, printerJob)
	}

	release()

	// Wie oben: Ein nachfolgender Auftrag zeigt, dass der Worker mit dem
	// abgebrochenen fertig ist.
	f.awaitJob(t, f.printOne(t, 1))

	if got := f.job(t, id); got.State != printing.StateFailed || got.Reason != "canceled-by-user" {
		t.Fatalf("der Worker hat den Abbruch überschrieben: Zustand %s, Grund %q", got.State, got.Reason)
	}

	spec.Verified(t, druck.RDruckKeinNachdruck)
}

// TestCancelIgnoresFinishedAndUnknownJobs: Ein Abbruch in einer veralteten
// Liste darf einen fertigen Auftrag nicht nachträglich zum Fehler machen –
// sonst stimmt die Papierbilanz nicht mehr.
func TestCancelIgnoresFinishedAndUnknownJobs(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}
	f := newFixture(t, printer)

	id := f.printOne(t, 0)
	before := f.awaitJob(t, id)

	if err := f.uc.Cancel(su, id, "gibt-es-nicht"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	if after := f.job(t, id); after.State != printing.StateDone || after.Reason != before.Reason {
		t.Fatalf("ein fertiger Auftrag wurde abgebrochen: %s / %q", after.State, after.Reason)
	}

	if got := printer.canceledJobs(); len(got) != 0 {
		t.Fatalf("für einen fertigen Auftrag wurde storniert: %v", got)
	}
}

// TestObserveSeesEveryFinishedJobOnce: Am Beobachter hängt der Papierzähler.
// Ein doppelt gemeldetes Blatt verbraucht auf dem Zähler zwei, ein
// vergessenes lässt das Papier überraschend ausgehen.
func TestObserveSeesEveryFinishedJobOnce(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}
	f := newFixture(t, printer)

	batch, err := f.uc.Print(su, printing.PrintCmd{Photos: f.photos.ids[:2], Layout: printing.DefaultLayout(), Copies: 1})
	if err != nil {
		t.Fatalf("Print: %v", err)
	}

	for _, id := range batch.Jobs {
		f.awaitJob(t, id)
	}

	printer.setOutcome(printing.Outcome{Done: true, Reason: "canceled-at-device", Message: "Ribbon End"})
	failed := f.printOne(t, 2)
	f.awaitJob(t, failed)

	want := append(slices.Clone(batch.Jobs), failed)

	// Der Beobachter wird direkt nach dem Speichern aufgerufen; ein kurzes
	// Nachwarten fängt sowohl ein verspätetes als auch ein doppeltes Melden.
	deadline := time.Now().Add(5 * time.Second)
	for len(f.observed.seen()) < len(want) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(100 * time.Millisecond)

	seen := f.observed.seen()
	if len(seen) != len(want) {
		t.Fatalf("Observe wurde %d-mal aufgerufen, erwartet %d", len(seen), len(want))
	}

	for i, job := range seen {
		if job.ID != want[i] {
			t.Errorf("Meldung %d: Auftrag %s, erwartet %s", i, job.ID, want[i])
		}

		// Gemeldet wird der endgültige Zustand, nicht ein Zwischenstand –
		// der Beobachter zählt nur Fertiges als verbrauchtes Papier.
		stored := f.job(t, job.ID)
		if !job.State.Done() || job.State != stored.State || job.Reason != stored.Reason || job.PrinterJob != stored.PrinterJob {
			t.Errorf("Meldung %d: %s/%q, gespeichert %s/%q", i, job.State, job.Reason, stored.State, stored.Reason)
		}

		if !slices.Equal(job.Photos, stored.Photos) {
			t.Errorf("Meldung %d: Photos %v, gespeichert %v", i, job.Photos, stored.Photos)
		}
	}

	if seen[0].State != printing.StateDone || seen[2].State != printing.StateFailed {
		t.Errorf("Zustände %s, %s, %s – erwartet fertig, fertig, Fehler", seen[0].State, seen[1].State, seen[2].State)
	}
}

// TestPreviewRendersWithoutPrinting sichert zu, dass die Vorschau das Ergebnis
// zeigt, ohne Papier zu verbrauchen.
func TestPreviewRendersWithoutPrinting(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}
	f := newFixture(t, printer)

	// Das echte Kamerabild statt der einfarbigen Fixture: Auf einer
	// einfarbigen Fläche sähe ein beschnittenes Motiv genauso aus wie ein
	// unbeschnittenes.
	path := filepath.Join(t.TempDir(), "sample.jpg")
	if err := os.WriteFile(path, loadSample(t), 0o600); err != nil {
		t.Fatal(err)
	}

	f.photos.mutex.Lock()
	f.photos.files["sample"] = photo.Location{Photo: photo.Photo{ID: "sample"}, Path: path}
	f.photos.mutex.Unlock()

	seen := map[string]bool{}

	for _, tpl := range printing.Templates() {
		buf, err := f.uc.Preview(su, printing.PreviewCmd{Photos: []photo.ID{"sample"}, Layout: tpl.ID.Layout(), MaxEdge: 300})
		if err != nil {
			t.Fatalf("Preview(%s): %v", tpl.ID, err)
		}

		if len(buf) == 0 || !bytes.HasPrefix(buf, []byte{0xFF, 0xD8}) {
			t.Fatalf("Preview(%s) lieferte kein JPEG (%d Bytes)", tpl.ID, len(buf))
		}

		// Jedes Layout muss ein anderes Bild ergeben, sonst zeigt die
		// Vorschau dreimal dasselbe und die Auswahl wäre eine Behauptung.
		key := string(buf)
		if seen[key] {
			t.Fatalf("Layout %s liefert dieselbe Vorschau wie ein anderes", tpl.ID)
		}

		seen[key] = true
	}

	if printer.printedCount() != 0 {
		t.Fatalf("die Vorschau hat %d Aufträge an den Drucker übergeben, erwartet 0", printer.printedCount())
	}

	spec.Verified(t, druck.RDruckVorschau)
}

// TestPreviewFollowsPaperOrientation: Die Vorschau muss dieselbe Lage zeigen
// wie der Ausdruck. Ein Querformat liegt quer, ein Polaroid und alle
// Aufteilungen bleiben hochkant.
func TestPreviewFollowsPaperOrientation(t *testing.T) {
	printer := &fakePrinter{outcome: succeeded}
	f := newFixture(t, printer)

	landscape := newLibrary(t, 3, 450, 300)
	f.photos.mutex.Lock()
	for id, loc := range landscape.files {
		loc.Photo.ID = "quer-" + id
		f.photos.files[loc.Photo.ID] = loc
	}
	f.photos.mutex.Unlock()

	with := func(format printing.Format, design printing.Design) printing.Layout {
		l := printing.DefaultLayout()
		l.Format, l.Design = format, design

		return l
	}

	cases := []struct {
		name   string
		photos []photo.ID
		layout printing.Layout
		wide   bool
	}{
		{"hochkant einzeln", []photo.ID{"foto-1"}, with(printing.FormatSingle, printing.DesignBorderless), false},
		{"quer einzeln", []photo.ID{"quer-foto-1"}, with(printing.FormatSingle, printing.DesignBorderless), true},
		{"quer Passepartout", []photo.ID{"quer-foto-1"}, with(printing.FormatSingle, printing.DesignMatte), true},
		{"quer Polaroid", []photo.ID{"quer-foto-1"}, with(printing.FormatSingle, printing.DesignPolaroid), false},
		{"quer Collage", []photo.ID{"quer-foto-1", "quer-foto-2", "quer-foto-3"}, with(printing.FormatCollage, printing.DesignBorderless), false},
	}

	for _, c := range cases {
		buf, err := f.uc.Preview(su, printing.PreviewCmd{Photos: c.photos, Layout: c.layout, MaxEdge: 600})
		if err != nil {
			t.Fatalf("%s: Preview: %v", c.name, err)
		}

		img, err := jpeg.Decode(bytes.NewReader(buf))
		if err != nil {
			t.Fatalf("%s: kein lesbares JPEG: %v", c.name, err)
		}

		b := img.Bounds()
		if max(b.Dx(), b.Dy()) != 600 {
			t.Errorf("%s: lange Kante %d, erwartet 600", c.name, max(b.Dx(), b.Dy()))
		}

		// Die Vorschau zeigt die sichtbare Fläche, also 2:3.
		if short := min(b.Dx(), b.Dy()); short != 400 {
			t.Errorf("%s: kurze Kante %d, erwartet 400", c.name, short)
		}

		if wide := b.Dx() > b.Dy(); wide != c.wide {
			t.Errorf("%s: quer = %v, erwartet %v (%dx%d)", c.name, wide, c.wide, b.Dx(), b.Dy())
		}
	}
}

// deniedSubject hat alle Rechte außer einem. So beweist jeder Fall, dass der
// Anwendungsfall genau seine eigene Berechtigung prüft – und nicht bloß
// irgendeine, die ein Gast zufällig auch nicht hat.
type deniedSubject struct{ missing permission.ID }

var errDenied = errors.New("verweigert")

func (s deniedSubject) Audit(p permission.ID) error {
	if p == s.missing {
		return errDenied
	}

	return nil
}

func (s deniedSubject) HasPermission(p permission.ID) bool { return p != s.missing }

// resumablePrinter ist ein Drucker, der sich freigeben lässt, damit auch die
// Verweigerung von [printing.Resume] etwas zu verhindern hat.
type resumablePrinter struct {
	*fakePrinter
	resumed int
}

func (p *resumablePrinter) Resume(context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.resumed++

	return nil
}

// TestUseCasesDenyWithoutPermission: Die Grenze zwischen Gast und Besitzer
// hängt an den Berechtigungen, nicht an den Knöpfen der Oberfläche.
func TestUseCasesDenyWithoutPermission(t *testing.T) {
	fake := &fakePrinter{outcome: succeeded}
	printer := &resumablePrinter{fakePrinter: fake}
	f := newFixture(t, fake, func(o *printing.Options) { o.Printer = printer })

	done := f.printOne(t, 0)
	f.awaitJob(t, done)

	photos := []photo.ID{f.photos.ids[1]}

	calls := map[permission.ID]func(permission.Auditable) error{
		printing.PermPrint: func(s permission.Auditable) error {
			_, err := f.uc.Print(s, printing.PrintCmd{Photos: photos, Layout: printing.DefaultLayout(), Copies: 1})
			return err
		},
		printing.PermPrintSimple: func(s permission.Auditable) error {
			_, err := f.uc.PrintSimple(s, printing.SimpleCmd{Photo: photos[0], Template: printing.TemplateFull, Copies: 1})
			return err
		},
		printing.PermPreview: func(s permission.Auditable) error {
			_, err := f.uc.Preview(s, printing.PreviewCmd{Photos: photos, Layout: printing.DefaultLayout()})
			return err
		},
		printing.PermFindAllJobs: func(s permission.Auditable) error {
			_, err := f.uc.FindAllJobs(s)
			return err
		},
		printing.PermFindJobByID: func(s permission.Auditable) error {
			_, err := f.uc.FindJobByID(s, done)
			return err
		},
		printing.PermRetry: func(s permission.Auditable) error {
			return f.uc.Retry(s, done)
		},
		printing.PermCancel: func(s permission.Auditable) error {
			return f.uc.Cancel(s, done)
		},
		printing.PermDiagnose: func(s permission.Auditable) error {
			_, err := f.uc.Diagnose(s)
			return err
		},
		printing.PermResume: func(s permission.Auditable) error {
			return f.uc.Resume(s)
		},
	}

	for _, perm := range printing.Permissions() {
		call, ok := calls[perm]
		if !ok {
			t.Errorf("für %s gibt es keinen Anwendungsfall im Test", perm)
			continue
		}

		if err := call(deniedSubject{missing: perm}); !errors.Is(err, errDenied) {
			t.Errorf("ohne %s: Fehler %v, erwartet die Verweigerung", perm, err)
		}
	}

	// Nichts davon darf trotz Verweigerung eine Wirkung gehabt haben.
	seq, err := f.uc.FindAllJobs(su)
	if err != nil {
		t.Fatal(err)
	}

	n := 0
	for range seq {
		n++
	}

	if n != 1 {
		t.Errorf("nach verweigerten Aufrufen gibt es %d Aufträge, erwartet nur den einen von vorher", n)
	}

	if job := f.job(t, done); job.State != printing.StateDone || job.Reason == "canceled-by-user" {
		t.Errorf("der fertige Auftrag wurde trotz Verweigerung verändert: %s / %q", job.State, job.Reason)
	}

	if fake.printedCount() != 1 || len(fake.canceledJobs()) != 0 {
		t.Errorf("trotz Verweigerung gedruckt (%d) oder storniert (%v)", fake.printedCount(), fake.canceledJobs())
	}

	fake.mutex.Lock()
	resumed := printer.resumed
	fake.mutex.Unlock()

	if resumed != 0 {
		t.Errorf("der Drucker wurde trotz Verweigerung %d-mal freigegeben", resumed)
	}
}

// TestDiagnoseReportsPrinterState prüft die Auskunft, die der Betreuung das
// Terminal ersparen soll.
func TestDiagnoseReportsPrinterState(t *testing.T) {
	printer := &fakePrinter{}
	f := newFixture(t, printer)

	status, err := f.uc.Diagnose(su)
	if err != nil {
		t.Fatalf("Diagnose: %v", err)
	}

	if !status.OK() || status.Problem() != "" {
		t.Fatalf("ein bereiter Drucker meldet ein Problem: %q", status.Problem())
	}

	// Angehaltener Drucker mit Gerätemeldung.
	printer.setStatus(printing.PrinterStatus{
		Queue: "Fake", Exists: true, Enabled: false, Accepting: true,
		Message: "Out of paper",
	})

	status, err = f.uc.Diagnose(su)
	if err != nil {
		t.Fatalf("Diagnose: %v", err)
	}

	if status.OK() {
		t.Fatal("ein angehaltener Drucker gilt als bereit")
	}

	if status.Problem() == "" {
		t.Fatal("zum angehaltenen Drucker fehlt die Erklärung im Klartext")
	}

	if status.Message != "Out of paper" {
		t.Fatalf("die Meldung des Geräts fehlt: %q", status.Message)
	}

	// Fehlende Warteschlange ist der zweite Fall, der sonst nur im Terminal
	// sichtbar wäre.
	printer.setStatus(printing.PrinterStatus{Queue: "Fake", Exists: false})

	status, err = f.uc.Diagnose(su)
	if err != nil {
		t.Fatalf("Diagnose: %v", err)
	}

	if status.OK() || status.Problem() == "" {
		t.Fatal("eine fehlende Warteschlange wird nicht als Problem gemeldet")
	}

	spec.Verified(t, druck.RDruckDiagnose)
}
