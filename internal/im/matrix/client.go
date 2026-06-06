package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	baseURL     string
	accessToken string
	httpClient  *http.Client
}

func NewClient(homeserverURL, accessToken string) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(homeserverURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("matrix homeserver_url is required")
	}
	if accessToken == "" {
		return nil, fmt.Errorf("matrix access_token is required")
	}
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("invalid matrix homeserver_url: %w", err)
	}

	return &Client{
		baseURL:     baseURL,
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 70 * time.Second,
		},
	}, nil
}

type whoAmIResponse struct {
	UserID string `json:"user_id"`
}

func (c *Client) WhoAmI(ctx context.Context) (string, error) {
	var resp whoAmIResponse
	if err := c.do(ctx, http.MethodGet, "/_matrix/client/v3/account/whoami", nil, nil, &resp); err != nil {
		return "", err
	}
	if resp.UserID == "" {
		return "", fmt.Errorf("matrix whoami returned empty user_id")
	}
	return resp.UserID, nil
}

type syncResponse struct {
	NextBatch string `json:"next_batch"`
	Rooms     struct {
		Join map[string]struct {
			Timeline struct {
				Events []event `json:"events"`
			} `json:"timeline"`
		} `json:"join"`
	} `json:"rooms"`
}

func (c *Client) Sync(ctx context.Context, since string, timeoutMS int) (*syncResponse, error) {
	query := url.Values{}
	if since != "" {
		query.Set("since", since)
	}
	if timeoutMS > 0 {
		query.Set("timeout", fmt.Sprintf("%d", timeoutMS))
	}
	query.Set("set_presence", "offline")

	var resp syncResponse
	if err := c.do(ctx, http.MethodGet, "/_matrix/client/v3/sync", query, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetEvent(ctx context.Context, roomID, eventID string) (*event, error) {
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/event/%s", url.PathEscape(roomID), url.PathEscape(eventID))
	var evt event
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &evt); err != nil {
		return nil, err
	}
	evt.RoomID = roomID
	return &evt, nil
}

type sendMessageResponse struct {
	EventID string `json:"event_id"`
}

func (c *Client) SendMessage(ctx context.Context, roomID string, content map[string]any) (string, error) {
	txnID := uuid.NewString()
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s", url.PathEscape(roomID), url.PathEscape(txnID))

	var resp sendMessageResponse
	if err := c.do(ctx, http.MethodPut, path, nil, content, &resp); err != nil {
		return "", err
	}
	if resp.EventID == "" {
		return "", fmt.Errorf("matrix send message returned empty event_id")
	}
	return resp.EventID, nil
}

func (c *Client) SetTyping(ctx context.Context, roomID, userID string, typing bool, timeout time.Duration) error {
	if strings.TrimSpace(roomID) == "" || strings.TrimSpace(userID) == "" {
		return nil
	}

	payload := map[string]any{
		"typing": typing,
	}
	if typing && timeout > 0 {
		payload["timeout"] = int(timeout / time.Millisecond)
	}

	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/typing/%s", url.PathEscape(roomID), url.PathEscape(userID))
	return c.do(ctx, http.MethodPut, path, nil, payload, nil)
}

func (c *Client) DownloadMXC(ctx context.Context, mxcURL string) (io.ReadCloser, error) {
	serverName, mediaID, err := parseMXCURL(mxcURL)
	if err != nil {
		return nil, err
	}

	downloadPaths := []string{
		fmt.Sprintf("/_matrix/client/v1/media/download/%s/%s", url.PathEscape(serverName), url.PathEscape(mediaID)),
		fmt.Sprintf("/_matrix/media/v3/download/%s/%s", url.PathEscape(serverName), url.PathEscape(mediaID)),
	}

	var lastErr error
	for _, path := range downloadPaths {
		resp, err := c.doMediaRequest(ctx, http.MethodGet, path)
		if err != nil {
			lastErr = err
			continue
		}
		return resp.Body, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("matrix media download failed: no download path succeeded")
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	reqURL := c.baseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("matrix API %s %s failed: status=%d body=%s", method, path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) doMediaRequest(ctx context.Context, method, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("matrix media download failed: path=%s status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return resp, nil
}
