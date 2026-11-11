package ai

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// DefaultAnthropicModel is used when ai.model is empty.
const DefaultAnthropicModel = "claude-opus-5-5"

// anthropicMaxTokens leaves room for the model's thinking as well as the
// short answer.
const anthropicMaxTokens = 16000

type anthropicClient struct {
	client anthropic.Client
	model  string
}

// NewAnthropic returns the Anthropic provider. baseURL is for tests; empty
// means the public API.
func NewAnthropic(apiKey, model, baseURL string) Provider {
	if model == "" {
		model = DefaultAnthropicModel
	}
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithRequestTimeout(3 * time.Minute),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &llm{name: "anthropic", c: &anthropicClient{client: anthropic.NewClient(opts...), model: model}}
}

func (a *anthropicClient) complete(ctx context.Context, system, user string) (string, error) {
	msg, err := a.client.Beta.Messages.New(ctx, anthropic.BetaMessageNewParams{
		Model:     a.model,
		MaxTokens: anthropicMaxTokens,
		System:    []anthropic.BetaTextBlockParam{{Text: system}},
		Messages:  []anthropic.BetaMessageParam{anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(user))},
		// A request a safety classifier declines is answered by another
		// model in the same call instead of failing.
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
		Fallbacks: anthropic.BetaFallbacksParamOfDefault(),
	})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) {
			return "", classifyHTTP("anthropic", apiErr.StatusCode)
		}
		return "", classifyTransport(ctx, "anthropic", err)
	}
	if msg.StopReason == anthropic.BetaStopReasonRefusal {
		return "", &Error{Kind: KindRefused, Provider: "anthropic"}
	}
	var b strings.Builder
	for _, block := range msg.Content {
		if text, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String(), nil
}
