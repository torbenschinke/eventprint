package lightroom

// UseCases bündelt die Anwendungsfälle der Lightroom-Anbindung.
type UseCases struct {
	BeginConnect  BeginConnect
	AwaitConnect  AwaitConnect
	Disconnect    Disconnect
	Account       Account
	Albums        Albums
	Assets        Assets
	OpenThumbnail OpenThumbnail
	Download      Download
}

// NewUseCases bindet alle Anwendungsfälle an einen gemeinsamen Client.
//
// Gemeinsam, weil sie sich Schlüssel und Katalog teilen: Eine Erneuerung, die
// die Vorschau auslöst, muss auch das Herunterladen sehen.
func NewUseCases(config func() Config, store TokenStore) UseCases {
	client := NewClient(config, store)

	return UseCases{
		BeginConnect:  NewBeginConnect(client),
		AwaitConnect:  NewAwaitConnect(client),
		Disconnect:    NewDisconnect(client),
		Account:       NewAccount(client),
		Albums:        NewAlbums(client),
		Assets:        NewAssets(client),
		OpenThumbnail: NewOpenThumbnail(client),
		Download:      NewDownload(client),
	}
}
