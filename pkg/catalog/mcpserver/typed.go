package mcpserver

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/go-go-golems/francois/pkg/catalog/client"
	"github.com/go-go-golems/go-go-mcp/pkg/embeddable"
	"github.com/go-go-golems/go-go-mcp/pkg/protocol"
	"github.com/google/jsonschema-go/jsonschema"
)

// maxText bounds the JSON echoed as text content for clients that ignore
// structured content.
const maxText = 60000

// spec is one tool in the catalogue.
type spec struct {
	name, toolset, description string
	scope                      string // "" for any token
	readOnly, destructive      bool
	localOnly                  bool
	option                     func(e *env) embeddable.ServerOption
}

// tool builds a spec from a typed handler. In is decoded from the call's
// arguments; its JSON Schema (from json and jsonschema struct tags) is the
// tool's input schema. A struct Out is returned as structured content with an
// output schema; the text content repeats it as JSON, prefixed with summary.
func tool[In, Out any](name, toolset, scope string, readOnly, destructive bool, description string,
	fn func(ctx context.Context, e *env, in In) (Out, string, error)) spec {
	s := spec{name: name, toolset: toolset, scope: scope, readOnly: readOnly, destructive: destructive, description: description}
	s.option = func(e *env) embeddable.ServerOption {
		opts := []embeddable.ToolOption{
			embeddable.WithDescription(description),
			embeddable.WithSchema(schemaMap[In]()),
			embeddable.WithToolAnnotations(protocol.ToolAnnotations{ReadOnlyHint: readOnly, DestructiveHint: &destructive, IdempotentHint: readOnly}),
		}
		structured := reflect.TypeFor[Out]().Kind() == reflect.Struct
		if structured {
			if raw, err := json.Marshal(must(jsonschema.For[Out](nil))); err == nil {
				opts = append(opts, embeddable.WithOutputSchema(json.RawMessage(raw)))
			}
		}
		return embeddable.WithTool(name, func(ctx context.Context, args map[string]any) (*protocol.ToolResult, error) {
			var in In
			raw, _ := json.Marshal(args)
			if err := json.Unmarshal(raw, &in); err != nil {
				return protocol.NewToolResult(protocol.WithError("invalid arguments: " + err.Error())), nil
			}
			out, summary, err := fn(ctx, e, in)
			if err != nil {
				return protocol.NewToolResult(protocol.WithError(toolError(err))), nil
			}
			text := summary
			if js, err := json.Marshal(out); err == nil && string(js) != "null" && string(js) != "{}" {
				body := string(js)
				if len(body) > maxText {
					body = body[:maxText] + "… (truncated)"
				}
				if text != "" {
					text += "\n"
				}
				text += body
			}
			ropts := []protocol.ToolResultOption{protocol.WithText(text)}
			if structured {
				ropts = append(ropts, protocol.WithStructuredContent(out))
			}
			return protocol.NewToolResult(ropts...), nil
		}, opts...)
	}
	return s
}

// schemaMap infers In's JSON Schema as a map, the form go-go-mcp keeps.
func schemaMap[In any]() map[string]any {
	raw, err := json.Marshal(must(jsonschema.For[In](nil)))
	if err != nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// toolError reports an API error as "code: message" (and release checks), so
// the model can correct its call.
func toolError(err error) string {
	var b strings.Builder
	b.WriteString(err.Error())
	if e, ok := err.(*client.Error); ok {
		for _, c := range e.Checks {
			b.WriteString("\n- " + c.Level + " " + c.Code + ": " + c.Message)
		}
	}
	return b.String()
}

// none is the input of tools without arguments.
type none struct{}

// items wraps lists, because structured content must be an object.
type items[T any] struct {
	Items []T `json:"items"`
}

// done is the output of calls without a body.
type done struct {
	OK bool `json:"ok"`
}
