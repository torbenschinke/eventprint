package nas

// UseCases bündelt die Anwendungsfälle rund um das NAS.
type UseCases struct {
	Shares    Shares
	Browse    Browse
	Thumbnail Thumbnail
	Read      Read
}

// NewUseCases bindet alle Anwendungsfälle an einen Client und die
// eingerichteten Zugangsdaten.
//
// Für die Fotobox: NewUseCases(NewSMB(), func() Config { … Einstellungen … }).
func NewUseCases(client Client, config func() Config) UseCases {
	return UseCases{
		Shares:    NewShares(client, config),
		Browse:    NewBrowse(client, config),
		Thumbnail: NewThumbnail(client, config),
		Read:      NewRead(client, config),
	}
}
