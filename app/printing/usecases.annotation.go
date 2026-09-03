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

var _ = spec.For[FindAllJobs](
	spec.Satisfies(druck.RDruckStatus),
	spec.Help(`Liefert alle Druckaufträge, beginnend mit dem neuesten.`),
)

var _ = spec.For[FindJobByID](
	spec.Satisfies(druck.RDruckStatus),
	spec.Help(`Liefert einen einzelnen Druckauftrag anhand seiner Kennung.`),
)

var _ = spec.For[Retry](
	spec.Satisfies(druck.RDruckWiederholung),
	spec.Help(`Stellt einen gescheiterten Auftrag erneut in die Warteschlange.
Ein noch anhängiger Auftrag desselben Bildes wird dabei zurückgenommen.`),
)

var _ = spec.For[Preview](
	spec.Satisfies(druck.RDruckVorschau),
	spec.Help(`Rendert ein Foto mit dem gewählten Layout, ohne zu drucken.`),
)

var _ = spec.For[Diagnose](
	spec.Satisfies(druck.RDruckDiagnose),
	spec.Help(`Beschreibt den Zustand des Druckers für die Betreuung.`),
)
