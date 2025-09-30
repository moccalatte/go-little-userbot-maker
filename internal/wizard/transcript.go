package wizard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

type TranscriptWriter struct {
	basePath string
	log      *zap.Logger
}

func NewTranscriptWriter(basePath string, log *zap.Logger) *TranscriptWriter {
	return &TranscriptWriter{basePath: basePath, log: log}
}

func (w *TranscriptWriter) Write(chatID int64, update tgbotapi.Update, response string, state FlowState, err error) {
	if w == nil {
		return
	}

	date := time.Now().UTC().Format("2006-01-02")
	dir := filepath.Join(w.basePath, fmt.Sprint(chatID))
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		w.log.Error("mkdir transcript", zap.Error(mkErr))
		return
	}
	filePath := filepath.Join(dir, fmt.Sprintf("%s.jsonl", date))

	payload := map[string]any{
		"ts":       time.Now().UTC(),
		"chat_id":  chatID,
		"state":    state,
		"response": response,
	}
	if update.Message != nil {
		payload["message"] = map[string]any{
			"id":       update.Message.MessageID,
			"text":     maskSensitive(update.Message.Text),
			"from":     update.Message.From,
			"entities": update.Message.Entities,
		}
	}
	if err != nil {
		payload["error"] = err.Error()
	}

	line, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		w.log.Error("marshal transcript", zap.Error(marshalErr))
		return
	}

	f, openErr := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if openErr != nil {
		w.log.Error("open transcript", zap.Error(openErr))
		return
	}
	defer f.Close()

	if _, writeErr := f.Write(append(line, '\n')); writeErr != nil {
		w.log.Error("write transcript", zap.Error(writeErr))
	}
}

func maskSensitive(text string) string {
	if text == "" {
		return text
	}
	lowered := strings.ToLower(text)
	keywords := []string{"otp", "password", "token", "session"}
	for _, keyword := range keywords {
		if strings.Contains(lowered, keyword) {
			return "[REDACTED]"
		}
	}
	if len(text) > 80 {
		return text[:80] + "…"
	}
	return text
}
