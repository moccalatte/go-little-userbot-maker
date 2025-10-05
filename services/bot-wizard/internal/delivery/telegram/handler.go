package telegram

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go-little-userbot-maker/pkg/logger"
)

// Usecase is an interface for the wizard usecase.
// This makes the handler testable in isolation.
type Usecase interface {
	HandleUpdate(ctx context.Context, update tgbotapi.Update) error
}

// Handler is the delivery layer for Telegram updates.
type Handler struct {
	usecase Usecase
	log     *logger.Logger
}

// NewHandler creates a new Telegram handler.
func NewHandler(usecase Usecase, log *logger.Logger) *Handler {
	return &Handler{
		usecase: usecase,
		log:     log,
	}
}

// HandleUpdate receives an update from the main service loop and passes it to the usecase.
func (h *Handler) HandleUpdate(ctx context.Context, update tgbotapi.Update) {
	if err := h.usecase.HandleUpdate(ctx, update); err != nil {
		h.log.Error("failed to handle update (ID: %d): %v", update.UpdateID, err)
	}
}