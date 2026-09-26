// Package heif dekodiert HEIF/HEIC-Bilder über die Systembibliothek libheif.
//
// iPhones speichern Fotos als HEIC. Gäste, deren Handy die Bilder beim
// Hochladen nicht umwandelt, konnten sie der Box bisher nicht schicken. Die
// Bibliothek wird zur Laufzeit mit purego geladen: Das Programm baut ohne cgo
// und ohne libheif, und fehlt sie auf einem Gerät, fehlt nur das Format.
//
// Das Paket registriert sich nicht von selbst; siehe [Register].
//
// Rechtlicher Hinweis: HEIF/HEIC ist mit Patenten belastet. Das Paket bringt
// keinen Codec mit, sondern benutzt den, der auf dem System installiert ist.
// Ob dessen Nutzung zulässig ist, verantwortet der Betreiber.
package heif

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// ErrUnavailable meldet, dass libheif auf diesem System fehlt.
var ErrUnavailable = errors.New("heif: libheif ist nicht installiert")

// MaxBytes begrenzt die Eingabe. Ein Foto dieser Größe ist kein Foto mehr.
const MaxBytes = 64 << 20

// heifError ist struct heif_error: zwei Aufzählungen und ein C-String.
type heifError struct {
	Code    int32
	Subcode int32
	Message *byte
}

func (e heifError) err(op string) error {
	if e.Code == 0 {
		return nil
	}

	return fmt.Errorf("heif: %s: %s (%d/%d)", op, cString(e.Message), e.Code, e.Subcode)
}

const (
	colorspaceRGB         = 1
	chromaInterleavedRGBA = 11
	channelInterleaved    = 10
)

var lib struct {
	once sync.Once
	err  error

	contextAlloc       func() uintptr
	contextFree        func(ctx uintptr)
	readFromMemory     func(ctx uintptr, mem unsafe.Pointer, size uintptr, opts uintptr) heifError
	primaryImageHandle func(ctx uintptr, out *uintptr) heifError
	handleWidth        func(h uintptr) int32
	handleHeight       func(h uintptr) int32
	handleRelease      func(h uintptr)
	decodeImage        func(h uintptr, out *uintptr, colorspace, chroma int32, opts uintptr) heifError
	imageWidth         func(img uintptr, channel int32) int32
	imageHeight        func(img uintptr, channel int32) int32
	imagePlaneReadonly func(img uintptr, channel int32, stride *int32) unsafe.Pointer
	imageRelease       func(img uintptr)
}

// candidates sind die Namen, unter denen libheif üblicherweise liegt.
func candidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"libheif.1.dylib", "/opt/homebrew/lib/libheif.1.dylib", "/usr/local/lib/libheif.1.dylib"}
	default:
		return []string{"libheif.so.1", "libheif.so"}
	}
}

func load() error {
	lib.once.Do(func() {
		var h uintptr
		for _, name := range candidates() {
			var err error
			if h, err = purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
				break
			}
		}

		if h == 0 {
			lib.err = ErrUnavailable
			return
		}

		defer func() {
			// Eine ältere libheif, der ein Symbol fehlt, lässt RegisterLibFunc
			// in Panik fallen. Das Format fehlt dann, das Programm läuft.
			if r := recover(); r != nil {
				lib.err = fmt.Errorf("heif: libheif unbrauchbar: %v", r)
			}
		}()

		purego.RegisterLibFunc(&lib.contextAlloc, h, "heif_context_alloc")
		purego.RegisterLibFunc(&lib.contextFree, h, "heif_context_free")
		purego.RegisterLibFunc(&lib.readFromMemory, h, "heif_context_read_from_memory_without_copy")
		purego.RegisterLibFunc(&lib.primaryImageHandle, h, "heif_context_get_primary_image_handle")
		purego.RegisterLibFunc(&lib.handleWidth, h, "heif_image_handle_get_width")
		purego.RegisterLibFunc(&lib.handleHeight, h, "heif_image_handle_get_height")
		purego.RegisterLibFunc(&lib.handleRelease, h, "heif_image_handle_release")
		purego.RegisterLibFunc(&lib.decodeImage, h, "heif_decode_image")
		purego.RegisterLibFunc(&lib.imageWidth, h, "heif_image_get_width")
		purego.RegisterLibFunc(&lib.imageHeight, h, "heif_image_get_height")
		purego.RegisterLibFunc(&lib.imagePlaneReadonly, h, "heif_image_get_plane_readonly")
		purego.RegisterLibFunc(&lib.imageRelease, h, "heif_image_release")
	})

	return lib.err
}

// Available meldet, ob HEIF auf diesem System dekodiert werden kann.
func Available() bool { return load() == nil }

// Sniff erkennt HEIF am Dateikopf: eine ftyp-Box mit einer HEIF-Marke.
func Sniff(h []byte) bool {
	if len(h) < 12 || string(h[4:8]) != "ftyp" {
		return false
	}

	switch string(h[8:12]) {
	case "heic", "heix", "heim", "heis", "hevc", "hevx", "hevm", "hevs", "mif1", "msf1":
		return true
	default:
		return false
	}
}

// handle öffnet das Hauptbild. Die Eingabe muss so lange leben wie der
// Kontext, denn libheif liest ohne Kopie.
func handle(data []byte, fn func(h uintptr) error) error {
	if err := load(); err != nil {
		return err
	}

	if len(data) == 0 {
		return errors.New("heif: leere Eingabe")
	}

	ctx := lib.contextAlloc()
	if ctx == 0 {
		return errors.New("heif: kein Kontext")
	}
	defer lib.contextFree(ctx)

	if err := lib.readFromMemory(ctx, unsafe.Pointer(&data[0]), uintptr(len(data)), 0).err("lesen"); err != nil {
		return err
	}

	var h uintptr
	if err := lib.primaryImageHandle(ctx, &h).err("Hauptbild"); err != nil {
		return err
	}
	defer lib.handleRelease(h)

	err := fn(h)
	runtime.KeepAlive(data)

	return err
}

func readAll(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, err
	}

	if len(data) > MaxBytes {
		return nil, errors.New("heif: Bild zu groß")
	}

	return data, nil
}

// DecodeConfig liest die Maße, ohne Pixel zu dekodieren. libheif meldet sie
// nach Anwendung der Drehung, also in der Lage, in der man das Bild sieht.
func DecodeConfig(r io.Reader) (image.Config, error) {
	data, err := readAll(r)
	if err != nil {
		return image.Config{}, err
	}

	var cfg image.Config
	err = handle(data, func(h uintptr) error {
		cfg = image.Config{Width: int(lib.handleWidth(h)), Height: int(lib.handleHeight(h)), ColorModel: nil}
		return nil
	})
	cfg.ColorModel = image.NewNRGBA(image.Rect(0, 0, 1, 1)).ColorModel()

	return cfg, err
}

// Decode dekodiert das Hauptbild. Drehung und Spiegelung aus den HEIF-Boxen
// wendet libheif dabei selbst an.
func Decode(r io.Reader) (image.Image, error) {
	data, err := readAll(r)
	if err != nil {
		return nil, err
	}

	var out *image.NRGBA
	err = handle(data, func(h uintptr) error {
		var img uintptr
		if err := lib.decodeImage(h, &img, colorspaceRGB, chromaInterleavedRGBA, 0).err("dekodieren"); err != nil {
			return err
		}
		defer lib.imageRelease(img)

		w := int(lib.imageWidth(img, channelInterleaved))
		ht := int(lib.imageHeight(img, channelInterleaved))

		var stride int32
		plane := lib.imagePlaneReadonly(img, channelInterleaved, &stride)
		if plane == nil || w <= 0 || ht <= 0 || int(stride) < w*4 {
			return errors.New("heif: keine Bildebene")
		}

		src := unsafe.Slice((*byte)(plane), int(stride)*ht)
		out = image.NewNRGBA(image.Rect(0, 0, w, ht))
		for y := range ht {
			copy(out.Pix[y*out.Stride:y*out.Stride+w*4], src[y*int(stride):y*int(stride)+w*4])
		}

		return nil
	})

	return out, err
}

func cString(p *byte) string {
	if p == nil {
		return ""
	}

	var buf bytes.Buffer
	for ptr := unsafe.Pointer(p); *(*byte)(ptr) != 0; ptr = unsafe.Add(ptr, 1) {
		buf.WriteByte(*(*byte)(ptr))
	}

	return buf.String()
}
