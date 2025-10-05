package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"go-little-userbot-maker/internal/orchestrator/usecase" // This will be created/updated next
)

// SessionUsecase defines the interface for session-related business logic.
// This allows the handler to be decoupled from the specific implementation.
type SessionUsecase interface {
	CreateSession(ctx context.Context, req usecase.CreateSessionInput) (usecase.CreateSessionResult, error)
	DeleteSession(ctx context.Context, telegramID int64) error
	UpdateFeature(ctx context.Context, feature string, telegramID int64, payload map[string]any) error
	GetStats() map[string]any
	HealthCheck(ctx context.Context) error
}

// Handler manages HTTP requests and delegates to the usecase layer.
type Handler struct {
	log     *zap.Logger
	usecase SessionUsecase
}

// NewHandler creates a new HTTP handler for the orchestrator.
func NewHandler(log *zap.Logger, usecase SessionUsecase) *Handler {
	return &Handler{
		log:     log,
		usecase: usecase,
	}
}

// RegisterRoutes connects the handler's methods to a router.
func (h *Handler) RegisterRoutes(r *chi.Mux) {
	r.Get("/healthz", h.handleHealth)
	r.Post("/sessions", h.handleCreateSession)
	r.Delete("/sessions/{telegramID}", h.handleDeleteSession)
	r.Get("/stats/database", h.handleStats)
	r.Patch("/features/{feature}/{telegramID}", h.handlePatchFeature)
}

// SessionRequest is the DTO for creating a new session.
type SessionRequest struct {
	TelegramID      int64             `json:"telegram_id"`
	Session         string            `json:"session_string"`
	LoginMethod     string            `json:"login_method"`
	Metadata        map[string]string `json:"metadata"`
	SessionHash     string            `json:"session_hash"`
	RequestID       string            `json:"request_id"`
	Origin          string            `json:"origin"`
	OwnerTelegramID int64             `json:"owner_telegram_id"`
	OwnerUsername   string            `json:"owner_username"`
	OwnerFullName   string            `json:"owner_full_name"`
	SessionType     string            `json:"session_type"`
	BotUsername     string            `json:"bot_username"`
	BotDisplayName  string            `json:"bot_display_name"`
	Features        map[string]any    `json:"features"`
	Config          map[string]any    `json:"config"`
}

type featurePatchRequest struct {
	Payload map[string]any `json:"payload"`
}

func (h *Handler) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req SessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.TelegramID == 0 || req.Session == "" {
		writeError(w, http.StatusBadRequest, errValidation("telegram_id and session_string are required"))
		return
	}
	if req.LoginMethod == "" {
		req.LoginMethod = "unknown"
	}

	if req.Metadata == nil {
		req.Metadata = map[string]string{}
	}
	if req.Features == nil {
		req.Features = map[string]any{}
	}
	if req.Config == nil {
		req.Config = map[string]any{}
	}
	if req.OwnerTelegramID == 0 {
		req.OwnerTelegramID = req.TelegramID
	}

	input := usecase.CreateSessionInput{
		TelegramID:      req.TelegramID,
		Session:         req.Session,
		LoginMethod:     req.LoginMethod,
		Metadata:        req.Metadata,
		SessionHash:     req.SessionHash,
		RequestID:       req.RequestID,
		Origin:          req.Origin,
		OwnerTelegramID: req.OwnerTelegramID,
		OwnerUsername:   req.OwnerUsername,
		OwnerFullName:   req.OwnerFullName,
		SessionType:     req.SessionType,
		BotUsername:     req.BotUsername,
		BotDisplayName:  req.BotDisplayName,
		Features:        req.Features,
		Config:          req.Config,
	}

	result, err := h.usecase.CreateSession(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":        "created",
		"session_id":    result.SessionID,
		"owner_user_id": result.OwnerUserID,
	})
}

func (h *Handler) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "telegramID")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errValidation("invalid telegramID"))
		return
	}
	if err := h.usecase.DeleteSession(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	stats := h.usecase.GetStats()
	_ = json.NewEncoder(w).Encode(stats)
}

func (h *Handler) handlePatchFeature(w http.ResponseWriter, r *http.Request) {
	feature := chi.URLParam(r, "feature")
	telegramID, err := strconv.ParseInt(chi.URLParam(r, "telegramID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errValidation("invalid telegramID"))
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
	if err := h.usecase.UpdateFeature(r.Context(), feature, telegramID, payload.Payload); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := h.usecase.HealthCheck(r.Context()); err != nil {
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
