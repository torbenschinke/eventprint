package usb

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/archiv"
	"github.com/torbenschinke/eventprint/requirements/fun/quellen"
)

var _ = spec.For[Drives](
	spec.Satisfies(archiv.RArchivExport, quellen.RQuellenUsb),
	spec.Help(`Listet eingesteckte USB-Sticks, niemals die Systemkarte.`),
)

var _ = spec.For[Export](
	spec.Satisfies(archiv.RArchivExport),
	spec.Help(`Kopiert Originale in einen Ordner auf dem Stick; bereits kopierte Dateien werden übersprungen.`),
)

var _ = spec.For[Eject](
	spec.Satisfies(archiv.RArchivExport),
	spec.Help(`Hängt den Stick aus und schaltet ihn ab, damit er gezogen werden kann.`),
)

var _ = spec.For[Images](
	spec.Satisfies(quellen.RQuellenUsb),
	spec.Help(`Listet die Bilder auf einem Stick.`),
)

var _ = spec.For[Read](
	spec.Satisfies(quellen.RQuellenUsb),
	spec.Help(`Liest ein Bild vom Stick und nur vom Stick.`),
)
