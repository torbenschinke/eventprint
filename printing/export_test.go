package printing

// DriverOptionsForTest legt die an Gutenprint übergebenen Optionen offen.
func (p CUPSPrinter) DriverOptionsForTest() []string { return p.driverOptions() }

// SetCancelExecutableForTest ersetzt das Storno-Kommando und liefert die
// Funktion zum Zurücksetzen. Damit lässt sich prüfen, dass ein aufgegebener
// Auftrag tatsächlich zurückgenommen wird, ohne CUPS zu benötigen.
func SetCancelExecutableForTest(name string) func() {
	old := cancelExecutable
	cancelExecutable = name

	return func() { cancelExecutable = old }
}

// SetLpstatExecutableForTest ersetzt die Abfrage von CUPS und liefert die
// Funktion zum Zurücksetzen.
func SetLpstatExecutableForTest(name string) func() {
	old := lpstatExecutable
	lpstatExecutable = name

	return func() { lpstatExecutable = old }
}
