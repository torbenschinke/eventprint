package photo

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/dec"
	"github.com/torbenschinke/eventprint/requirements/fun/archiv"
	"github.com/torbenschinke/eventprint/requirements/fun/druck"
	"github.com/torbenschinke/eventprint/requirements/fun/foto"
	"github.com/torbenschinke/eventprint/requirements/fun/modus"
)

var _ = spec.For[Photo](
	spec.Satisfies(dec.RDecZustandsablage),
	// Die Festlegung betrifft die Form der Ablage. Ein nicht vorhandenes
	// Ereignisprotokoll lässt sich nicht vorführen.
	spec.Waive("K14-REQ-UNVERIFIED", "Die Entscheidung gegen eine Ereignisfolge ist die Abwesenheit einer Sache; ein Test kann sie nicht zeigen."),
)

// Die Kennung, unter der ein Foto wiedergefunden wird. Sie beginnt mit dem
// Zeitpunkt der Aufnahme und ist dadurch zugleich die Sortierung der
// Mediathek.
var _ = spec.ForField[Photo]("ID",
	spec.Satisfies(foto.RFotoEinzelbild),
)

// Der Dateiname, unter dem das Bild geliefert wurde. Er macht ein Bild auf
// dem USB-Stick wiedererkennbar.
var _ = spec.ForField[Photo]("Name",
	spec.Satisfies(archiv.RArchivExport),
)

// Die unveränderte Datei in der Ablage, aus der gedruckt wird.
var _ = spec.ForField[Photo]("File",
	spec.Satisfies(foto.RFotoDruckvorlage),
)

// Woher das Bild stammt: Kamera, Handy, NAS oder USB-Stick.
var _ = spec.ForField[Photo]("Source",
	spec.Satisfies(foto.RFotoImport),
)

// Die Feier, zu der das Foto gehört; leer bei privaten Fotos. Daran hängt,
// was Gäste im Kiosk sehen.
var _ = spec.ForField[Photo]("Event",
	spec.Satisfies(modus.RModusPrivat),
)

// Das Foto liegt noch im Eingang.
var _ = spec.ForField[Photo]("Unseen",
	spec.Satisfies(foto.RFotoEingang),
)

// Vom Besitzer als Favorit markiert.
var _ = spec.ForField[Photo]("Favorite",
	spec.Satisfies(foto.RFotoHistorie),
)

// Wie viele Blätter mit diesem Foto gedruckt wurden.
var _ = spec.ForField[Photo]("Prints",
	spec.Satisfies(foto.RFotoHistorie),
)

// Breite und Höhe in der Lage, in der das Motiv betrachtet wird. Sie
// entscheiden, ob ein Einzelbild quer oder hochkant gedruckt wird.
var _ = spec.ForField[Photo]("Width",
	spec.Satisfies(druck.RDruckGestaltung),
)

var _ = spec.ForField[Photo]("Height",
	spec.Satisfies(druck.RDruckGestaltung),
)

// Zeitpunkt der Aufnahme in die Ablage.
var _ = spec.ForField[Photo]("CreatedAt",
	spec.Satisfies(foto.RFotoHistorie),
)
