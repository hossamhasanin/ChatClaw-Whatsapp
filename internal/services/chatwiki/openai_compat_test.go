package chatwiki

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestResolveSelfOwnedModelConfigID_FetchesFromOpenAIEndpoint(t *testing.T) {
	openAIModelCatalogCache = sync.Map{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/manage/chatclaw/showModelConfigList" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Token"); got != "test-token" {
			t.Fatalf("unexpected token header: %q", got)
		}
		_, _ = w.Write([]byte(`{
			"data": {
				"language_models": [
					{"id": 12, "model_name": "deepseek-r1", "type": "llm", "enabled": 1}
				],
				"embedding_models": [
					{"id": 34, "model_name": "text-embedding", "type": "embedding", "enabled": 1}
				]
			}
		}`))
	}))
	defer server.Close()

	id, err := ResolveSelfOwnedModelConfigID("test-token", server.URL+"/chatclaw/v1", "deepseek-r1", "llm")
	if err != nil {
		t.Fatalf("ResolveSelfOwnedModelConfigID returned error: %v", err)
	}
	if id != 12 {
		t.Fatalf("expected config id 12, got %d", id)
	}

	embeddingID, err := ResolveSelfOwnedModelConfigID("test-token", server.URL+"/chatclaw/v1", "text-embedding", "embedding")
	if err != nil {
		t.Fatalf("ResolveSelfOwnedModelConfigID returned error for embedding: %v", err)
	}
	if embeddingID != 34 {
		t.Fatalf("expected embedding config id 34, got %d", embeddingID)
	}
}

func TestResolveSelfOwnedModelConfigID_ReturnsErrorWhenMissing(t *testing.T) {
	openAIModelCatalogCache = sync.Map{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"language_models":[]}}`))
	}))
	defer server.Close()

	_, err := ResolveSelfOwnedModelConfigID("test-token", server.URL+"/chatclaw/v1", "missing-model", "llm")
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}
