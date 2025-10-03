package telegram

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// Usecase is an interface for the wizard usecase.
// This makes the handler testable in isolation.
type Usecase interface {
	HandleUpdate(ctx context.Context, update tgbotapi.Update) error
}

// Handler is the delivery layer for Telegram updates.
type Handler struct {
	usecase Usecase
	log     *zap.Logger
}

// NewHandler creates a new Telegram handler.
func NewHandler(usecase Usecase, log *zap.Logger) *Handler {
	return &Handler{
		usecase: usecase,
		log:     log.Named("telegram_handler"),
	}
}

// HandleUpdate receives an update from the main service loop and passes it to the usecase.
func (h *Handler) HandleUpdate(ctx context.Context, update tgbotapi.Update) {
	if err := h.usecase.HandleUpdate(ctx, update); err != nil {
		h.log.Error("failed to handle update",
			zap.Error(err),
			zap.Int("update_id", update.UpdateID),
		)
	}
}