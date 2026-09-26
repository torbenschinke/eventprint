package device

import (
	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/requirements/fun/druck"
	"github.com/torbenschinke/eventprint/requirements/fun/foto"
	"github.com/torbenschinke/eventprint/requirements/fun/modus"
)

var _ = spec.For[LoadSettings](
	spec.Satisfies(modus.RModusEinstellungen),
	spec.Help(`Liefert die Einstellungen des Geräts.`),
)

var _ = spec.For[SaveSettings](
	spec.Satisfies(modus.RModusEinstellungen),
	spec.Help(`Ändert die Einstellungen des Geräts, ohne gleichzeitige Änderungen anderer zu überschreiben.`),
)

var _ = spec.For[SetPin](
	spec.Satisfies(modus.RModusBetreuung),
	spec.Help(`Legt die Betreuer-PIN fest.`),
)

var _ = spec.For[StartKiosk](
	spec.Satisfies(modus.RModusKiosk),
	spec.Help(`Versetzt das Gerät bis zum nächsten Einschalten in den Kiosk und legt eine neue Feier an.`),
)

var _ = spec.For[StopKiosk](
	spec.Satisfies(modus.RModusBetreuung),
	spec.Help(`Beendet den Kiosk ohne Neustart; nur für die freigeschaltete Betreuung.`),
)

var _ = spec.For[CurrentKiosk](
	spec.Satisfies(modus.RModusHeim),
	spec.Help(`Liefert die laufende Feier; ohne Feier ist das Gerät im Heimbetrieb.`),
)

var _ = spec.For[Unlock](
	spec.Satisfies(modus.RModusBetreuung),
	spec.Help(`Schaltet mit der PIN die Betreuung befristet frei und bremst das Raten aus.`),
)

var _ = spec.For[Preflight](
	spec.Satisfies(modus.RModusKiosk),
	spec.Help(`Prüft vor einer Feier Drucker, Papier, Upload-Dienst, Kamera und PIN.`),
)

var _ = spec.For[RefillPaper](
	spec.Satisfies(druck.RDruckPapier),
	spec.Help(`Meldet ein neu eingelegtes Papierset.`),
)

var _ = spec.For[ConsumePaper](
	spec.Satisfies(druck.RDruckPapier),
	spec.Help(`Zieht gedruckte Blätter vom Papiervorrat ab.`),
)

var _ = spec.For[Intake](
	spec.Satisfies(foto.RFotoEingang, druck.RDruckKiosk),
	spec.Help(`Nimmt ein Bild von Handy oder Kamera an: im Heimbetrieb in den Eingang, im Kiosk zur Feier und je nach Einstellung sofort in den Druck.`),
)
