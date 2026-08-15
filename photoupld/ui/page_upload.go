package ui

import (
	"errors"
	"fmt"
	"time"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/presentation/core"
	nagoui "go.wdy.de/nago/presentation/ui"
	"go.wdy.de/nago/presentation/ui/alert"

	"github.com/torbenschinke/eventprint/printing"
	"github.com/torbenschinke/eventprint/ui/preview"
	"github.com/torbenschinke/eventprint/upld"
)

type Options struct {
	Registry     *upld.Registry
	CreateSrcSet image.CreateSrcSet
}

func PageUpload(wnd core.Window, opts Options) core.View {
	id := upld.UploadID(wnd.Values()["u"])
	if id == "" || !opts.Registry.Valid(id) {
		return expired()
	}

	img := core.StateOf[image.ID](wnd, "photoupld-image")
	filename := core.StateOf[string](wnd, "photoupld-filename")
	tpl := core.StateOf[printing.TemplateID](wnd, "photoupld-template").Init(func() printing.TemplateID { return printing.TemplateFull })
	submitted := core.StateOf[bool](wnd, "photoupld-submitted")
	if submitted.Get() {
		return message("Vielen Dank!", "Dein Bild wird an die Fotobox übertragen und dort gedruckt.")
	}

	var chooser core.View
	if img.Get() != "" {
		chooser = nagoui.VStack(
			nagoui.Text("Drucklayout wählen").Font(nagoui.TitleMedium),
			preview.Selector(img.Get(), tpl),
			nagoui.PrimaryButton(func() {
				jobID, err := upld.NewJobID()
				if err == nil {
					err = opts.Registry.Enqueue(id, upld.Job{ID: jobID, Image: img.Get(), Template: tpl.Get(), Filename: filename.Get(), CreatedAt: time.Now()})
				}
				if err != nil {
					alert.ShowBannerError(wnd, err)
					return
				}
				submitted.Set(true)
			}).Title("Jetzt drucken"),
		).Gap(nagoui.L16).Alignment(nagoui.Center)
	}

	return nagoui.VStack(
		alert.BannerMessages(wnd),
		nagoui.Text("Dein Foto drucken").Font(nagoui.DisplaySmall),
		nagoui.Text("Wähle ein Bild aus und entscheide, wie es auf dem Ausdruck aussehen soll.").Font(nagoui.BodyLarge).TextAlignment(nagoui.TextAlignCenter),
		nagoui.PrimaryButton(func() { importImage(wnd, opts, id, img, filename) }).Title("Foto auswählen"),
		chooser,
	).Gap(nagoui.L24).Alignment(nagoui.Center).WithPadding(nagoui.Padding{}.All(nagoui.L24)).Frame(nagoui.Frame{}.FullWidth())
}

func importImage(wnd core.Window, opts Options, uploadID upld.UploadID, target *core.State[image.ID], filename *core.State[string]) {
	wnd.ImportFiles(core.ImportFilesOptions{ID: "photoupld-file", AllowedMimeTypes: []string{"image/jpeg", "image/png"}, OnCompletion: func(files []core.File) {
		if len(files) == 0 {
			return
		}
		id, err := upld.NewJobID()
		if err != nil {
			alert.ShowBannerError(wnd, err)
			return
		}
		set, err := opts.CreateSrcSet(user.SU(), image.Options{ID: image.ID(upld.ImagePrefix + string(id))}, files[0])
		if err == nil {
			err = opts.Registry.Track(uploadID, set.ID)
		}
		if err != nil {
			if errors.Is(err, upld.ErrExpired) {
				alert.ShowBannerError(wnd, fmt.Errorf("der Upload-Link ist abgelaufen"))
			} else {
				alert.ShowBannerError(wnd, err)
			}
			return
		}
		target.Set(set.ID)
		filename.Set(files[0].Name())
	}})
}

func expired() core.View {
	return message("Dieser Link ist abgelaufen", "Bitte scanne den QR-Code an der Fotobox noch einmal.")
}

func message(title, text string) core.View {
	return nagoui.VStack(nagoui.Text(title).Font(nagoui.DisplaySmall), nagoui.Text(text).Font(nagoui.BodyLarge).TextAlignment(nagoui.TextAlignCenter)).
		Gap(nagoui.L16).Alignment(nagoui.Center).WithPadding(nagoui.Padding{}.All(nagoui.L32)).Frame(nagoui.Frame{}.MatchScreen())
}
