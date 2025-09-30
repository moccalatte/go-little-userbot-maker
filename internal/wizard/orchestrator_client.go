package wizard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type OrchestratorClient struct {
	baseURL string
	http    *http.Client
	log     *zap.Logger
}

type SessionPayload struct {
	TelegramID  int64             `json:"telegram_id"`
	Session     string            `json:"session_string"`
	LoginMethod string            `json:"login_method"`
	Metadata    map[string]string `json:"metadata"`
	SessionHash string            `json:"session_hash"`
	RequestID   string            `json:"request_id"`
	Origin      string            `json:"origin"`
}

func NewOrchestratorClient(baseURL string, log *zap.Logger) *OrchestratorClient {
	return &OrchestratorClient{
		baseURL: baseURL,
		log:     log,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *OrchestratorClient) CreateSession(ctx context.Context, payload SessionPayload) error {
	return c.send(ctx, http.MethodPost, "/sessions", payload)
}

func (c *OrchestratorClient) DeleteSession(ctx context.Context, telegramID int64) error {
	path := fmt.Sprintf("/sessions/%d", telegramID)
	return c.send(ctx, http.MethodDelete, path, nil)
}

func (c *OrchestratorClient) PatchFeature(ctx context.Context, feature string, telegramID int64, data any) error {
	path := fmt.Sprintf("/features/%s/%d", feature, telegramID)
	return c.send(ctx, http.MethodPatch, path, data)
}

func (c *OrchestratorClient) FetchStats(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/stats/database", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("stats request failed: %s", resp.Status)
	}
	return nil
}

func (c *OrchestratorClient) send(ctx context.Context, method, path string, payload any) error {
	url := c.baseURL + path
	var bodyBytes []byte
	var err error
	if payload != nil {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("orchestrator %s %s failed: %s", method, path, resp.Status)
	}
	return nil
}
