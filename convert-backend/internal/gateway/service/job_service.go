package service

import (
	"context"
	"errors"
	"fmt"
	"path"
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

type PresignUploadRequest struct {
	UserID      string `json:"user_id,omitempty"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

type PresignUploadResponse struct {
	FileID    string    `json:"file_id"`
	UploadURL string    `json:"upload_url"`
	URI       string    `json:"uri"`
	ExpiresAt time.Time `json:"expires_at"`
}

type FileResponse struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	URI         string    `json:"uri"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DownloadResponse struct {
	FileID      string    `json:"file_id"`
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
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

func (s *JobService) PresignUpload(ctx context.Context, req PresignUploadRequest) (PresignUploadResponse, error) {
	filename := cleanFilename(req.Filename)
	if filename == "" {
		return PresignUploadResponse{}, errors.New("filename is required")
	}
	if strings.TrimSpace(req.ContentType) == "" {
		return PresignUploadResponse{}, errors.New("content_type is required")
	}
	if req.Size <= 0 {
		return PresignUploadResponse{}, errors.New("size must be greater than zero")
	}

	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		userID = "dev-user"
	}

	now := time.Now().UTC()
	fileID := idgen.New("file")
	key := fmt.Sprintf("uploads/%s/%s/%s", userID, fileID, filename)
	uploadURL, err := s.storage.PresignUpload(ctx, key, req.ContentType)
	if err != nil {
		return PresignUploadResponse{}, err
	}

	file := repository.File{
		ID:          fileID,
		UserID:      userID,
		Filename:    filename,
		ContentType: req.ContentType,
		Size:        req.Size,
		StorageURI:  s.storage.URI(key),
		Status:      "pending_upload",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.CreateFile(ctx, file); err != nil {
		return PresignUploadResponse{}, err
	}

	return PresignUploadResponse{
		FileID:    file.ID,
		UploadURL: uploadURL,
		URI:       file.StorageURI,
		ExpiresAt: now.Add(s.storage.UploadExpiry()),
	}, nil
}

func (s *JobService) CompleteUpload(ctx context.Context, fileID string) (FileResponse, error) {
	file, err := s.repo.UpdateFileStatus(ctx, fileID, "uploaded", time.Now().UTC())
	if err != nil {
		return FileResponse{}, err
	}
	return toFileResponse(file), nil
}

func (s *JobService) PresignDownload(ctx context.Context, fileID string) (DownloadResponse, error) {
	file, err := s.repo.GetFile(ctx, fileID)
	if err != nil {
		return DownloadResponse{}, err
	}
	if file.Status != "uploaded" && file.Status != "ready" {
		return DownloadResponse{}, errors.New("file is not available for download")
	}
	key, err := storage.KeyFromURI(file.StorageURI)
	if err != nil {
		return DownloadResponse{}, err
	}
	downloadURL, err := s.storage.PresignDownload(ctx, key)
	if err != nil {
		return DownloadResponse{}, err
	}
	return DownloadResponse{
		FileID:      file.ID,
		DownloadURL: downloadURL,
		ExpiresAt:   time.Now().UTC().Add(s.storage.DownloadExpiry()),
	}, nil
}

func (s *JobService) Create(ctx context.Context, req CreateJobRequest) (JobResponse, error) {
	if strings.TrimSpace(req.Type) == "" {
		return JobResponse{}, errors.New("job type is required")
	}
	if len(req.InputFileIDs) == 0 {
		return JobResponse{}, errors.New("at least one input file is required")
	}
	userID := ""
	for _, fileID := range req.InputFileIDs {
		file, err := s.repo.GetFile(ctx, strings.TrimSpace(fileID))
		if err != nil {
			return JobResponse{}, fmt.Errorf("input file %q not found", fileID)
		}
		if file.Status != "uploaded" {
			return JobResponse{}, fmt.Errorf("input file %q is not uploaded", fileID)
		}
		if userID == "" {
			userID = file.UserID
		}
	}
	if userID == "" {
		userID = "dev-user"
	}

	now := time.Now().UTC()
	job := repository.Job{
		ID:        idgen.New("job"),
		UserID:    userID,
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
	for _, fileID := range req.InputFileIDs {
		if err := s.repo.AttachJobFile(ctx, job.ID, strings.TrimSpace(fileID), "input"); err != nil {
			return JobResponse{}, err
		}
	}
	if err := s.queue.PublishJob(ctx, queue.JobMessage{JobID: job.ID, Type: job.Type, InputFileIDs: req.InputFileIDs}); err != nil {
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

func toFileResponse(file repository.File) FileResponse {
	return FileResponse{
		ID:          file.ID,
		UserID:      file.UserID,
		Filename:    file.Filename,
		ContentType: file.ContentType,
		Size:        file.Size,
		URI:         file.StorageURI,
		Status:      file.Status,
		CreatedAt:   file.CreatedAt,
		UpdatedAt:   file.UpdatedAt,
	}
}

func cleanFilename(filename string) string {
	filename = strings.TrimSpace(strings.ReplaceAll(filename, "\\", "/"))
	if filename == "" {
		return ""
	}
	return path.Base(filename)
}
