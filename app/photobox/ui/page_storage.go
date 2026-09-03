package uiphotobox

import (
	"fmt"
	"io"
	"time"

	"go.wdy.de/nago/presentation/core"
	"go.wdy.de/nago/presentation/ui"
	"go.wdy.de/nago/presentation/ui/alert"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/pkg/diskfree"
)

// StorageOptions bündelt, was die Speicherseite braucht.
type StorageOptions struct {
	Photos photo.UseCases

	// DiskUsage misst den Datenträger, auf dem die Fotobox arbeitet.
	DiskUsage func() (diskfree.Usage, error)

	// DataBytes ist der Platzbedarf der Fotobox insgesamt, also Bildablage,
	// Archiv und Verwaltung.
	DataBytes func() (int64, error)
}

// PageStorage zeigt die Speicherbelegung und räumt die Fotobox frei.
//
// Der Anlass ist der Abbau nach einer Feier: Bilder herunterladen, Karte
// leeren, Box bereit für den nächsten Einsatz.
func PageStorage(wnd core.Window, opts StorageOptions) core.View {
	m := newStorageModel(wnd, opts)

	return ui.VStack(
		alert.BannerMessages(wnd),
		m.purgeDialog(),

		ui.Text("Speicher und Archiv").Font(ui.Title),

		m.statistics(),
		m.actions(),
	).
		Gap(ui.L16).
		Alignment(ui.Leading).
		WithPadding(ui.Padding{}.All(ui.L16)).
		Frame(ui.Frame{}.FullWidth())
}

type storageModel struct {
	wnd  core.Window
	opts StorageOptions

	purgePresented  *core.State[bool]
	purgeUnderstood *core.State[bool]
	busy            *core.State[string]
	result          *core.State[string]
}

func newStorageModel(wnd core.Window, opts StorageOptions) *storageModel {
	return &storageModel{
		wnd:  wnd,
		opts: opts,

		purgePresented:  core.StateOf[bool](wnd, "storage-purge-presented"),
		purgeUnderstood: core.StateOf[bool](wnd, "storage-purge-understood"),
		busy:            core.StateOf[string](wnd, "storage-busy"),
		result:          core.StateOf[string](wnd, "storage-result"),
	}
}

// statistics zeigt, wohin der Platz geht.
func (m *storageModel) statistics() core.View {
	disk, err := m.opts.DiskUsage()
	if err != nil {
		return alert.BannerError(err)
	}

	archive, err := m.opts.Photos.InspectArchive(m.wnd.Subject())
	if err != nil {
		return alert.BannerError(err)
	}

	data, err := m.opts.DataBytes()
	if err != nil {
		return alert.BannerError(err)
	}

	// Die Bildablage getrennt vom Archiv ausweisen.
	//
	// Beides sind Kopien derselben Fotos: Das Archiv ist der Ordner für die
	// Weitergabe, die Bildablage das, woraus Galerie und Druck lesen. Wer nur
	// eine Summe sähe, könnte nicht einschätzen, was das Aufräumen bringt.
	images := data - archive.Bytes
	if images < 0 {
		images = 0
	}

	rest := disk.UsedBytes() - data
	if rest < 0 {
		rest = 0
	}

	return ui.VStack(
		storageRow("Speicherkarte insgesamt", diskfree.GiB(disk.TotalBytes), true),
		storageRow("Fotoarchiv (Originale zur Weitergabe)",
			fmt.Sprintf("%s in %d Dateien", diskfree.GiB(archive.Bytes), archive.Files), false),
		storageRow("Bildablage (Galerie und Druck)", diskfree.GiB(images), false),
		storageRow("System und Übriges", diskfree.GiB(rest), false),
		storageRow("Frei", diskfree.GiB(disk.FreeBytes), true),
	).
		Gap(ui.L8).
		Alignment(ui.Leading).
		BackgroundColor(ui.M2).
		WithPadding(ui.Padding{}.All(ui.L16)).
		Border(ui.Border{}.Radius(ui.L12)).
		Frame(ui.Frame{}.FullWidth())
}

func storageRow(label, value string, strong bool) core.View {
	font := ui.BodyMedium
	if strong {
		font = ui.BodyLarge
	}

	return ui.HStack(
		ui.Text(label).Font(font),
		ui.Spacer(),
		ui.Text(value).Font(font),
	).FullWidth()
}

func (m *storageModel) actions() core.View {
	busy := m.busy.Get()

	var status core.View
	if busy != "" {
		status = ui.Text(busy).Font(ui.BodySmall)
	} else if msg := m.result.Get(); msg != "" {
		status = ui.Text(msg).Font(ui.BodySmall)
	}

	return ui.VStack(
		ui.Text("Nach der Veranstaltung").Font(ui.TitleSmall),
		ui.Text("Zuerst die Bilder herunterladen, dann die Fotobox freiräumen.").
			Font(ui.BodyMedium),

		ui.HStack(
			ui.PrimaryButton(m.download).
				Title("Fotoarchiv herunterladen").
				Enabled(busy == ""),

			ui.SecondaryButton(func() {
				m.purgeUnderstood.Set(false)
				m.purgePresented.Set(true)
			}).
				Title("Fotobox freiräumen").
				Enabled(busy == ""),
		).Gap(ui.L12),

		status,
	).
		Gap(ui.L12).
		Alignment(ui.Leading).
		Frame(ui.Frame{}.FullWidth())
}

// download reicht das Archiv als ZIP an den Browser.
func (m *storageModel) download() {
	usage, err := m.opts.Photos.InspectArchive(m.wnd.Subject())
	if err != nil {
		alert.ShowBannerError(m.wnd, err)
		return
	}

	if usage.Files == 0 {
		m.result.Set("Das Fotoarchiv ist leer.")
		return
	}

	m.wnd.ExportFiles(core.ExportFilesOptions{
		ID:    "fotoarchiv",
		Files: []core.File{archiveZip{wnd: m.wnd, photos: m.opts.Photos}},
	})
}

// archiveZip erzeugt das ZIP beim Abruf, statt es vorher zu bauen.
//
// core.ExportFileBytes hielte die ganze Datei im Speicher. Das Archiv einer
// Feier wiegt mehrere Gigabyte, und die Fotobox hat vier davon insgesamt – der
// Dienst stürbe am Speicher, bevor der Browser das erste Byte sähe.
type archiveZip struct {
	wnd    core.Window
	photos photo.UseCases
}

func (z archiveZip) Name() string { return photo.ArchiveZipName(time.Now()) }

func (z archiveZip) MimeType() (string, bool) { return "application/zip", true }

// Size ist unbekannt, weil das ZIP erst beim Schreiben entsteht. Eine
// geschätzte Zahl wäre schlimmer als keine: Der Browser bräche den Download
// ab, sobald sie nicht stimmt.
func (z archiveZip) Size() (int64, bool) { return 0, false }

func (z archiveZip) Transfer(dst io.Writer) (int64, error) {
	// Der zurückgegebene Zähler ist die Anzahl der Dateien, nicht der Bytes.
	// Nago wertet ihn nicht aus; entscheidend ist der Fehler.
	if _, err := z.photos.ExportArchive(z.wnd.Subject(), dst); err != nil {
		return 0, err
	}

	return 0, nil
}

func (z archiveZip) Open() (io.ReadCloser, error) {
	// Eine Röhre statt eines Puffers: Der Erzeuger schreibt, während der
	// Abnehmer liest, und nichts davon liegt vollständig im Speicher.
	pr, pw := io.Pipe()

	go func() {
		_, err := z.photos.ExportArchive(z.wnd.Subject(), pw)
		_ = pw.CloseWithError(err)
	}()

	return pr, nil
}

// purgeDialog ist die Rückfrage vor dem endgültigen Löschen.
func (m *storageModel) purgeDialog() core.View {
	body := ui.VStack(
		ui.Text("Alle Fotos dieser Veranstaltung werden endgültig entfernt: "+
			"die Bilder in der Galerie, ihre Bilddaten und das Archiv mit den Originalen.").
			Font(ui.BodyMedium),

		ui.Text("Das lässt sich nicht rückgängig machen. Wer die Bilder noch braucht, "+
			"lädt zuerst das Archiv herunter.").
			Font(ui.BodyMedium),

		// Die zweite, gesonderte Bestätigung.
		//
		// Ein Dialog allein genügt hier nicht: Wer zweimal am Abend etwas
		// weggeklickt hat, klickt auch das dritte Mal weg. Das Häkchen
		// verlangt eine andere Bewegung als der Knopf daneben und lässt sich
		// nicht im Vorbeigehen setzen.
		ui.CheckboxField("Ja, die Bilder sind gesichert oder werden nicht mehr gebraucht.",
			m.purgeUnderstood.Get()).
			InputValue(m.purgeUnderstood),
	).Gap(ui.L12).Alignment(ui.Leading).Frame(ui.Frame{}.FullWidth())

	return alert.Dialog(
		"Fotobox freiräumen?",
		body,
		m.purgePresented,
		alert.Width(ui.L480),
		alert.Cancel(nil),
		alert.Custom(func(close func(closeDlg bool)) core.View {
			return ui.PrimaryButton(func() {
				if !m.purgeUnderstood.Get() {
					return
				}

				close(true)
				m.purge()
			}).
				Title("Endgültig löschen").
				Enabled(m.purgeUnderstood.Get())
		}),
	)
}

func (m *storageModel) purge() {
	m.busy.Set("Die Fotobox wird freigeräumt …")
	m.result.Set("")

	subject := m.wnd.Subject()

	go func() {
		res, err := m.opts.Photos.PurgeEvent(subject)

		m.wnd.Post(func() {
			m.busy.Set("")

			if err != nil {
				alert.ShowBannerError(m.wnd, err)
				return
			}

			m.result.Set(fmt.Sprintf(
				"%d Fotos entfernt, %s wieder frei. Die Fotobox ist bereit für die nächste Veranstaltung.",
				res.Photos, diskfree.GiB(res.FreedBytes())))
		})
	}()
}
