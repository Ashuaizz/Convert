package repository

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrNotFound = errors.New("resource not found")
var ErrNotImplemented = errors.New("repository method not implemented")

type Job struct {
	ID           string
	UserID       string
	Type         string
	Status       string
	Progress     int
	Options      map[string]any
	Message      string
	ErrorCode    string
	ErrorMessage string
	RetryCount   int
	MaxRetries   int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
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
	ClaimJob(ctx context.Context, id string, now time.Time) (Job, bool, error)
	SucceedJob(ctx context.Context, id string, now time.Time) (Job, error)
	FailJob(ctx context.Context, id string, code string, message string, now time.Time) (Job, error)
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

func (r *MemoryRepository) ClaimJob(ctx context.Context, id string, now time.Time) (Job, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[id]
	if !ok {
		return Job{}, false, ErrNotFound
	}
	if job.Status != "queued" && job.Status != "retrying" {
		return job, false, nil
	}
	job.Status = "running"
	job.UpdatedAt = now
	job.StartedAt = &now
	r.jobs[id] = job
	return job, true, nil
}

func (r *MemoryRepository) SucceedJob(ctx context.Context, id string, now time.Time) (Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[id]
	if !ok {
		return Job{}, ErrNotFound
	}
	job.Status = "succeeded"
	job.Progress = 100
	job.UpdatedAt = now
	job.FinishedAt = &now
	r.jobs[id] = job
	return job, nil
}

func (r *MemoryRepository) FailJob(ctx context.Context, id string, code string, message string, now time.Time) (Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[id]
	if !ok {
		return Job{}, ErrNotFound
	}
	job.Status = "failed"
	job.ErrorCode = code
	job.ErrorMessage = message
	job.RetryCount++
	job.UpdatedAt = now
	job.FinishedAt = &now
	r.jobs[id] = job
	return job, nil
}
