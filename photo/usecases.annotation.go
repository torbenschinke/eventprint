package photo

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/foto"
)

var _ = spec.For[Import](
	spec.Satisfies(foto.RFotoImport),
	spec.Help(`Nimmt ein eingehendes Bild auf, gleich ob es von der Kamera, aus
dem Netz der Fotobox oder über das Internet kommt. Das gelieferte Original
wird dabei unverändert gesichert.`),
)

var _ = spec.For[FindByID](
	spec.Satisfies(foto.RFotoEinzelbild),
	spec.Help(`Liefert ein einzelnes Bild anhand seiner Kennung.`),
)

var _ = spec.For[FindAll](
	spec.Satisfies(foto.RFotoHistorie),
	spec.Help(`Liefert alle Bilder, beginnend mit dem neuesten.`),
)

var _ = spec.For[FindLatest](
	spec.Satisfies(foto.RFotoHistorie),
	spec.Help(`Liefert die jüngsten Bilder für den Startbildschirm.`),
)

var _ = spec.For[Delete](
	spec.Satisfies(foto.RFotoLoeschen),
	spec.Help(`Entfernt ein Bild aus der Historie.`),
)

var _ = spec.For[OpenOriginal](
	spec.Satisfies(foto.RFotoDruckvorlage),
	spec.Help(`Öffnet die unveränderten Originaldaten als Vorlage für den Druck.`),
)
