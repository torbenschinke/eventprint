package photo

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/archiv"
	"github.com/torbenschinke/eventprint/requirements/fun/foto"
	"github.com/torbenschinke/eventprint/requirements/fun/modus"
)

var _ = spec.For[Import](
	spec.Satisfies(foto.RFotoImport),
	spec.Help(`Übernimmt ein Bild aus beliebiger Quelle und sichert es Byte für Byte.`),
)

var _ = spec.For[FindAll](
	spec.Satisfies(foto.RFotoHistorie),
	spec.Help(`Liefert die Mediathek oder einen Ausschnitt davon, die neuesten Fotos zuerst.`),
)

var _ = spec.For[FindEvent](
	spec.Satisfies(modus.RModusPrivat),
	spec.Help(`Liefert ausschließlich die Fotos einer Feier – die Sicht der Gäste im Kiosk.`),
)

var _ = spec.For[FindByID](
	spec.Satisfies(foto.RFotoEinzelbild, modus.RModusPrivat),
	spec.Help(`Liefert ein einzelnes Foto; private Fotos nur für den, der die Mediathek sehen darf.`),
)

var _ = spec.For[Delete](
	spec.Satisfies(foto.RFotoLoeschen),
	spec.Help(`Entfernt Fotos samt Original endgültig.`),
)

var _ = spec.For[SetFavorite](
	spec.Satisfies(foto.RFotoHistorie),
	spec.Help(`Markiert Fotos als Favorit oder nimmt die Markierung zurück.`),
)

var _ = spec.For[MarkSeen](
	spec.Satisfies(foto.RFotoEingang),
	spec.Help(`Nimmt Fotos aus dem Eingang, sobald sie für den Druck ausgewählt wurden.`),
)

var _ = spec.For[MarkPrinted](
	spec.Satisfies(foto.RFotoHistorie),
	spec.Help(`Vermerkt ein gedrucktes Blatt an seinen Fotos.`),
)

var _ = spec.For[OpenOriginal](
	spec.Satisfies(foto.RFotoDruckvorlage),
	spec.Help(`Öffnet die unveränderte Datei eines Fotos.`),
)

var _ = spec.For[Locate](
	spec.Satisfies(foto.RFotoDruckvorlage, modus.RModusPrivat, archiv.RArchivExport),
	spec.Help(`Liefert die Ablageorte der Originale für Vorschau, Druck und Weitergabe; private Fotos nur für den, der die Mediathek sehen darf.`),
)

var _ = spec.For[InspectStorage](
	spec.Satisfies(archiv.RArchivPlatz),
	spec.Help(`Beschreibt, wie viel Platz die Fotos belegen und wie viel frei ist.`),
)

var _ = spec.For[PurgeEvent](
	spec.Satisfies(archiv.RArchivLoeschen),
	spec.Help(`Löscht alle Fotos einer Feier; private Fotos bleiben unberührt.`),
)
