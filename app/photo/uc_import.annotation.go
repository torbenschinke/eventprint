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
