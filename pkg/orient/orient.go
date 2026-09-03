// Package orient richtet Bilder anhand ihrer EXIF-Angabe auf.
//
// Kameras und Smartphones speichern das Bild fast immer in der Ausrichtung
// des Sensors und vermerken die tatsächliche Lage lediglich als Zahl im
// EXIF-Block. Browser und Bildbetrachter werten diese Zahl aus, Gos
// image/jpeg dagegen nicht: Es liefert die Rohlage.
//
// Für eine Fotobox ist das folgenschwer. Ein hochkant fotografiertes Motiv
// käme als Querformat an, würde auf die lange Papierkante gelegt, dort
// formatfüllend beschnitten – und der Ausdruck zeigte einen um 90 Grad
// gedrehten, stark vergrößerten Ausschnitt.
package orient

import (
	"bytes"
	"encoding/binary"
	"image"

	// Registrierung der Formate, die an einer Fotobox ankommen.
	_ "image/jpeg"
	_ "image/png"
)

// Orientation ist der Wert des EXIF-Feldes 0x0112.
type Orientation int

const (
	// Normal bedeutet, dass das Bild bereits richtig herum vorliegt. Das ist
	// auch der Rückfall, wenn keine Angabe gefunden wurde.
	Normal Orientation = 1

	FlipHorizontal Orientation = 2
	Rotate180      Orientation = 3
	FlipVertical   Orientation = 4
	Transpose      Orientation = 5
	Rotate90       Orientation = 6
	Transverse     Orientation = 7
	Rotate270      Orientation = 8
)

// SwapsDimensions meldet, ob die Ausrichtung Breite und Höhe vertauscht.
func (o Orientation) SwapsDimensions() bool {
	switch o {
	case Transpose, Rotate90, Transverse, Rotate270:
		return true
	default:
		return false
	}
}

// Apply richtet das Bild gemäß der Ausrichtung auf.
//
// Bei [Normal] wird das Bild unverändert zurückgegeben; der häufigste Fall
// kostet also nichts.
func Apply(img image.Image, o Orientation) image.Image {
	if o == Normal || img == nil {
		return img
	}

	src := img.Bounds()
	w, h := src.Dx(), src.Dy()

	dstW, dstH := w, h
	if o.SwapsDimensions() {
		dstW, dstH = h, w
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	for y := range h {
		for x := range w {
			dx, dy := mapPoint(o, x, y, w, h)
			dst.Set(dx, dy, img.At(src.Min.X+x, src.Min.Y+y))
		}
	}

	return dst
}

// mapPoint bildet einen Quellpunkt auf seine Lage im aufgerichteten Bild ab.
func mapPoint(o Orientation, x, y, w, h int) (int, int) {
	switch o {
	case FlipHorizontal:
		return w - 1 - x, y
	case Rotate180:
		return w - 1 - x, h - 1 - y
	case FlipVertical:
		return x, h - 1 - y
	case Transpose:
		return y, x
	case Rotate90:
		return h - 1 - y, x
	case Transverse:
		return h - 1 - y, w - 1 - x
	case Rotate270:
		return y, w - 1 - x
	default:
		return x, y
	}
}

// Decode liest ein Bild und richtet es anhand seines EXIF-Blocks auf.
func Decode(buf []byte) (image.Image, Orientation, error) {
	img, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		return nil, Normal, err
	}

	o := FromJPEG(buf)

	return Apply(img, o), o, nil
}

// FromJPEG liest die Ausrichtung aus dem EXIF-Block einer JPEG-Datei.
//
// Fehlt der Block, ist er beschädigt oder enthält er das Feld nicht, wird
// [Normal] gemeldet: Ein unlesbares Zusatzfeld darf niemals verhindern, dass
// ein Foto gedruckt wird.
func FromJPEG(buf []byte) Orientation {
	app1, ok := findExifSegment(buf)
	if !ok {
		return Normal
	}

	return orientationFromTIFF(app1)
}

// findExifSegment sucht den Nutzdatenteil des APP1-Segments mit der Kennung
// "Exif\0\0".
func findExifSegment(buf []byte) ([]byte, bool) {
	// Startmarker FF D8
	if len(buf) < 4 || buf[0] != 0xFF || buf[1] != 0xD8 {
		return nil, false
	}

	pos := 2
	for pos+4 <= len(buf) {
		if buf[pos] != 0xFF {
			return nil, false
		}

		marker := buf[pos+1]

		// Start of Scan: ab hier folgen die Bilddaten, kein EXIF mehr.
		if marker == 0xDA {
			return nil, false
		}

		size := int(binary.BigEndian.Uint16(buf[pos+2 : pos+4]))
		if size < 2 || pos+2+size > len(buf) {
			return nil, false
		}

		payload := buf[pos+4 : pos+2+size]

		if marker == 0xE1 && len(payload) > 6 && string(payload[:6]) == "Exif\x00\x00" {
			return payload[6:], true
		}

		pos += 2 + size
	}

	return nil, false
}

// orientationFromTIFF liest Feld 0x0112 aus dem ersten Verzeichnis des
// eingebetteten TIFF-Blocks.
func orientationFromTIFF(tiff []byte) Orientation {
	if len(tiff) < 8 {
		return Normal
	}

	var order binary.ByteOrder

	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return Normal
	}

	// Magische Zahl 42 bestätigt die Byte-Reihenfolge.
	if order.Uint16(tiff[2:4]) != 42 {
		return Normal
	}

	offset := int(order.Uint32(tiff[4:8]))
	if offset < 8 || offset+2 > len(tiff) {
		return Normal
	}

	count := int(order.Uint16(tiff[offset : offset+2]))
	entry := offset + 2

	const entrySize = 12

	for range count {
		if entry+entrySize > len(tiff) {
			return Normal
		}

		tag := order.Uint16(tiff[entry : entry+2])
		if tag == 0x0112 {
			// Der Wert ist ein SHORT und steht direkt im Eintrag.
			value := Orientation(order.Uint16(tiff[entry+8 : entry+10]))
			if value < Normal || value > Rotate270 {
				return Normal
			}

			return value
		}

		entry += entrySize
	}

	return Normal
}
