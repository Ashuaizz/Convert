package repository

import (
	"context"
	"database/sql"
	"time"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateFile(ctx context.Context, file File) error {
	return ErrNotImplemented
}

func (r *PostgresRepository) GetFile(ctx context.Context, id string) (File, error) {
	return File{}, ErrNotImplemented
}

func (r *PostgresRepository) UpdateFileStatus(ctx context.Context, id string, status string, updatedAt time.Time) (File, error) {
	return File{}, ErrNotImplemented
}

func (r *PostgresRepository) CreateJob(ctx context.Context, job Job) error {
	return ErrNotImplemented
}

func (r *PostgresRepository) AttachJobFile(ctx context.Context, jobID string, fileID string, role string) error {
	return ErrNotImplemented
}

func (r *PostgresRepository) GetJob(ctx context.Context, id string) (Job, error) {
	return Job{}, ErrNotImplemented
}
