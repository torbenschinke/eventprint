package printing

// DriverOptionsForTest legt die an Gutenprint übergebenen Optionen offen.
func (p CUPSPrinter) DriverOptionsForTest() []string { return p.driverOptions() }
