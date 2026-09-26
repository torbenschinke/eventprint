package printing

import (
	"bytes"
	"container/list"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"os"
	"sync"
	"time"

	"go.wdy.de/nago/application/permission"
	xdraw "golang.org/x/image/draw"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/pkg/orient"
)

// ScaledJPEGDecoder dekodiert ein JPEG verkleinert, mindestens aber auf
// minW × minH. libjpeg-turbo kann das direkt in der DCT und spart dabei den
// größten Teil der Arbeit; siehe gift/asset/turbojpeg.
type ScaledJPEGDecoder func(raw []byte, minW, minH int) (image.Image, error)

// PreviewSourceEdge ist die lange Kante, auf die ein Original für die
// Vorschau verkleinert wird.
//
// Die Vorschau entsteht mit dem Renderer des Druckers auf dem Druckraster –
// die sichtbare Fläche misst dort 1200 × 1800 Punkte – und wird danach auf
// höchstens gut 1300 Pixel verkleinert. Ein Original mit 6000 Pixeln an der
// langen Kante trägt dazu nichts bei, was 1600 nicht auch tragen: Es kostet
// nur das Dekodieren von 24 Megapixeln, das Drehen und die Gesichtserkennung
// darauf, bei jedem Tipp auf ein anderes Format. Die volle Auflösung bleibt
// dem Druck.
const PreviewSourceEdge = 1600

// previewCacheSize ist die Zahl verkleinerter Originale, die die Vorschau
// behält: genug für ein Blatt mit drei Motiven und das Blättern zwischen
// zwei Blättern, klein genug für einen Pi (je rund 7 MB).
const previewCacheSize = 8

// decodeOriginal liest ein Original und richtet es auf. Mit maxEdge > 0 wird
// es auf diese lange Kante verkleinert – und zwar vor dem Aufrichten, damit
// nicht erst 24 Megapixel gedreht werden.
func decodeOriginal(raw []byte, maxEdge int, scaled ScaledJPEGDecoder) (image.Image, error) {
	o := orient.FromJPEG(raw)

	// Mit libjpeg-turbo auch dann, wenn voll dekodiert wird: Der Druck
	// braucht die ganze Auflösung, aber nicht die Geduld von image/jpeg.
	// Scheitert der schnelle Weg – keine Bibliothek, CMYK-JPEG –, bleibt der
	// langsame.
	var img image.Image
	if scaled != nil && isJPEG(raw) {
		if cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw)); err == nil {
			w, h := cfg.Width, cfg.Height
			if maxEdge > 0 {
				w, h = fitEdge(w, h, maxEdge)
			}

			if i, err := scaled(raw, w, h); err == nil {
				img = i
			}
		}
	}

	if img == nil {
		i, _, err := image.Decode(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}

		img = i
	}

	if maxEdge > 0 {
		img = shrink(img, maxEdge)
	}

	return orient.Apply(img, o), nil
}

// fitEdge verkleinert w × h so, dass die lange Kante edge misst. Kleinere
// Bilder bleiben, wie sie sind.
func fitEdge(w, h, edge int) (int, int) {
	long := max(w, h)
	if long <= edge || long == 0 {
		return w, h
	}

	return max(1, (w*edge+long-1)/long), max(1, (h*edge+long-1)/long)
}

// shrink verkleinert img auf die lange Kante edge, wenn es spürbar größer
// ist. Ein Rest von wenigen Prozent lohnt das Umrechnen nicht.
func shrink(img image.Image, edge int) image.Image {
	b := img.Bounds()
	if max(b.Dx(), b.Dy()) <= edge*105/100 {
		return img
	}

	w, h := fitEdge(b.Dx(), b.Dy(), edge)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)

	return dst
}

func isJPEG(raw []byte) bool {
	return len(raw) > 3 && raw[0] == 0xFF && raw[1] == 0xD8 && raw[2] == 0xFF
}

// previewSource ist ein verkleinertes Original samt Datum und den darauf
// erkannten Gesichtern.
type previewSource struct {
	img   image.Image
	date  time.Time
	faces []image.Rectangle
	found bool
}

// previewSources hält die verkleinerten Originale der zuletzt gezeigten
// Vorschauen.
//
// Im Druck-Studio rendert jede Wahl – Format, Rahmen, Filter, Schrift – die
// Vorschau neu, immer mit denselben Fotos. Ohne diesen Speicher hieß jeder
// Tipp: Original lesen, dekodieren, drehen, Gesichter suchen. Mit ihm ist es
// nur noch das Zusammensetzen des Blattes. Die Originale ändern sich nie, der
// Schlüssel ist deshalb allein die Kennung.
type previewSources struct {
	mu      sync.Mutex
	entries map[photo.ID]*list.Element
	order   *list.List // vorne das zuletzt benutzte
	size    int
}

type previewEntry struct {
	id  photo.ID
	src *previewSource
}

func newPreviewSources(size int) *previewSources {
	return &previewSources{entries: map[photo.ID]*list.Element{}, order: list.New(), size: size}
}

func (c *previewSources) get(id photo.ID) (*previewSource, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[id]
	if !ok {
		return nil, false
	}

	c.order.MoveToFront(e)

	return e.Value.(*previewEntry).src, true
}

func (c *previewSources) put(id photo.ID, src *previewSource) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[id]; ok {
		e.Value.(*previewEntry).src = src
		c.order.MoveToFront(e)
		return
	}

	c.entries[id] = c.order.PushFront(&previewEntry{id: id, src: src})
	for c.order.Len() > c.size {
		last := c.order.Back()
		c.order.Remove(last)
		delete(c.entries, last.Value.(*previewEntry).id)
	}
}

// faces merkt sich die Gesichter eines verkleinerten Originals. Der Renderer
// fragt nach dem Bild, nicht nach der Kennung; die Bilder im Speicher sind
// aber dieselben Werte über alle Vorschauen hinweg.
func (c *previewSources) detector(detect FaceDetector) FaceDetector {
	if detect == nil {
		return nil
	}

	return func(img image.Image) []image.Rectangle {
		c.mu.Lock()
		var hit *previewSource
		for e := c.order.Front(); e != nil; e = e.Next() {
			if s := e.Value.(*previewEntry).src; s.img == img {
				hit = s
				break
			}
		}

		if hit != nil && hit.found {
			faces := hit.faces
			c.mu.Unlock()
			return faces
		}
		c.mu.Unlock()

		faces := detect(img)
		if hit != nil {
			c.mu.Lock()
			hit.faces, hit.found = faces, true
			c.mu.Unlock()
		}

		return faces
	}
}

// previewLoader lädt die Motive einer Vorschau verkleinert und aus dem
// Speicher, wenn sie schon einmal geladen wurden.
func previewLoader(locate photo.Locate, scaled ScaledJPEGDecoder, cache *previewSources) loadMotifs {
	return func(ids []photo.ID, layout Layout) (Sheet, error) {
		sheet := Sheet{Layout: layout}

		var missing []photo.ID
		for _, id := range ids {
			if _, ok := cache.get(id); !ok {
				missing = append(missing, id)
			}
		}

		if len(missing) > 0 {
			locs, err := locate(permission.SU(), missing...)
			if err != nil {
				return Sheet{}, err
			}

			for _, loc := range locs {
				raw, err := os.ReadFile(loc.Path)
				if err != nil {
					return Sheet{}, fmt.Errorf("cannot read original: %w", err)
				}

				img, err := decodeOriginal(raw, PreviewSourceEdge, scaled)
				if err != nil {
					return Sheet{}, fmt.Errorf("cannot decode original: %w", err)
				}

				cache.put(loc.Photo.ID, &previewSource{img: img, date: loc.Photo.CreatedAt})
			}
		}

		for _, id := range ids {
			src, ok := cache.get(id)
			if !ok {
				continue
			}

			sheet.Images = append(sheet.Images, src.img)
			sheet.Dates = append(sheet.Dates, src.date)
		}

		if len(sheet.Images) == 0 {
			return Sheet{}, errPhotoGone
		}

		return sheet, nil
	}
}

// previewRenderOptions merkt sich die Gesichter der Vorschau-Quellen, damit
// die Erkennung je Foto nur einmal läuft und nicht bei jedem Tipp.
func previewRenderOptions(load func() RenderOptions, cache *previewSources) func() RenderOptions {
	return func() RenderOptions {
		o := load()
		o.DetectFaces = cache.detector(o.DetectFaces)
		return o
	}
}
