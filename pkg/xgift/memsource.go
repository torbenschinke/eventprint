package xgift

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/worldiety/gift/asset"
)

// MemorySource is a picture held in memory, for example a rendered preview or
// a generated QR code.
//
// Its identity is derived from the bytes, so two sources over the same bytes
// share the pipeline's cache entries and a changed picture is a new entry
// rather than a stale thumbnail.
type MemorySource struct {
	id   asset.ID
	rev  string
	mime string
	data []byte
}

// Memory returns a source over data. The slice must not be modified
// afterwards.
func Memory(data []byte) *MemorySource {
	sum := sha256.Sum256(data)
	rev := hex.EncodeToString(sum[:12])

	return &MemorySource{
		id:   asset.ID("mem://" + rev),
		rev:  rev,
		mime: http.DetectContentType(data),
		data: data,
	}
}

// Metadata implements asset.Source.
func (s *MemorySource) Metadata() asset.Metadata {
	return asset.Metadata{ID: s.id, Revision: s.rev, MIMEType: s.mime}
}

// Open implements asset.Source.
func (s *MemorySource) Open(context.Context) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data)), nil
}

// Probe implements asset.Prober: the revision and size are known without any
// I/O, which lets the pipeline cache the decoded picture.
func (s *MemorySource) Probe(context.Context) (asset.ProbeResult, error) {
	return asset.ProbeResult{Revision: s.rev, Size: int64(len(s.data)), MIMEType: s.mime}, nil
}

// FuncSource is a picture whose bytes come from a function, for example an
// authenticated HTTP request that asset.HTTP cannot express.
type FuncSource struct {
	id   asset.ID
	rev  string
	open func(ctx context.Context) (io.ReadCloser, error)
}

// Func returns a source with the given stable id and revision. An empty
// revision disables disk caching for it.
func Func(id, revision string, open func(ctx context.Context) (io.ReadCloser, error)) *FuncSource {
	return &FuncSource{id: asset.ID(id), rev: revision, open: open}
}

// Metadata implements asset.Source.
func (s *FuncSource) Metadata() asset.Metadata {
	return asset.Metadata{ID: s.id, Revision: s.rev}
}

// Open implements asset.Source.
func (s *FuncSource) Open(ctx context.Context) (io.ReadCloser, error) { return s.open(ctx) }

// Probe implements asset.Prober with the revision given at construction.
func (s *FuncSource) Probe(context.Context) (asset.ProbeResult, error) {
	return asset.ProbeResult{Revision: s.rev}, nil
}
