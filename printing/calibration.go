package printing

// Diese Datei enthält die Kalibrierung des Citizen CZ-01, wie sie am Gerät
// ausgemessen wurde. Sie gleicht zwei Abweichungen der Linux-Druckkette
// gegenüber dem Herstellertreiber aus, die beide zu sichtbar schlechteren
// Ausdrucken führten.
//
// Ermittelt wurde sie, indem dieselbe Vorlage einmal durch die Linux-Kette
// (imagetoraster + rastertogutenprint) und einmal durch den Citizen-Treiber
// unter macOS geschickt und die erzeugten Datenströme byteweise verglichen
// wurden. Die Vorgehensweise ist in der README beschrieben.

// ToneCurve bildet einen Eingangswert auf den Wert ab, den der
// Herstellertreiber an derselben Stelle sendet.
//
// Gutenprint reicht die Werte für diesen Drucker **linear** durch: Eine
// Quellhelligkeit von 17 wird als 17 gesendet. Der Herstellertreiber legt
// dagegen eine Kurve auf, die die Schatten anhebt und die Lichter absenkt.
// Ohne diese Kurve trägt der Drucker in dunklen Partien zu viel Farbe auf,
// und die überschüssige Farbe verläuft in Transportrichtung — im Ausdruck
// sichtbar als ausblutende Kanten.
//
// Die Stützstellen wurden mit einer Testtafel aus 64 Vollton-Bändern
// gemessen, liegen also vier Tonwerte auseinander; dazwischen wird linear
// interpoliert.
type ToneCurve [64]uint8

// Gemessen am 14.08.2026, Citizen CZ-01, Firmware 01.10, Medium 4x6 (PC) SD.
//
// Die drei Kanäle unterscheiden sich nur geringfügig, werden aber getrennt
// geführt, weil der Drucker für Gelb, Magenta und Cyan eigene Farbtabellen
// hält.
var (
	// CurveYellow gilt für den Blaukanal, aus dem die Gelb-Ebene entsteht.
	CurveYellow = ToneCurve{
		0, 5, 13, 19, 23, 26, 30, 34,
		38, 43, 47, 52, 55, 58, 61, 65,
		69, 73, 78, 82, 85, 90, 94, 98,
		102, 107, 111, 116, 120, 124, 129, 133,
		138, 141, 145, 149, 152, 156, 160, 163,
		167, 171, 174, 178, 181, 184, 188, 191,
		194, 198, 201, 204, 208, 212, 216, 220,
		223, 227, 230, 234, 238, 242, 249, 255,
	}

	// CurveMagenta gilt für den Grünkanal.
	CurveMagenta = ToneCurve{
		0, 4, 11, 16, 20, 24, 30, 35,
		39, 43, 48, 52, 56, 60, 63, 67,
		70, 74, 78, 82, 85, 89, 93, 97,
		101, 105, 109, 114, 118, 122, 127, 130,
		135, 139, 143, 147, 151, 155, 158, 161,
		165, 169, 173, 177, 181, 184, 187, 190,
		193, 196, 199, 202, 205, 209, 213, 216,
		220, 223, 227, 231, 235, 241, 248, 255,
	}

	// CurveCyan gilt für den Rotkanal.
	CurveCyan = ToneCurve{
		0, 5, 12, 19, 22, 26, 31, 35,
		40, 44, 49, 53, 55, 58, 61, 65,
		68, 73, 77, 81, 84, 88, 91, 95,
		99, 103, 109, 114, 118, 122, 126, 129,
		135, 139, 143, 147, 150, 154, 158, 161,
		165, 169, 172, 176, 180, 183, 186, 189,
		192, 196, 199, 202, 206, 210, 213, 215,
		218, 222, 226, 230, 236, 241, 248, 255,
	}
)

// lookup interpoliert die Kurve für einen einzelnen Wert.
func (c ToneCurve) lookup(v uint8) uint8 {
	// 64 Stützstellen über 256 Werte: Position im Stützstellenraster.
	pos := float64(v) * 63 / 255
	i := int(pos)

	if i >= 63 {
		return c[63]
	}

	f := pos - float64(i)
	lo := float64(c[i])
	hi := float64(c[i+1])

	return uint8(lo + (hi-lo)*f + 0.5)
}

// Table entfaltet die Kurve zu einer vollständigen Nachschlagetabelle.
//
// Das lohnt sich, weil ein Ausdruck aus über zwei Millionen Bildpunkten
// besteht und die Interpolation sonst je Punkt und Kanal anfiele.
func (c ToneCurve) Table() [256]uint8 {
	var t [256]uint8
	for v := range 256 {
		t[v] = c.lookup(uint8(v))
	}

	return t
}

// calibration bündelt die Tabellen aller drei Kanäle.
type calibration struct {
	red   [256]uint8 // wird zur Cyan-Ebene
	green [256]uint8 // wird zur Magenta-Ebene
	blue  [256]uint8 // wird zur Gelb-Ebene
}

// printerCalibration wird einmalig beim Start aufgebaut.
var printerCalibration = calibration{
	red:   CurveCyan.Table(),
	green: CurveMagenta.Table(),
	blue:  CurveYellow.Table(),
}
