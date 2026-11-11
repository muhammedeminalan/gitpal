package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DefaultOpenAIURL is the OpenAI API; any service speaking its chat
// completions protocol works.
const DefaultOpenAIURL = "https://api.openai.com/v1"

// DefaultOllamaURL is where a local Ollama listens.
const DefaultOllamaURL = "http://localhost:11434"

// httpTimeout bounds one request to an HTTP provider.
const httpTimeout = 3 * time.Minute

type openAIClient struct {
	http    *http.Client
	baseURL string
	key     string
	model   string
}

// NewOpenAI returns a provider for an OpenAI-compatible chat completions
// API. The model must be named: gitpal does not guess one.
func NewOpenAI(apiKey, model, baseURL string) Provider {
	if baseURL == "" {
		baseURL = DefaultOpenAIURL
	}
	return &llm{name: "openai", c: &openAIClient{
		http: &http.Client{Timeout: httpTimeout}, baseURL: strings.TrimRight(baseURL, "/"), key: apiKey, model: model,
	}}
}

func (o *openAIClient) complete(ctx context.Context, system, user string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model": o.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	})
	if err != nil {
		return "", err
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := postJSON(ctx, o.http, "openai", o.baseURL+"/chat/completions", o.key, body, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", nil
	}
	return out.Choices[0].Message.Content, nil
}

type ollamaClient struct {
	http    *http.Client
	baseURL string
	model   string
}

// NewOllama returns a provider for a local Ollama: nothing leaves the
// machine. The model must be named.
func NewOllama(model, baseURL string) Provider {
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}
	return &llm{name: "ollama", c: &ollamaClient{
		http: &http.Client{Timeout: httpTimeout}, baseURL: strings.TrimRight(baseURL, "/"), model: model,
	}}
}

func (o *ollamaClient) complete(ctx context.Context, system, user string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model":  o.model,
		"stream": false,
		"format": "json",
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	})
	if err != nil {
		return "", err
	}
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := postJSON(ctx, o.http, "ollama", o.baseURL+"/api/chat", "", body, &out); err != nil {
		return "", err
	}
	return out.Message.Content, nil
}

// postJSON sends a request and decodes a successful answer into out.
func postJSON(ctx context.Context, client *http.Client, provider, url, key string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ai %s: build request: %w", provider, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := client.Do(req)
	if err != nil {
		return classifyTransport(ctx, provider, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return classifyHTTP(provider, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return &Error{Kind: KindEmpty, Provider: provider}
	}
	return nil
}
