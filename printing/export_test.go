package printing

// LpArgsForTest legt den Aufbau der lp-Argumente für Tests offen.
//
// Die Argumentliste ist der einzige Ort, an dem sich überprüfen lässt, welche
// Treiberoptionen tatsächlich gesetzt werden – ohne dafür Papier zu
// verbrauchen. Nach außen bleibt sie ein Implementierungsdetail.
func (p CUPSPrinter) LpArgsForTest(file, title string) []string {
	return p.lpArgs(file, title)
}
