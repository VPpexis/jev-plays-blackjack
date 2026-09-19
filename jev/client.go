package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DefaultBaseURL = "https://openrouter.ai/api/alpha/decisions"

// Question is a single decision question. Criteria is `any` so it can be an
// object (noul/choice) or an array (score).
type Question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type DecisionRequest struct {
	Model     string              `json:"model"`
	State     string              `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// DecisionResponse matches the real server payload, e.g.
// {"model":"...","answers":{"is_urgent":{"type":"noul","noul":0.84}},"usage":{...}}
type DecisionResponse struct {
	Model    string                     `json:"model"`
	Answers  map[string]json.RawMessage `json:"answers"`
	Usage    json.RawMessage            `json:"usage,omitempty"`
	ID       string                     `json:"id,omitempty"`
	Provider string                     `json:"provider,omitempty"`
}

type Client struct {
	APIKey  string
	BaseURL string
	HTTP    *http.Client
	Headers map[string]string
}

func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:  apiKey,
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) Decide(ctx context.Context, req DecisionRequest) (*DecisionResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "https://github.com")
	httpReq.Header.Set("X-Title", "Go Jev Client")
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Go-Http-Client)")
	for k, v := range c.Headers {
		httpReq.Header.Set(k, v)
	}

	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(raw))
	}
	if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 && trimmed[0] == '<' {
		return nil, fmt.Errorf("received HTML instead of JSON")
	}

	var out DecisionResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &out, nil
}

// Answer is a parsed decision answer, tolerant of the different question types.
type Answer struct {
	Type          string
	Noul          *float64
	Choice        string
	Score         *float64
	Probabilities map[string]float64
	Fields        map[string]json.RawMessage
}

func ParseAnswer(raw json.RawMessage) Answer {
	a := Answer{Fields: map[string]json.RawMessage{}}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return a
	}
	a.Fields = fields

	if v, ok := fields["type"]; ok {
		_ = json.Unmarshal(v, &a.Type)
	}
	if v, ok := fields["noul"]; ok {
		var p float64
		if json.Unmarshal(v, &p) == nil {
			a.Noul = &p
		}
	}
	if v, ok := fields["choice"]; ok {
		var c string
		if json.Unmarshal(v, &c) == nil {
			a.Choice = c
		}
	}
	if v, ok := fields["score"]; ok {
		var s float64
		if json.Unmarshal(v, &s) == nil {
			a.Score = &s
		}
	}

	if v, ok := fields["probabilities"]; ok {
		var probs map[string]float64
		if json.Unmarshal(v, &probs) == nil {
			a.Probabilities = probs
		}
	}
	if a.Probabilities == nil {
		for k, v := range fields {
			switch k {
			case "type", "noul", "choice", "score":
				continue
			}
			var probs map[string]float64
			if json.Unmarshal(v, &probs) == nil && len(probs) > 0 {
				a.Probabilities = probs
				break
			}
		}
	}
	return a
}
