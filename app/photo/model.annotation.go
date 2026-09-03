package photo

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/dec"
	"github.com/torbenschinke/eventprint/requirements/fun/druck"
	"github.com/torbenschinke/eventprint/requirements/fun/foto"
)

var _ = spec.For[Photo](
	spec.Satisfies(dec.RDecZustandsablage),
	// Die Festlegung betrifft die Form der Ablage. Ein Test könnte zeigen,
	// dass ein Foto einen Neustart übersteht, aber nicht, dass sein Verlauf
	// bewusst nicht aufbewahrt wird – ein nicht vorhandenes Ereignisprotokoll
	// lässt sich nicht vorführen.
	spec.Waive("K14-REQ-UNVERIFIED", "Die Entscheidung gegen eine Ereignisfolge ist die Abwesenheit einer Sache; ein Test kann sie nicht zeigen."),
)

// Die Kennung, unter der ein Foto wiedergefunden wird. Sie beginnt
// mit dem Aufnahmezeitpunkt in Millisekunden und ist dadurch zugleich die
// Sortierreihenfolge der Historie.
var _ = spec.ForField[Photo]("ID",
	spec.Satisfies(foto.RFotoEinzelbild),
)

// Verweist auf die Ablage im Bild-Subsystem. Unter demselben
// Schlüssel liegen die Originaldaten, die der Druck verwendet.
var _ = spec.ForField[Photo]("Image",
	spec.Satisfies(foto.RFotoDruckvorlage),
)

// Der Dateiname, unter dem das Bild geliefert wurde. Er macht eine
// Aufnahme außerhalb der Fotobox wiedererkennbar.
var _ = spec.ForField[Photo]("Name",
	spec.Satisfies(foto.RFotoImport),
)

// Woher das Bild stammt: Kamera, Gast-Upload oder Internet.
var _ = spec.ForField[Photo]("Source",
	spec.Satisfies(foto.RFotoImport),
)

// Breite des aufgerichteten Bildes. Sie entscheidet, ob Vorschau
// und Ausdruck das Blatt quer oder hochkant verwenden.
var _ = spec.ForField[Photo]("Width",
	spec.Satisfies(druck.RDruckVorschau),
)

// Höhe des aufgerichteten Bildes, siehe Width.
var _ = spec.ForField[Photo]("Height",
	spec.Satisfies(druck.RDruckVorschau),
)

// Aufnahmezeitpunkt. Die Historie zeigt ihn an und ordnet danach.
var _ = spec.ForField[Photo]("CreatedAt",
	spec.Satisfies(foto.RFotoHistorie),
)
