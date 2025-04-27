package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type GoogleLLM struct {
	Model  string
	Client *genai.Client
}

func (m *GoogleLLM) Close() error {
	return m.Client.Close()
}

func (m *GoogleLLM) Invoke(ctx context.Context, w io.Writer, r *Request) error {
	model := m.Client.GenerativeModel(m.Model)

	temp := float32(0.0)
	model.Temperature = &temp

	if m.Model != "gemini-2.0-flash-lite" {
		model.Tools = []*genai.Tool{
			&genai.Tool{
				CodeExecution: &genai.CodeExecution{},
			},
		}
	}

	switch r.Format {
	case "plain text":
		model.ResponseMIMEType = "text/plain"
	case "JSON":
		model.ResponseMIMEType = "application/json"
	case "JSON Schema":
		model.ResponseMIMEType = "application/json"

		var schema JSONSchema
		err := json.Unmarshal([]byte(r.Schema), &schema)
		if err != nil {
			return fmt.Errorf("Invalid schema: %w", err)
		}
		model.ResponseSchema = schema.GeminiSchema()
	}

	model.SystemInstruction = genai.NewUserContent(genai.Text(r.SystemPrompt()))

	iter := model.GenerateContentStream(ctx, genai.Text(r.UserPrompt()))
	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("Failed to generate content: %w", err)
		}
		cand := resp.Candidates[0]
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				if p, ok := part.(genai.Text); ok {
					fmt.Fprint(w, p)
				}
			}
		}
	}

	return nil
}

type GoogleDialer struct {
}

func (d GoogleDialer) Dial(model string) (LLM, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		return nil, fmt.Errorf("Failed to create client: %w", err)
	}

	return &GoogleLLM{
		Model:  model,
		Client: client,
	}, nil
}

type JSONSchema struct {
	Type        string                `json:"type"`
	Format      string                `json:"format,omitempty"`
	Description string                `json:"description,omitempty"`
	Nullable    bool                  `json:"nullable,omitempty"`
	Enum        []string              `json:"enum,omitempty"`
	Properties  map[string]JSONSchema `json:"properties,omitempty"`
	Required    []string              `json:"required,omitempty"`
	Items       *JSONSchema           `json:"items,omitempty"`
}

func (s JSONSchema) GeminiSchema() *genai.Schema {
	var res genai.Schema

	switch s.Type {
	case "string":
		res.Type = genai.TypeString
	case "number":
		res.Type = genai.TypeNumber
	case "integer":
		res.Type = genai.TypeInteger
	case "boolean":
		res.Type = genai.TypeBoolean
	case "array":
		res.Type = genai.TypeArray
	case "object":
		res.Type = genai.TypeObject
	default:
		res.Type = genai.TypeUnspecified
	}

	res.Format = s.Format
	res.Description = s.Description
	res.Nullable = s.Nullable
	res.Enum = s.Enum

	if s.Properties != nil {
		res.Properties = make(map[string]*genai.Schema)
		for k, v := range s.Properties {
			res.Properties[k] = v.GeminiSchema()
		}
	}

	res.Required = s.Required

	if s.Items != nil {
		res.Items = s.Items.GeminiSchema()
	}

	return &res
}
