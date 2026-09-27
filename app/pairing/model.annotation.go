package pairing

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/dec"
	"github.com/torbenschinke/eventprint/requirements/fun/upload"
)

var _ = spec.For[Box](
	spec.Satisfies(dec.RDecZustandsablage),
	spec.Waive("K14-REQ-UNVERIFIED", "Die Entscheidung gegen eine Ereignisfolge ist die Abwesenheit einer Sache; ein Test kann sie nicht zeigen."),
)

// Die Kennung des Zugangstokens der Box; mit ihr wird das Token beim Trennen
// gelöscht.
var _ = spec.ForField[Box]("ID",
	spec.Satisfies(upload.RUploadKopplung),
)

// Der Nutzer, der die Box gekoppelt hat. Nur er sieht und trennt sie.
var _ = spec.ForField[Box]("Owner",
	spec.Satisfies(upload.RUploadKopplung),
)

// Die Adresse, mit der gekoppelt wurde, für die Anzeige.
var _ = spec.ForField[Box]("Mail",
	spec.Satisfies(upload.RUploadKopplung),
)

// Der Name der Box, wie sie in "Meine Fotoboxen" erscheint.
var _ = spec.ForField[Box]("Device",
	spec.Satisfies(upload.RUploadKopplung),
)

// Wann gekoppelt wurde.
var _ = spec.ForField[Box]("PairedAt",
	spec.Satisfies(upload.RUploadKopplung),
)
