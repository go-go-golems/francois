// Package piworker owns the private model worker protocol.
package piworker

import (
	"context"
	"encoding/json"
)

type Model struct {
	Provider       string   `json:"provider"`
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Reasoning      bool     `json:"reasoning"`
	ThinkingLevels []string `json:"thinkingLevels,omitempty"`
	Images         bool     `json:"images"`
	Tier           string   `json:"tier,omitempty"`
}
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}
type Options struct {
	ModelTier string `json:"modelTier"`
	Model     string `json:"model,omitempty"`
	Effort    string `json:"effort,omitempty"`
}
type Image struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}
type Request struct {
	Images  []Image         `json:"images,omitempty"`
	CallID  string          `json:"callId"`
	Input   json.RawMessage `json:"input"`
	Options Options         `json:"options"`
	Tools   []Tool          `json:"tools,omitempty"`
}
type Result struct {
	Thinking         string  `json:"thinking,omitempty"`
	FinalText        *string `json:"finalText,omitempty"`
	Text             string  `json:"text"`
	Truncated        bool    `json:"truncated"`
	ModelTierApplied string  `json:"modelTierApplied"`
	ModelApplied     string  `json:"modelApplied,omitempty"`
	ModelUsed        string  `json:"modelUsed,omitempty"`
}
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Text    string `json:"text,omitempty"`
}

func (f *Failure) Error() string { return f.Message }

type Event struct {
	Type       string          `json:"type"`
	CallID     string          `json:"callId"`
	Text       string          `json:"text,omitempty"`
	Delta      string          `json:"delta,omitempty"`
	ToolCallID string          `json:"toolCallId,omitempty"`
	Name       string          `json:"name,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	Result     *Result         `json:"result,omitempty"`
	Error      *Failure        `json:"error,omitempty"`
	Models     []Model         `json:"models,omitempty"`
}
type ToolResult struct {
	Value   json.RawMessage `json:"value"`
	IsError bool            `json:"isError"`
}
type ExecuteTool func(context.Context, Event) (ToolResult, error)
type Backend interface {
	Models() []Model
	Sample(context.Context, Request, func(Event), ExecuteTool) error
	Close() error
}
