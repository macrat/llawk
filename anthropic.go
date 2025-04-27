package main

import (
	"context"
	"fmt"
	"io"

	"github.com/anthropics/anthropic-sdk-go"
)

type AnthropicLLM struct {
	Model     string
	MaxTokens int64
	Client    anthropic.Client
}

func (m *AnthropicLLM) Close() error {
	return nil
}

func (m *AnthropicLLM) Invoke(ctx context.Context, w io.Writer, r *Request) error {
	stream := m.Client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     m.Model,
		MaxTokens: m.MaxTokens,
		System: []anthropic.TextBlockParam{
			{Text: r.SystemPrompt()},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(r.UserPrompt())),
		},
		Temperature: anthropic.Float(0.0),
	})
	defer stream.Close()

	for stream.Next() {
		event := stream.Current()

		_, err := fmt.Fprint(w, event.Delta.Text)
		if err != nil {
			return fmt.Errorf("Failed to write response: %w", err)
		}
	}

	return stream.Err()
}

type AnthropicDialer struct {
	Model     string
	MaxTokens int64
}

func (d AnthropicDialer) Dial(model string) (LLM, error) {
	return &AnthropicLLM{
		Model:     d.Model,
		MaxTokens: d.MaxTokens,
		Client:    anthropic.NewClient(),
	}, nil
}
