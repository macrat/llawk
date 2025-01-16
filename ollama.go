package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ollama/ollama/api"
)

type OllamaLLM struct {
	Model  string
	Client *api.Client
}

func (m *OllamaLLM) Close() error {
	return nil
}

func (m *OllamaLLM) Invoke(ctx context.Context, w io.Writer, r *Request) error {
	messages := []api.Message{
		{
			Role:    "system",
			Content: r.SystemPrompt(),
		},
		{
			Role:    "user",
			Content: r.UserPrompt(),
		},
	}

	stream := true
	req := api.ChatRequest{
		Model:    m.Model,
		Messages: messages,
		Stream:   &stream,
	}

	switch r.Format {
	case "plain text":
		// no-op
	case "JSON":
		req.Format = json.RawMessage(`"json"`)
	case "JSON Schema":
		req.Format = json.RawMessage(r.Schema)
	default:
		return fmt.Errorf("Model %s does not support format %s", m.Model, r.Format)
	}

	err := m.Client.Chat(ctx, &req, func(resp api.ChatResponse) error {
		fmt.Fprint(w, resp.Message.Content)
		return nil
	})

	if err != nil {
		return fmt.Errorf("Failed to generate content: %w", err)
	}
	return nil
}

type OllamaDialer struct {
}

func (d OllamaDialer) Dial(model string) (LLM, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("Failed to create Ollama client: %w", err)
	}

	return &OllamaLLM{
		Model:  strings.TrimPrefix(model, "ollama:"),
		Client: client,
	}, nil
}
