// Package solver — Python FastAPI(/generate, /validate) HTTP 클라이언트.
// Design Ref: §2.1 (internal-only), §7 (X-Internal-Token), §4.3
package solver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Client struct {
	BaseURL       string
	InternalToken string
	HTTP          *http.Client
}

func NewFromEnv() *Client {
	timeout := 300 * time.Second
	if v := os.Getenv("SOLVER_TIMEOUT_SECONDS"); v != "" {
		var s int
		_, _ = fmt.Sscanf(v, "%d", &s)
		if s > 0 {
			timeout = time.Duration(s) * time.Second
		}
	}
	return &Client{
		BaseURL:       envOr("SOLVER_URL", "http://ward-duty-solver:8000"),
		InternalToken: os.Getenv("SOLVER_INTERNAL_TOKEN"),
		HTTP:          &http.Client{Timeout: timeout},
	}
}

func (c *Client) Generate(ctx context.Context, in GenerateInput) (*GenerateOutput, error) {
	var out GenerateOutput
	if err := c.post(ctx, "/generate", in, &out); err != nil {
		return nil, err
	}
	switch out.Status {
	case "ok":
		return &out, nil
	case "infeasible":
		return &out, ErrInfeasible // caller가 sugesstion 등을 활용
	case "timeout":
		return &out, ErrTimeout
	case "error":
		return &out, ErrUnknown
	default:
		return &out, ErrUnknown
	}
}

func (c *Client) Validate(ctx context.Context, in ValidateInput) (*ValidateOutput, error) {
	var out ValidateOutput
	if err := c.post(ctx, "/validate", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	return nil
}

func (c *Client) post(ctx context.Context, path string, body, dst any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.InternalToken != "" {
		req.Header.Set("X-Internal-Token", c.InternalToken)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 500 {
		return fmt.Errorf("%w: status %d body %s", ErrUnavailable, resp.StatusCode, truncate(respBody, 300))
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("solver: internal token rejected")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("solver: status %d body %s", resp.StatusCode, truncate(respBody, 300))
	}
	if err := json.Unmarshal(respBody, dst); err != nil {
		return fmt.Errorf("solver: decode: %w", err)
	}
	return nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
