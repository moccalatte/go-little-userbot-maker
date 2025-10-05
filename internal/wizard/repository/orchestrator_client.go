package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

type SessionCreateResponse struct {
	Status      string `json:"status"`
	SessionID   int64  `json:"session_id"`
	OwnerUserID int64  `json:"owner_user_id"`
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

func (c *OrchestratorClient) CreateSession(ctx context.Context, payload SessionPayload) (SessionCreateResponse, error) {
	body, err := c.send(ctx, http.MethodPost, "/sessions", payload)
	if err != nil {
		return SessionCreateResponse{}, err
	}
	var resp SessionCreateResponse
	if len(body) == 0 {
		return resp, fmt.Errorf("empty response from orchestrator")
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return SessionCreateResponse{}, err
	}
	return resp, nil
}

func (c *OrchestratorClient) DeleteSession(ctx context.Context, telegramID int64) error {
	path := fmt.Sprintf("/sessions/%d", telegramID)
	_, err := c.send(ctx, http.MethodDelete, path, nil)
	return err
}

func (c *OrchestratorClient) PatchFeature(ctx context.Context, feature string, telegramID int64, data map[string]any) error {
	path := fmt.Sprintf("/features/%s/%d", feature, telegramID)
	_, err := c.send(ctx, http.MethodPatch, path, data)
	return err
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

func (c *OrchestratorClient) send(ctx context.Context, method, path string, payload any) ([]byte, error) {
	url := c.baseURL + path
	var bodyBytes []byte
	var err error
	if payload != nil {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("orchestrator %s %s failed: %s: %s", method, path, resp.Status, string(body))
	}
	return body, nil
}
