package vlm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteAPIVLM_UsesMaxCompletionTokensForGPT5(t *testing.T) {
	var captured map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	model, err := NewRemoteAPIVLM(&Config{
		BaseURL:   srv.URL,
		APIKey:    "test-key",
		ModelName: "gpt-5.4",
		Provider:  "openai",
	})
	if err != nil {
		t.Fatalf("NewRemoteAPIVLM error: %v", err)
	}

	out, err := model.Predict(context.Background(), [][]byte{[]byte("fake-image")}, "hello")
	if err != nil {
		t.Fatalf("Predict error: %v", err)
	}
	if out != "ok" {
		t.Fatalf("unexpected output: %q", out)
	}

	if got, ok := captured["max_completion_tokens"].(float64); !ok || int(got) != defaultMaxToks {
		t.Fatalf("expected max_completion_tokens=%d, got %#v", defaultMaxToks, captured["max_completion_tokens"])
	}
	if _, exists := captured["max_tokens"]; exists {
		t.Fatalf("max_tokens should not be sent for GPT-5 VLM, got %#v", captured["max_tokens"])
	}
}

func TestRemoteAPIVLM_UsesMaxTokensForNonGPT5(t *testing.T) {
	var captured map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	model, err := NewRemoteAPIVLM(&Config{
		BaseURL:   srv.URL,
		APIKey:    "test-key",
		ModelName: "gpt-4.1",
		Provider:  "openai",
	})
	if err != nil {
		t.Fatalf("NewRemoteAPIVLM error: %v", err)
	}

	if _, err := model.Predict(context.Background(), [][]byte{[]byte("fake-image")}, "hello"); err != nil {
		t.Fatalf("Predict error: %v", err)
	}

	if got, ok := captured["max_tokens"].(float64); !ok || int(got) != defaultMaxToks {
		t.Fatalf("expected max_tokens=%d, got %#v", defaultMaxToks, captured["max_tokens"])
	}
	if _, exists := captured["max_completion_tokens"]; exists {
		t.Fatalf("max_completion_tokens should not be sent for non GPT-5 VLM, got %#v", captured["max_completion_tokens"])
	}
}
