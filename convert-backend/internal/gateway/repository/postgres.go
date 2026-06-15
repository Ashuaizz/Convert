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
		INSERT INTO jobs (id, user_id, type, status, progress, options, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, job.ID, job.UserID, job.Type, job.Status, job.Progress, options, job.CreatedAt, job.UpdatedAt)
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
	var job Job
	var options []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, type, status, progress, options, created_at, updated_at
		FROM jobs
		WHERE id = $1
	`, id).Scan(&job.ID, &job.UserID, &job.Type, &job.Status, &job.Progress, &options, &job.CreatedAt, &job.UpdatedAt)
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
	return job, nil
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
