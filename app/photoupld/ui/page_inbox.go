package uiphotoupld

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/presentation/core"
	nagoui "go.wdy.de/nago/presentation/ui"
	"go.wdy.de/nago/presentation/ui/alert"

	"github.com/torbenschinke/eventprint/app/upld"
)

// modeInbox ist der Wert von m, mit dem die Box ihre Upload-Seite in den
// Eingangsmodus schaltet.
const modeInbox = "inbox"

// pageInbox nimmt Bilder für den Eingang der Fotobox entgegen.
//
// Zuhause steht die Box nicht als Partydrucker im Raum, sondern als privater
// Fotodrucker. Wer ihr Bilder schickt, will sie meist erst am großen
// Bildschirm anschauen, sortieren und dann entscheiden, was wie gedruckt wird.
// Deshalb gibt es hier weder Layout noch Vorschau, dafür mehrere Bilder auf
// einmal – niemand möchte zwanzig Urlaubsfotos einzeln auswählen.
func pageInbox(wnd core.Window, opts Options, id upld.UploadID) core.View {
	sent := core.StateOf[int](wnd, "photoupld-inbox-sent")
	if sent.Get() > 0 {
		return inboxSent(wnd, sent.Get(), func() { sent.Set(0) })
	}

	return nagoui.VStack(
		alert.BannerMessages(wnd),
		nagoui.Text("Fotos an die Box senden").Font(nagoui.DisplaySmall),
		nagoui.Text("Wähle ein oder mehrere Bilder. Sie landen im Eingang der Box – Format und Design wählst du dort am Bildschirm.").Font(nagoui.BodyLarge).TextAlignment(nagoui.TextAlignCenter),
		nagoui.PrimaryButton(func() { importInbox(wnd, opts, id, sent) }).Title("Fotos auswählen"),
	).Gap(nagoui.L24).Alignment(nagoui.Center).WithPadding(nagoui.Padding{}.All(nagoui.L24)).Frame(nagoui.Frame{}.FullWidth())
}

// importInbox legt jedes gewählte Bild als eigenen Auftrag ohne Layout ab.
//
// Ein misslungenes Bild hält die übrigen nicht auf: Bei zwanzig Fotos soll
// nicht ein einziges kaputtes alle anderen mitreißen. Ein unlesbares Bild
// meldet ein Banner mit Dateinamen, damit klar ist, welches fehlt.
func importInbox(wnd core.Window, opts Options, uploadID upld.UploadID, sent *core.State[int]) {
	wnd.ImportFiles(core.ImportFilesOptions{ID: "photoupld-inbox-files", Multiple: true, AllowedMimeTypes: []string{"image/jpeg", "image/png"}, OnCompletion: func(files []core.File) {
		if len(files) == 0 {
			return
		}

		// Erst nachsehen, wie viel Platz ist: Ein Bild aufzubereiten ist
		// teuer, und eines, das danach an der vollen Warteschlange abprallt,
		// hätte man sich sparen können.
		free, err := opts.Registry.Remaining(uploadID)
		if err != nil {
			showInboxExpired(wnd)
			return
		}

		count, full := 0, 0
		for i, file := range files {
			if i >= free {
				full++
				continue
			}

			err := sendToInbox(opts, uploadID, file)
			switch {
			case err == nil:
				count++
			case errors.Is(err, upld.ErrFull):
				// Ein anderer Gast war schneller.
				full++
			case errors.Is(err, upld.ErrExpired):
				// Ohne Sitzung scheitern auch alle weiteren. Ein Banner je
				// Bild sagte zwanzigmal dasselbe.
				showInboxExpired(wnd)
				return
			default:
				showInboxProblem(wnd, file.Name(), err)
			}
		}

		// Die volle Warteschlange trifft alle übrigen Bilder gleich. Ein
		// Banner für alle genügt.
		if full > 0 {
			showInboxFull(wnd, full)
		}

		if count > 0 {
			sent.Set(count)
		}
	}})
}

func sendToInbox(opts Options, uploadID upld.UploadID, file core.File) error {
	jobID, err := upld.NewJobID()
	if err != nil {
		return err
	}

	set, err := opts.CreateSrcSet(user.SU(), image.Options{ID: image.ID(upld.ImagePrefix + string(jobID))}, file)
	if err != nil {
		return err
	}

	// Track vor Enqueue: Läuft die Sitzung genau dazwischen ab, räumt die
	// Registry das Bild selbst weg, statt es verwaist liegen zu lassen.
	if err := opts.Registry.Track(uploadID, set.ID); err != nil {
		return err
	}

	return opts.Registry.Enqueue(uploadID, upld.Job{ID: jobID, Image: set.ID, Template: upld.InboxTemplate, Filename: file.Name(), CreatedAt: time.Now()})
}

func showInboxExpired(wnd core.Window) {
	alert.ShowBannerMessage(wnd, alert.Message{
		Title:   "Dieser Link ist abgelaufen",
		Message: "Bitte scanne den QR-Code an der Fotobox noch einmal.",
		Intent:  alert.IntentError,
	})
}

func showInboxFull(wnd core.Window, n int) {
	title := fmt.Sprintf("%d Fotos nicht gesendet", n)
	if n == 1 {
		title = "1 Foto nicht gesendet"
	}

	alert.ShowBannerMessage(wnd, alert.Message{
		Title:   title,
		Message: fmt.Sprintf("Der Eingang der Box nimmt höchstens %d wartende Bilder auf. Sobald die Box die bisherigen abgeholt hat, ist wieder Platz.", upld.MaxJobsPerSession),
		Intent:  alert.IntentError,
	})
}

func showInboxProblem(wnd core.Window, filename string, err error) {
	// Die Einzelheiten gehören ins Protokoll, nicht auf das Handy.
	slog.Error("photoupld: Bild für den Eingang nicht angenommen", "file", filename, "err", err)

	alert.ShowBannerMessage(wnd, alert.Message{
		Title:   fmt.Sprintf("„%s“ nicht gesendet", filename),
		Message: "Das Bild konnte nicht verarbeitet werden. Ist es wirklich ein JPEG- oder PNG-Bild?",
		Intent:  alert.IntentError,
	})
}

// inboxSent bestätigt die gesendeten Bilder und bietet gleich weitere an.
//
// Die Banner bleiben eingebunden: Kam nur ein Teil an, erfährt man hier,
// welche Bilder fehlen.
func inboxSent(wnd core.Window, count int, again func()) core.View {
	title := fmt.Sprintf("%d Fotos gesendet", count)
	text := "Die Bilder werden an die Box übertragen und liegen dort im Eingang."
	if count == 1 {
		title = "1 Foto gesendet"
		text = "Das Bild wird an die Box übertragen und liegt dort im Eingang."
	}

	return nagoui.VStack(
		alert.BannerMessages(wnd),
		nagoui.Text(title).Font(nagoui.DisplaySmall),
		nagoui.Text(text).
			Font(nagoui.BodyLarge).TextAlignment(nagoui.TextAlignCenter),
		nagoui.PrimaryButton(again).Title("Weitere Fotos senden"),
	).
		Gap(nagoui.L16).
		Alignment(nagoui.Center).
		WithPadding(nagoui.Padding{}.All(nagoui.L32)).
		Frame(nagoui.Frame{}.MatchScreen())
}
