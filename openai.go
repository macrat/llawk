package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/openai/openai-go"
)

type OpenAILLM struct {
	Model  string
	Client *openai.Client
	Stream bool
}

func (m *OpenAILLM) Close() error {
	return nil
}

func (m *OpenAILLM) systemPrompt(r *Request) openai.ChatCompletionMessageParam {
	msg := openai.ChatCompletionMessageParam{
		Role:    openai.F(openai.ChatCompletionMessageParamRoleSystem),
		Content: openai.F[any](r.SystemPrompt()),
	}
	if m.Model == "o1" {
		msg.Role = openai.F(openai.ChatCompletionMessageParamRoleDeveloper)
	}
	return msg
}

func (m *OpenAILLM) Invoke(ctx context.Context, w io.Writer, r *Request) error {
	format := openai.ChatCompletionNewParamsResponseFormat{}
	switch r.Format {
	case "plain text":
		format.Type = openai.F(openai.ChatCompletionNewParamsResponseFormatTypeText)
	case "JSON":
		format.Type = openai.F(openai.ChatCompletionNewParamsResponseFormatTypeJSONObject)
	case "JSON Schema":
		format.Type = openai.F(openai.ChatCompletionNewParamsResponseFormatTypeJSONSchema)
		format.JSONSchema = openai.F[any](map[string]any{
			"name":   "Output",
			"schema": json.RawMessage(r.Schema),
		})
	default:
		return fmt.Errorf("Model %s does not support format %s", m.Model, r.Format)
	}

	req := openai.ChatCompletionNewParams{
		Model: openai.F(m.Model),
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(r.SystemPrompt()),
			openai.UserMessage(r.UserPrompt()),
		}),
		ResponseFormat: openai.F[openai.ChatCompletionNewParamsResponseFormatUnion](format),
	}
	if m.Model != "o1" {
		req.Temperature = openai.F(0.0)
	}

	if m.Stream {
		return m.invokeStream(ctx, w, req)
	} else {
		return m.invokeSimple(ctx, w, req)
	}
}

func (m *OpenAILLM) invokeStream(ctx context.Context, w io.Writer, req openai.ChatCompletionNewParams) error {
	stream := m.Client.Chat.Completions.NewStreaming(ctx, req)

	for stream.Next() {
		chunk := stream.Current()

		fmt.Fprint(w, chunk.Choices[0].Delta.Content)
	}

	err := stream.Err()
	if err != nil {
		return fmt.Errorf("Failed to generate content: %w", err)
	}
	return nil
}

func (m *OpenAILLM) invokeSimple(ctx context.Context, w io.Writer, req openai.ChatCompletionNewParams) error {
	resp, err := m.Client.Chat.Completions.New(ctx, req)
	if err != nil {
		return fmt.Errorf("Failed to generate content: %w", err)
	}

	fmt.Fprint(w, resp.Choices[0].Message.Content)
	return nil
}

type OpenAIDialer struct {
	Stream bool
}

func (d OpenAIDialer) Dial(model string) (LLM, error) {
	return &OpenAILLM{
		Model:  model,
		Client: openai.NewClient(),
		Stream: d.Stream,
	}, nil
}
