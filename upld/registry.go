package upld

import (
	"errors"
	"sync"
	"time"

	"go.wdy.de/nago/application/image"

	"github.com/torbenschinke/eventprint/printing"
)

var (
	ErrExpired = errors.New("upload link expired")
	ErrFull    = errors.New("upload queue is full")
)

const MaxJobsPerSession = 50

type Session struct {
	ID        UploadID
	Token     TokenID
	CreatedAt time.Time
	Jobs      []Job
	Images    map[image.ID]struct{}
}

// Registry keeps all upload identities and queues exclusively in memory.
type Registry struct {
	mu       sync.Mutex
	byUpload map[UploadID]*Session
	byToken  map[TokenID]UploadID
	onRemove func(image.ID)
}

func NewRegistry(onRemove func(image.ID)) *Registry {
	return &Registry{byUpload: map[UploadID]*Session{}, byToken: map[TokenID]UploadID{}, onRemove: onRemove}
}

func (r *Registry) Open(token TokenID) (UploadID, error) {
	id, err := randomID[UploadID]()
	if err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if old, ok := r.byToken[token]; ok {
		r.removeLocked(old)
	}
	r.byToken[token] = id
	r.byUpload[id] = &Session{ID: id, Token: token, CreatedAt: time.Now(), Images: map[image.ID]struct{}{}}
	return id, nil
}

func (r *Registry) Valid(id UploadID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.byUpload[id]
	return ok
}

func (r *Registry) Track(id UploadID, img image.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byUpload[id]
	if !ok {
		if r.onRemove != nil {
			go r.onRemove(img)
		}
		return ErrExpired
	}
	s.Images[img] = struct{}{}
	return nil
}

func (r *Registry) Enqueue(id UploadID, job Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byUpload[id]
	if !ok {
		return ErrExpired
	}
	if len(s.Jobs) >= MaxJobsPerSession {
		return ErrFull
	}
	job.Template = printing.TemplateByID(job.Template).ID
	s.Jobs = append(s.Jobs, job)
	s.Images[job.Image] = struct{}{}
	return nil
}

func (r *Registry) Pending(token TokenID) ([]Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessionByTokenLocked(token)
	if !ok {
		return nil, ErrExpired
	}
	return append([]Job(nil), s.Jobs...), nil
}

func (r *Registry) Find(token TokenID, id JobID) (Job, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessionByTokenLocked(token)
	if !ok {
		return Job{}, false
	}
	for _, job := range s.Jobs {
		if job.ID == id {
			return job, true
		}
	}
	return Job{}, false
}

func (r *Registry) Ack(token TokenID, id JobID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessionByTokenLocked(token)
	if !ok {
		return false
	}
	for i, job := range s.Jobs {
		if job.ID != id {
			continue
		}
		s.Jobs = append(s.Jobs[:i], s.Jobs[i+1:]...)
		delete(s.Images, job.Image)
		if r.onRemove != nil {
			go r.onRemove(job.Image)
		}
		return true
	}
	return false
}

func (r *Registry) PurgeOlderThan(cutoff time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.byUpload {
		if s.CreatedAt.Before(cutoff) {
			r.removeLocked(id)
		}
	}
}

func (r *Registry) sessionByTokenLocked(token TokenID) (*Session, bool) {
	id, ok := r.byToken[token]
	if !ok {
		return nil, false
	}
	s, ok := r.byUpload[id]
	return s, ok
}

func (r *Registry) removeLocked(id UploadID) {
	s, ok := r.byUpload[id]
	if !ok {
		return
	}
	delete(r.byUpload, id)
	delete(r.byToken, s.Token)
	if r.onRemove != nil {
		for img := range s.Images {
			go r.onRemove(img)
		}
	}
}
