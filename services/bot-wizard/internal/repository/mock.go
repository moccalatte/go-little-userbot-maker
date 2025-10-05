package repository

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go-little-userbot-maker/pkg/logger"
)

type MockBot struct {
	log     *logger.Logger
	updates tgbotapi.UpdatesChannel
}

func NewMockBot(log *logger.Logger) *MockBot {
	return &MockBot{log: log, updates: make(chan tgbotapi.Update)}
}

func (m *MockBot) GetUpdatesChan(tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return m.updates
}

func (m *MockBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.log.Debug("mock send: %+v", c)
	return tgbotapi.Message{}, nil
}

func (m *MockBot) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	m.log.Debug("mock request: %+v", c)
	return &tgbotapi.APIResponse{Ok: true}, nil
}