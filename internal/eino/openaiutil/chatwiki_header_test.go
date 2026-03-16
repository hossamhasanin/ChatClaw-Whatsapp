package openaiutil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openaicomponent "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

func TestWrapToolCallingChatModelWithToken_AddsTokenHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		if got := r.Header.Get("Token"); got != "test-token" {
			t.Fatalf("unexpected token header: %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "test-model",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "ok",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		})
	}))
	defer server.Close()

	base, err := openaicomponent.NewChatModel(context.Background(), &openaicomponent.ChatModelConfig{
		APIKey:  "test-token",
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewChatModel returned error: %v", err)
	}

	wrapped := WrapToolCallingChatModelWithToken(base, "test-token")
	msg, err := wrapped.Generate(context.Background(), []*schema.Message{{
		Role:    schema.User,
		Content: "hello",
	}})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if msg == nil || msg.Content != "ok" {
		t.Fatalf("unexpected response message: %#v", msg)
	}
}
