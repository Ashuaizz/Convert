package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"convert-backend/internal/gateway/repository"
	"convert-backend/internal/gateway/rpcclient"
	"convert-backend/internal/pkg/idgen"
	"convert-backend/internal/pkg/queue"
	"convert-backend/internal/pkg/storage"
)

type CreateJobRequest struct {
	Type         string         `json:"type"`
	InputFileIDs []string       `json:"input_file_ids"`
	Options      map[string]any `json:"options"`
}

type JobResponse struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Status    string         `json:"status"`
	Progress  int            `json:"progress"`
	Options   map[string]any `json:"options,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type JobService struct {
	repo       repository.Repository
	storage    storage.Client
	queue      queue.Publisher
	processors *rpcclient.Registry
}

func NewJobService(repo repository.Repository, storage storage.Client, queue queue.Publisher, processors *rpcclient.Registry) *JobService {
	return &JobService{
		repo:       repo,
		storage:    storage,
		queue:      queue,
		processors: processors,
	}
}

func (s *JobService) Create(ctx context.Context, req CreateJobRequest) (JobResponse, error) {
	if strings.TrimSpace(req.Type) == "" {
		return JobResponse{}, errors.New("job type is required")
	}
	if len(req.InputFileIDs) == 0 {
		return JobResponse{}, errors.New("at least one input file is required")
	}

	now := time.Now().UTC()
	job := repository.Job{
		ID:        idgen.New("job"),
		Type:      req.Type,
		Status:    "queued",
		Progress:  0,
		Options:   req.Options,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.CreateJob(ctx, job); err != nil {
		return JobResponse{}, err
	}
	if err := s.queue.PublishJob(ctx, queue.JobMessage{JobID: job.ID, Type: job.Type}); err != nil {
		return JobResponse{}, err
	}

	return toJobResponse(job), nil
}

func (s *JobService) Get(ctx context.Context, jobID string) (JobResponse, error) {
	job, err := s.repo.GetJob(ctx, jobID)
	if err != nil {
		return JobResponse{}, err
	}
	return toJobResponse(job), nil
}

func toJobResponse(job repository.Job) JobResponse {
	return JobResponse{
		ID:        job.ID,
		Type:      job.Type,
		Status:    job.Status,
		Progress:  job.Progress,
		Options:   job.Options,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
	}
}
