package lightroom

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/quellen"
)

var _ = spec.For[BeginConnect](
	spec.Satisfies(quellen.RQuellenLightroomAnmeldung),
	spec.Help(`Beginnt die Anmeldung auf dem Handy und liefert den kurzen Link für den QR-Code.`),
)

var _ = spec.For[AwaitConnect](
	spec.Satisfies(quellen.RQuellenLightroomAnmeldung),
	spec.Help(`Fragt einmal beim Upload-Dienst nach, ob die Anmeldung abgeschlossen ist, und holt dann die Tokens.`),
)

var _ = spec.For[Disconnect](
	spec.Satisfies(quellen.RQuellenLightroomAnmeldung),
	spec.Help(`Trennt die Verbindung zu Lightroom.`),
)

var _ = spec.For[Account](
	spec.Satisfies(quellen.RQuellenLightroomAnmeldung),
	spec.Help(`Sagt, ob und als wer das Gerät mit Lightroom verbunden ist.`),
)

var _ = spec.For[Albums](
	spec.Satisfies(quellen.RQuellenLightroom),
	spec.Help(`Liefert die Alben des Kontos.`),
)

var _ = spec.For[Assets](
	spec.Satisfies(quellen.RQuellenLightroom),
	spec.Help(`Liefert eine Seite Fotos eines Albums oder aller Fotos.`),
)

var _ = spec.For[OpenThumbnail](
	spec.Satisfies(quellen.RQuellenLightroom),
	spec.Help(`Öffnet das Vorschaubild eines Fotos.`),
)

var _ = spec.For[Download](
	spec.Satisfies(quellen.RQuellenLightroom),
	spec.Help(`Lädt die in Lightroom bearbeitete Fassung eines Fotos für den Druck.`),
)
