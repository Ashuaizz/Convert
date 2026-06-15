package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"convert-backend/internal/gateway/service"
)

type Handler struct {
	jobs *service.JobService
}

func RegisterRoutes(mux *http.ServeMux, jobs *service.JobService) {
	h := &Handler{jobs: jobs}
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("POST /api/v1/jobs", h.createJob)
	mux.HandleFunc("GET /api/v1/jobs/", h.getJob)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"code": "OK",
		"data": map[string]string{"status": "ok"},
	})
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	var req service.CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("INVALID_ARGUMENT", "invalid JSON body"))
		return
	}
	job, err := h.jobs.Create(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("INVALID_ARGUMENT", err.Error()))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"code": "OK",
		"data": job,
	})
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs/")
	if jobID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("INVALID_ARGUMENT", "missing job id"))
		return
	}
	job, err := h.jobs.Get(r.Context(), jobID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse("NOT_FOUND", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": "OK",
		"data": job,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func errorResponse(code, message string) map[string]any {
	return map[string]any{
		"code":    code,
		"message": message,
	}
}
