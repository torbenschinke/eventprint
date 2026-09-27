package uidevice

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"sync"

	"github.com/worldiety/gift/asset/turbojpeg"
	"golang.org/x/image/draw"

	"github.com/torbenschinke/eventprint/pkg/orient"
	"github.com/torbenschinke/eventprint/pkg/xgift"
)

// Hintergrundbilder
//
// Home-Bildschirm und Druck-Studio liegen auf einem Foto: dem neuesten im
// Eingang und dem, das gerade gedruckt wird. Es wird so stark weichgezeichnet,
// dass nur Farbe und Licht bleiben, und darüber schweben die Flächen als Glas.
//
// Das Bild entsteht klein – 192 Pixel an der langen Kante – auf dem Prozessor
// und wird von der Grafik auf den Bildschirm gezogen; stark weichgezeichnet
// sieht man den Unterschied zum großen nicht. Klein ist es in wenigen
// Millisekunden gerechnet und belegt als Textur kaum Speicher. Das Foto
// selbst wird dafür mit libjpeg-turbo schon beim Dekodieren verkleinert.
//
// Dass es ein deckendes Bild ist und keine Farbfläche, ist Absicht: gift
// erkennt ein Glas über einem Bild und zeichnet den Hintergrund dann nur
// einmal weich, statt ihn in jedem Bild neu vom Bildschirm zu kopieren. Auf
// dem Pi ist das der Unterschied zwischen zehn und dreißig Millisekunden für
// den Home-Bildschirm.

// wallEdge ist die lange Kante eines Hintergrundbildes in Pixeln.
const wallEdge = 192

// wallTone ist, wie ein Hintergrundbild getönt wird.
type wallTone int

const (
	// wallLight hellt auf und macht wärmer: dunkle Schrift bleibt lesbar.
	wallLight wallTone = iota
	// wallDark dunkelt ab, für das dunkle Erscheinungsbild.
	wallDark
	// wallStage dunkelt stärker ab und zum Rand hin mehr: Im Druck-Studio
	// soll das Blatt das Hellste sein.
	wallStage
)

type wallKey struct {
	path string
	tone wallTone
}

var walls = struct {
	sync.Mutex
	m     map[wallKey]*xgift.MemorySource
	order []wallKey
}{m: map[wallKey]*xgift.MemorySource{}}

// wallpaperOf liefert das Hintergrundbild zum Foto unter path, ohne Foto einen
// ruhigen Verlauf. Das Ergebnis wird für die letzten Fotos behalten; gerechnet
// wird nur, wenn ein neues dazukommt. Es läuft im Hintergrund, nie beim
// Aufbau.
func wallpaperOf(path string, tone wallTone) *xgift.MemorySource {
	k := wallKey{path, tone}

	walls.Lock()
	if src, ok := walls.m[k]; ok {
		walls.Unlock()
		return src
	}
	walls.Unlock()

	img := decodeSmall(path)
	src := xgift.Memory(encodeWall(tint(blur(img), tone)))

	walls.Lock()
	defer walls.Unlock()

	walls.m[k] = src
	walls.order = append(walls.order, k)
	if len(walls.order) > 6 {
		delete(walls.m, walls.order[0])
		walls.order = walls.order[1:]
	}

	return src
}

// decodeSmall liest das Foto klein und aufgerichtet, oder liefert einen
// Verlauf, wenn es keines gibt oder es sich nicht lesen lässt.
func decodeSmall(path string) *image.RGBA {
	var img image.Image
	if raw, err := os.ReadFile(path); err == nil && path != "" {
		o := orient.FromJPEG(raw)
		if i, err := turbojpeg.DecodeScaled(raw, wallEdge, wallEdge); err == nil {
			img = i
		} else if i, _, err := image.Decode(bytes.NewReader(raw)); err == nil {
			img = i
		}

		if img != nil {
			img = orient.Apply(img, o)
		}
	}

	if img == nil {
		return gradient()
	}

	b := img.Bounds()
	w, h := wallEdge, wallEdge*b.Dy()/max(b.Dx(), 1)
	if b.Dy() > b.Dx() {
		w, h = wallEdge*b.Dx()/max(b.Dy(), 1), wallEdge
	}

	dst := image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)

	return dst
}

// gradient ist der Hintergrund ohne Foto: warm oben, kühl unten.
func gradient() *image.RGBA {
	w, h := wallEdge, wallEdge*9/16
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	top, bottom := color.RGBA{0xF2, 0xB8, 0x8C, 0xFF}, color.RGBA{0x7C, 0x9C, 0xD8, 0xFF}
	for y := range h {
		for x := range w {
			t := (float32(y)/float32(h) + float32(x)/float32(w)*0.35) / 1.35
			img.SetRGBA(x, y, color.RGBA{
				R: lerp8(top.R, bottom.R, t), G: lerp8(top.G, bottom.G, t), B: lerp8(top.B, bottom.B, t), A: 0xFF,
			})
		}
	}

	return img
}

func lerp8(a, b uint8, t float32) uint8 { return uint8(float32(a) + (float32(b)-float32(a))*t) }

// blur zeichnet dreimal mit einem Kastenfilter weich, was einer Gaußglocke
// nahekommt. Der Radius ist ein Zweiunddreißigstel der Kante; auf dem
// Bildschirm sind das bei Full-HD rund sechzig Pixel.
func blur(img *image.RGBA) *image.RGBA {
	r := max(img.Bounds().Dx(), img.Bounds().Dy()) / 32
	tmp := image.NewRGBA(img.Bounds())
	for range 3 {
		boxPass(tmp, img, r, true)
		boxPass(img, tmp, r, false)
	}

	return img
}

// boxPass mittelt über 2r+1 Pixel waagerecht oder senkrecht; am Rand wird
// das letzte Pixel wiederholt.
func boxPass(dst, src *image.RGBA, r int, horizontal bool) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	n, m := w, h
	if !horizontal {
		n, m = h, w
	}

	at := func(line, i int) int {
		i = min(max(i, 0), n-1)
		if horizontal {
			return line*src.Stride + i*4
		}

		return i*src.Stride + line*4
	}

	div := uint32(2*r + 1)
	for line := range m {
		var sum [3]uint32
		for i := -r; i <= r; i++ {
			p := at(line, i)
			sum[0] += uint32(src.Pix[p])
			sum[1] += uint32(src.Pix[p+1])
			sum[2] += uint32(src.Pix[p+2])
		}

		for i := range n {
			q := at(line, i)
			dst.Pix[q], dst.Pix[q+1], dst.Pix[q+2], dst.Pix[q+3] = uint8(sum[0]/div), uint8(sum[1]/div), uint8(sum[2]/div), 0xFF

			out, in := at(line, i-r), at(line, i+r+1)
			for c := range 3 {
				sum[c] += uint32(src.Pix[in+c]) - uint32(src.Pix[out+c])
			}
		}
	}
}

// tint hebt die Sättigung an, wie ein Foto hinter Milchglas leuchtet, und
// hellt oder dunkelt nach dem Erscheinungsbild ab.
func tint(img *image.RGBA, tone wallTone) *image.RGBA {
	b := img.Bounds()
	cx, cy := float32(b.Dx())/2, float32(b.Dy())/2
	for y := range b.Dy() {
		for x := range b.Dx() {
			p := y*img.Stride + x*4
			r, g, bl := float32(img.Pix[p]), float32(img.Pix[p+1]), float32(img.Pix[p+2])
			grey := 0.3*r + 0.59*g + 0.11*bl
			r, g, bl = grey+(r-grey)*1.45, grey+(g-grey)*1.45, grey+(bl-grey)*1.45

			switch tone {
			case wallLight:
				// Ein warmer, heller Schleier wie im Entwurf.
				r, g, bl = r*0.72+255*0.28, g*0.72+246*0.28, bl*0.72+236*0.28
			case wallDark:
				r, g, bl = r*0.5, g*0.5, bl*0.5
			case wallStage:
				dx, dy := (float32(x)-cx)/cx, (float32(y)-cy)/cy
				k := 0.55 - 0.25*min(dx*dx+dy*dy, 1)
				r, g, bl = r*k, g*k, bl*k
			}

			img.Pix[p], img.Pix[p+1], img.Pix[p+2] = clamp8(r), clamp8(g), clamp8(bl)
		}
	}

	return img
}

func clamp8(v float32) uint8 { return uint8(min(max(v, 0), 255)) }

// encodeWall speichert als PNG: Ein JPEG zeigte seine Achterblöcke, zehnfach
// vergrößert, als Stufen im Verlauf.
func encodeWall(img *image.RGBA) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
