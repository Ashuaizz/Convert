package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateFile(ctx context.Context, file File) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO files (id, user_id, filename, content_type, size, storage_uri, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, file.ID, file.UserID, file.Filename, file.ContentType, file.Size, file.StorageURI, file.Status, file.CreatedAt, file.UpdatedAt)
	return err
}

func (r *PostgresRepository) GetFile(ctx context.Context, id string) (File, error) {
	return scanFile(r.db.QueryRowContext(ctx, `
		SELECT id, user_id, filename, content_type, size, storage_uri, status, created_at, updated_at
		FROM files
		WHERE id = $1
	`, id))
}

func (r *PostgresRepository) UpdateFileStatus(ctx context.Context, id string, status string, updatedAt time.Time) (File, error) {
	return scanFile(r.db.QueryRowContext(ctx, `
		UPDATE files
		SET status = $2, updated_at = $3
		WHERE id = $1
		RETURNING id, user_id, filename, content_type, size, storage_uri, status, created_at, updated_at
	`, id, status, updatedAt))
}

func (r *PostgresRepository) CreateJob(ctx context.Context, job Job) error {
	options, err := json.Marshal(job.Options)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO jobs (id, user_id, type, status, progress, options, retry_count, max_retries, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, job.ID, job.UserID, job.Type, job.Status, job.Progress, options, job.RetryCount, maxRetries(job.MaxRetries), job.CreatedAt, job.UpdatedAt)
	return err
}

func (r *PostgresRepository) AttachJobFile(ctx context.Context, jobID string, fileID string, role string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO job_files (job_id, file_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (job_id, file_id, role) DO NOTHING
	`, jobID, fileID, role)
	return err
}

func (r *PostgresRepository) GetJob(ctx context.Context, id string) (Job, error) {
	return scanJob(r.db.QueryRowContext(ctx, `
		SELECT id, user_id, type, status, progress, options, COALESCE(message, ''),
		       COALESCE(error_code, ''), COALESCE(error_message, ''), retry_count, max_retries,
		       created_at, updated_at, started_at, finished_at
		FROM jobs
		WHERE id = $1
	`, id))
}

func (r *PostgresRepository) ClaimJob(ctx context.Context, id string, now time.Time) (Job, bool, error) {
	job, err := scanJob(r.db.QueryRowContext(ctx, `
		UPDATE jobs
		SET status = 'running', updated_at = $2, started_at = COALESCE(started_at, $2)
		WHERE id = $1 AND status IN ('queued', 'retrying')
		RETURNING id, user_id, type, status, progress, options, COALESCE(message, ''),
		          COALESCE(error_code, ''), COALESCE(error_message, ''), retry_count, max_retries,
		          created_at, updated_at, started_at, finished_at
	`, id, now))
	if errors.Is(err, ErrNotFound) {
		if existing, getErr := r.GetJob(ctx, id); getErr == nil {
			return existing, false, nil
		}
		return Job{}, false, ErrNotFound
	}
	if err != nil {
		return Job{}, false, err
	}
	return job, true, nil
}

func (r *PostgresRepository) SucceedJob(ctx context.Context, id string, now time.Time) (Job, error) {
	return scanJob(r.db.QueryRowContext(ctx, `
		UPDATE jobs
		SET status = 'succeeded', progress = 100, updated_at = $2, finished_at = $2
		WHERE id = $1 AND status = 'running'
		RETURNING id, user_id, type, status, progress, options, COALESCE(message, ''),
		          COALESCE(error_code, ''), COALESCE(error_message, ''), retry_count, max_retries,
		          created_at, updated_at, started_at, finished_at
	`, id, now))
}

func (r *PostgresRepository) FailJob(ctx context.Context, id string, code string, message string, now time.Time) (Job, error) {
	return scanJob(r.db.QueryRowContext(ctx, `
		UPDATE jobs
		SET status = 'failed', error_code = $2, error_message = $3,
		    retry_count = retry_count + 1, updated_at = $4, finished_at = $4
		WHERE id = $1 AND status = 'running'
		RETURNING id, user_id, type, status, progress, options, COALESCE(message, ''),
		          COALESCE(error_code, ''), COALESCE(error_message, ''), retry_count, max_retries,
		          created_at, updated_at, started_at, finished_at
	`, id, code, message, now))
}

func scanJob(row rowScanner) (Job, error) {
	var job Job
	var options []byte
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	err := row.Scan(
		&job.ID,
		&job.UserID,
		&job.Type,
		&job.Status,
		&job.Progress,
		&options,
		&job.Message,
		&job.ErrorCode,
		&job.ErrorMessage,
		&job.RetryCount,
		&job.MaxRetries,
		&job.CreatedAt,
		&job.UpdatedAt,
		&startedAt,
		&finishedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, err
	}
	if len(options) > 0 {
		if err := json.Unmarshal(options, &job.Options); err != nil {
			return Job{}, err
		}
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		job.FinishedAt = &finishedAt.Time
	}
	return job, nil
}

func maxRetries(value int) int {
	if value <= 0 {
		return 2
	}
	return value
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFile(row rowScanner) (File, error) {
	var file File
	err := row.Scan(
		&file.ID,
		&file.UserID,
		&file.Filename,
		&file.ContentType,
		&file.Size,
		&file.StorageURI,
		&file.Status,
		&file.CreatedAt,
		&file.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrNotFound
	}
	if err != nil {
		return File{}, err
	}
	return file, nil
}
