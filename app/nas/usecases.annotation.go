package nas

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/quellen"
)

var _ = spec.For[Shares](
	spec.Satisfies(quellen.RQuellenNas),
	spec.Help(`Meldet sich probeweise an und listet die Freigaben zur Auswahl.`),
)

var _ = spec.For[Browse](
	spec.Satisfies(quellen.RQuellenNas),
	spec.Help(`Listet Unterordner und Bilder eines Ordners der eingerichteten Freigabe.`),
)

var _ = spec.For[Thumbnail](
	spec.Satisfies(quellen.RQuellenNas),
	spec.Help(`Liefert die Vorschau, die das NAS schon angelegt hat, sonst das Original.`),
)

var _ = spec.For[Read](
	spec.Satisfies(quellen.RQuellenNas),
	spec.Help(`Lädt ein Bild der eingerichteten Freigabe und nur von dort.`),
)
