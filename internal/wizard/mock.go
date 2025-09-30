package wizard

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

type MockBot struct {
	log     *zap.Logger
	updates tgbotapi.UpdatesChannel
}

func NewMockBot(log *zap.Logger) *MockBot {
	return &MockBot{log: log, updates: make(chan tgbotapi.Update)}
}

func (m *MockBot) GetUpdatesChan(tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return m.updates
}

func (m *MockBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.log.Debug("mock send", zap.Any("payload", c))
	return tgbotapi.Message{}, nil
}

func (m *MockBot) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	m.log.Debug("mock request", zap.Any("payload", c))
	return &tgbotapi.APIResponse{Ok: true}, nil
}
