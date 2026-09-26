package heif

import (
	"image"
	"io"
	"sync"
)

var registerOnce sync.Once

// Register meldet HEIF bei image.Decode an, sofern libheif vorhanden ist,
// und meldet, ob das gelang.
//
// Die Anmeldung ist ausdrücklich: Ein Paket, das ein patentbelastetes Format
// schon beim Importieren einschaltet, nähme dem Programm die Entscheidung ab.
func Register() bool {
	if !Available() {
		return false
	}

	registerOnce.Do(func() {
		for _, brand := range []string{"heic", "heix", "heim", "heis", "hevc", "hevx", "mif1", "msf1"} {
			image.RegisterFormat("heif", "????ftyp"+brand, Decode, DecodeConfig)
		}
	})

	return true
}

// Decoder ist die Anbindung an Bildpipelines mit eigener Decoderliste, etwa
// die von gift.
type Decoder struct{}

func (Decoder) DecodeConfig(r io.Reader) (image.Config, error) { return DecodeConfig(r) }
func (Decoder) Decode(r io.Reader) (image.Image, error)        { return Decode(r) }
func (Decoder) Sniff(h []byte) bool                            { return Sniff(h) }

// MemoryFactor ist großzügig: libheif hält beim Dekodieren das YUV-Bild und
// das RGBA-Ergebnis zugleich, und das Paket kopiert Letzteres noch einmal.
func (Decoder) MemoryFactor() float64 { return 12 }
