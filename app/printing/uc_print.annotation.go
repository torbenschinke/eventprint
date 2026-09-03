package printing

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/druck"
)

var _ = spec.For[Print](
	spec.Satisfies(druck.RDruckAuftrag, druck.RDruckKeinNachdruck),
	spec.Help(`Stellt ein Foto mit dem gewählten Layout in die Warteschlange.
Der Aufruf kehrt sofort zurück; gedruckt wird im Hintergrund. Gibt die Fotobox
den Auftrag später auf, nimmt sie ihn auch beim Druckdienst zurück.`),
)
