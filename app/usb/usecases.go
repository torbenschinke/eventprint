package usb

// UseCases bündelt die Anwendungsfälle rund um den USB-Stick.
type UseCases struct {
	Drives Drives
	Export Export
	Eject  Eject
	Images Images
	Read   Read
}

// NewUseCases bindet alle Anwendungsfälle an lsblk und udisksctl.
//
// Für die Fotobox: NewUseCases(ExecRunner{}, StatfsFreeSpace).
func NewUseCases(runner Runner, free FreeSpace) UseCases {
	return UseCases{
		Drives: NewDrives(runner),
		Export: NewExport(runner, free),
		Eject:  NewEject(runner),
		Images: NewImages(runner),
		Read:   NewRead(runner),
	}
}
