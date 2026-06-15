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

type File struct {
	ID          string
	UserID      string
	Filename    string
	ContentType string
	Size        int64
	StorageURI  string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Repository interface {
	CreateFile(ctx context.Context, file File) error
	GetFile(ctx context.Context, id string) (File, error)
	UpdateFileStatus(ctx context.Context, id string, status string, updatedAt time.Time) (File, error)
	CreateJob(ctx context.Context, job Job) error
	AttachJobFile(ctx context.Context, jobID string, fileID string, role string) error
	GetJob(ctx context.Context, id string) (Job, error)
}

type MemoryRepository struct {
	mu       sync.RWMutex
	files    map[string]File
	jobs     map[string]Job
	jobFiles map[string]map[string]string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		files:    make(map[string]File),
		jobs:     make(map[string]Job),
		jobFiles: make(map[string]map[string]string),
	}
}

func (r *MemoryRepository) CreateFile(ctx context.Context, file File) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.files[file.ID] = file
	return nil
}

func (r *MemoryRepository) GetFile(ctx context.Context, id string) (File, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	file, ok := r.files[id]
	if !ok {
		return File{}, ErrNotFound
	}
	return file, nil
}

func (r *MemoryRepository) UpdateFileStatus(ctx context.Context, id string, status string, updatedAt time.Time) (File, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	file, ok := r.files[id]
	if !ok {
		return File{}, ErrNotFound
	}
	file.Status = status
	file.UpdatedAt = updatedAt
	r.files[id] = file
	return file, nil
}

func (r *MemoryRepository) CreateJob(ctx context.Context, job Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[job.ID] = job
	return nil
}

func (r *MemoryRepository) AttachJobFile(ctx context.Context, jobID string, fileID string, role string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.jobs[jobID]; !ok {
		return ErrNotFound
	}
	if _, ok := r.files[fileID]; !ok {
		return ErrNotFound
	}
	if _, ok := r.jobFiles[jobID]; !ok {
		r.jobFiles[jobID] = make(map[string]string)
	}
	r.jobFiles[jobID][fileID] = role
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
