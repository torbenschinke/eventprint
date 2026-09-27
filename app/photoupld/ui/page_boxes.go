package uiphotoupld

import (
	"fmt"

	"go.wdy.de/nago/presentation/core"
	nagoui "go.wdy.de/nago/presentation/ui"
	"go.wdy.de/nago/presentation/ui/alert"
	"go.wdy.de/nago/presentation/ui/list"

	"github.com/torbenschinke/eventprint/app/pairing"
)

// PageBoxes zeigt die Fotoboxen, die der angemeldete Nutzer gekoppelt hat,
// und lässt ihn jede davon trennen.
//
// Vorher konnte nur ein Administrator eine Box vom Dienst lösen, und nur,
// wenn er in der Token-Verwaltung erriet, welches Token zu welcher Box
// gehört. Wer seine Box verkauft oder verleiht, soll das selbst erledigen.
func PageBoxes(wnd core.Window, uc pairing.UseCases) core.View {
	confirm := core.AutoState[bool](wnd)
	selected := core.AutoState[pairing.Box](wnd)
	rev := core.AutoState[int](wnd)
	rev.Get()

	boxes, err := uc.FindMyBoxes(wnd.Subject())
	if err != nil {
		return alert.BannerError(err)
	}

	entries := make([]core.View, 0, len(boxes))
	for _, box := range boxes {
		entries = append(entries, list.Entry().
			Headline(box.Device).
			SupportingText(fmt.Sprintf("Gekoppelt am %s", box.PairedAt.Local().Format("02.01.2006 um 15:04"))).
			Trailing(nagoui.SecondaryButton(func() {
				selected.Set(box)
				confirm.Set(true)
			}).Title("Trennen")))
	}

	var content core.View
	if len(entries) == 0 {
		content = nagoui.Text("Noch keine Fotobox gekoppelt. Trage an der Box unter Einstellungen → Handy-Upload deine Mailadresse ein und tippe den Code aus der Mail ein.").Font(nagoui.BodyLarge)
	} else {
		content = list.List(entries...).Caption(nagoui.Text("Diese Boxen holen Fotos ab, die Gäste über ihren QR-Code senden.")).FullWidth()
	}

	box := selected.Get()

	return nagoui.VStack(
		alert.BannerMessages(wnd),
		nagoui.Text("Meine Fotoboxen").Font(nagoui.DisplaySmall),
		content,
		alert.Dialog("Fotobox trennen?",
			nagoui.Text(fmt.Sprintf("„%s“ holt danach keine Fotos mehr ab. Um sie wieder zu verbinden, koppelst du sie an der Box neu.", box.Device)),
			confirm,
			alert.Cancel(nil),
			alert.Delete(func() {
				if err := uc.UnpairBox(wnd.Subject(), box.ID); err != nil {
					alert.ShowBannerError(wnd, err)
					return
				}

				alert.ShowBannerMessage(wnd, alert.Message{Title: "Getrennt", Message: fmt.Sprintf("„%s“ ist nicht mehr mit deinem Konto verbunden.", box.Device), Intent: alert.IntentOk})
				rev.Set(rev.Get() + 1)
			}),
		),
	).Gap(nagoui.L24).Alignment(nagoui.Leading).WithPadding(nagoui.Padding{}.All(nagoui.L24)).Frame(nagoui.Frame{}.FullWidth())
}
