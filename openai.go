package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/responses"
)

type OpenAILLM struct {
	Model  string
	Client openai.Client
}

func (m *OpenAILLM) Close() error {
	return nil
}

func (m *OpenAILLM) Invoke(ctx context.Context, w io.Writer, r *Request) error {
	var format responses.ResponseFormatTextConfigUnionParam

	if r.Format == "plain text" {
		format.OfText = &responses.ResponseFormatTextParam{
			Type: "text",
		}
	} else if r.Format == "JSON" {
		format.OfJSONObject = &responses.ResponseFormatJSONObjectParam{
			Type: "json_type",
		}
	} else if r.Format == "JSON Schema" {
		var schema map[string]any
		if err := json.Unmarshal([]byte(r.Schema), &schema); err != nil {
			return fmt.Errorf("Invalid schema: %w", err)
		}
		format.OfJSONSchema = &responses.ResponseFormatTextJSONSchemaConfigParam{
			Name:   "Output",
			Schema: schema,
			Strict: openai.Bool(true),
			Type:   "json_schema",
		}
	}

	req := responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(r.UserPrompt()),
		},
		Model:        m.Model,
		Instructions: openai.String(r.SystemPrompt()),
		Truncation:   "auto",
		Text: responses.ResponseTextConfigParam{
			Format: format,
		},
	}

	if m.Model != "o4-mini" && m.Model != "o3" {
		req.Temperature = openai.Float(0.0)
	}

	stream := m.Client.Responses.NewStreaming(ctx, req)
	defer stream.Close()

	for stream.Next() {
		chunk := stream.Current()
		_, err := fmt.Fprintf(w, "%s", chunk.Delta)
		if err != nil {
			return fmt.Errorf("Failed to write chunk: %w", err)
		}
	}

	return stream.Err()
}

type OpenAIDialer struct{}

func (d OpenAIDialer) Dial(model string) (LLM, error) {
	return &OpenAILLM{
		Model:  model,
		Client: openai.NewClient(),
	}, nil
}
