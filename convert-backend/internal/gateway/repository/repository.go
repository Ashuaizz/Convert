package repository

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrNotFound = errors.New("resource not found")

type Job struct {
	ID        string
	Type      string
	Status    string
	Progress  int
	Options   map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Repository interface {
	CreateJob(ctx context.Context, job Job) error
	GetJob(ctx context.Context, id string) (Job, error)
}

type MemoryRepository struct {
	mu   sync.RWMutex
	jobs map[string]Job
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{jobs: make(map[string]Job)}
}

func (r *MemoryRepository) CreateJob(ctx context.Context, job Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[job.ID] = job
	return nil
}

func (r *MemoryRepository) GetJob(ctx context.Context, id string) (Job, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	job, ok := r.jobs[id]
	if !ok {
		return Job{}, ErrNotFound
	}
	return job, nil
}
