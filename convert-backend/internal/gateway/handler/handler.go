package handler

import (
	"encoding/json"
	"net/http"

	"convert-backend/internal/gateway/middleware"
	"convert-backend/internal/gateway/service"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	jobs *service.JobService
}

func NewRouter(jobs *service.JobService) http.Handler {
	h := &Handler{jobs: jobs}

	r := chi.NewRouter()
	r.Get("/healthz", h.health)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/files/presign", h.presignUpload)
		r.Post("/files/{file_id}/complete", h.completeUpload)
		r.Get("/files/{file_id}/download", h.presignDownload)
		r.Post("/jobs", h.createJob)
		r.Get("/jobs/{job_id}", h.getJob)
	})

	return r
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeOK(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) presignUpload(w http.ResponseWriter, r *http.Request) {
	var req service.PresignUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body")
		return
	}
	if req.UserID == "" {
		req.UserID = r.Header.Get("X-User-ID")
	}
	result, err := h.jobs.PresignUpload(r.Context(), req)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}
	writeOK(w, r, http.StatusOK, result)
}

func (h *Handler) completeUpload(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	if fileID == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "missing file id")
		return
	}

	file, err := h.jobs.CompleteUpload(r.Context(), fileID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeOK(w, r, http.StatusOK, file)
}

func (h *Handler) presignDownload(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	if fileID == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "missing file id")
		return
	}

	result, err := h.jobs.PresignDownload(r.Context(), fileID)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}
	writeOK(w, r, http.StatusOK, result)
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	var req service.CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body")
		return
	}
	job, err := h.jobs.Create(r.Context(), req)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}
	writeOK(w, r, http.StatusAccepted, job)
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "job_id")
	if jobID == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "missing job id")
		return
	}
	job, err := h.jobs.Get(r.Context(), jobID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeOK(w, r, http.StatusOK, job)
}

type envelope struct {
	RequestID string `json:"request_id,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
}

func writeOK(w http.ResponseWriter, r *http.Request, status int, data any) {
	writeJSON(w, status, envelope{
		RequestID: middleware.RequestIDFromContext(r.Context()),
		Code:      "OK",
		Message:   "success",
		Data:      data,
	})
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, envelope{
		RequestID: middleware.RequestIDFromContext(r.Context()),
		Code:      code,
		Message:   message,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
