package orchestrator

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type SessionRequest struct {
	TelegramID  int64             `json:"telegram_id"`
	Session     string            `json:"session_string"`
	LoginMethod string            `json:"login_method"`
	Metadata    map[string]string `json:"metadata"`
	SessionHash string            `json:"session_hash"`
	RequestID   string            `json:"request_id"`
	Origin      string            `json:"origin"`
}

type featurePatchRequest struct {
	Payload map[string]any `json:"payload"`
}

func (s *Service) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req SessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.TelegramID == 0 || req.Session == "" {
		writeError(w, http.StatusBadRequest, errValidation("telegram_id dan session_string wajib"))
		return
	}
	if req.LoginMethod == "" {
		req.LoginMethod = "unknown"
	}
	if err := s.manager.CreateSession(r.Context(), req); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "created"})
}

func (s *Service) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "telegramID")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errValidation("telegramID tidak valid"))
		return
	}
	if err := s.manager.DeleteSession(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (s *Service) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	stats := s.manager.Stats()
	_ = json.NewEncoder(w).Encode(stats)
}

func (s *Service) handlePatchFeature(w http.ResponseWriter, r *http.Request) {
	feature := chi.URLParam(r, "feature")
	telegramID, err := strconv.ParseInt(chi.URLParam(r, "telegramID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errValidation("telegramID tidak valid"))
		return
	}
	var payload featurePatchRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if payload.Payload == nil {
		payload.Payload = map[string]any{}
	}
	if err := s.manager.UpdateFeature(r.Context(), feature, telegramID, payload.Payload); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.manager.Health(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func writeError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
}

type validationError struct {
	msg string
}

func (e validationError) Error() string { return e.msg }

func errValidation(msg string) error {
	return validationError{msg: msg}
}
