package printing

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/dec"
	"github.com/torbenschinke/eventprint/requirements/fun/druck"
)

var _ = spec.For[Job](
	spec.Satisfies(dec.RDecZustandsablage),
	spec.Waive("K14-REQ-UNVERIFIED", "Die Entscheidung gegen eine Ereignisfolge ist die Abwesenheit einer Sache; ein Test kann sie nicht zeigen."),
)

// Die Kennung des Auftrags. Sie beginnt mit dem Zeitpunkt der
// Erteilung und ordnet dadurch die Liste der Aufträge.
var _ = spec.ForField[Job]("ID",
	spec.Satisfies(druck.RDruckStatus),
)

// Das Foto, das gedruckt werden soll.
var _ = spec.ForField[Job]("Photo",
	spec.Satisfies(druck.RDruckAuftrag),
)

// Das gewählte Layout: formatfüllend, Passepartout oder Polaroid.
var _ = spec.ForField[Job]("Template",
	spec.Satisfies(druck.RDruckAuftrag),
)

// Name der Warteschlange, an die der Auftrag ging.
var _ = spec.ForField[Job]("Printer",
	spec.Satisfies(druck.RDruckStatus),
)

// Wartet, druckt, fertig oder Fehler.
var _ = spec.ForField[Job]("State",
	spec.Satisfies(druck.RDruckStatus),
)

// Die Ursache im Klartext, wie sie die Betreuung liest.
var _ = spec.ForField[Job]("Message",
	spec.Satisfies(druck.RDruckStatus),
)

// Die Kennung des Auftrags im Druckdienst, etwa "CZ01-31". Ohne
// sie ließe sich ein aufgegebener Auftrag dort nicht zurücknehmen, und der
// Drucker gäbe ihn später von sich aus aus.
var _ = spec.ForField[Job]("PrinterJob",
	spec.Satisfies(druck.RDruckKeinNachdruck),
)

// Der unübersetzte Grund des Abschlusses aus dem Druckprotokoll.
// Für die Fehlersuche belastbarer als die Meldung im Klartext.
var _ = spec.ForField[Job]("Reason",
	spec.Satisfies(druck.RDruckStatus),
)

// Wer den Druck ausgelöst hat.
var _ = spec.ForField[Job]("RequestedBy",
	spec.Satisfies(druck.RDruckStatus),
)

// Zeitpunkt der Erteilung.
var _ = spec.ForField[Job]("CreatedAt",
	spec.Satisfies(druck.RDruckStatus),
)

// Zeitpunkt des Abschlusses. Zusammen mit CreatedAt ergibt sich
// die Dauer, die die Druckstatus-Seite anzeigt.
var _ = spec.ForField[Job]("FinishedAt",
	spec.Satisfies(druck.RDruckStatus),
)
