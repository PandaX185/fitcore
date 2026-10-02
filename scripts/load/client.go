package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// client is a thin HTTP wrapper for the API under test. Reused across seed,
// correctness and scenario setup; load phases bypass it and go straight to
// vegeta with pre-built requests.
type client struct {
	base  string
	token string
	hc    *http.Client
}

func newClient(base string) *client {
	return &client{
		base: base,
		hc:   &http.Client{Timeout: 30 * time.Second},
	}
}

// login authenticates and caches the access token.
func (c *client) login(email, password string) error {
	body, err := json.Marshal(map[string]string{"email": email, "password": password})
	if err != nil {
		return err
	}
	resp, status, err := c.do("POST", "/auth/login", body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("login failed: HTTP %d: %s", status, truncate(string(resp), 200))
	}
	var out struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(resp, &out); err != nil {
		return fmt.Errorf("login: parse response: %w", err)
	}
	if out.AccessToken == "" {
		return fmt.Errorf("login: no accessToken in response")
	}
	c.token = out.AccessToken
	return nil
}

// do issues an authenticated JSON request and returns the body and status.
// A nil body means GET with no payload.
func (c *client) do(method, path string, payload []byte) ([]byte, int, error) {
	var rdr io.Reader
	if payload != nil {
		rdr = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return b, resp.StatusCode, nil
}

// postJSON marshals v and POSTs it, expecting status want (200 default 201).
func (c *client) postJSON(path string, v any) ([]byte, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	body, status, err := c.do("POST", path, payload)
	if err != nil {
		return nil, err
	}
	if status < 200 || status > 299 {
		return nil, fmt.Errorf("POST %s -> HTTP %d: %s", path, status, truncate(string(body), 200))
	}
	return body, nil
}

// idOf extracts "id" from a JSON object response.
func idOf(body []byte, label string) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("%s: parse id: %w", label, err)
	}
	if out.ID == "" {
		return "", fmt.Errorf("%s: empty id in response", label)
	}
	return out.ID, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
