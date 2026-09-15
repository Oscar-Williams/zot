package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoverLMStudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected discovery request: %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"models":[
		{"type":"llm","key":"model-key","display_name":"Local Model","max_context_length":262144,"loaded_instances":[{"id":"instance","config":{"context_length":50176}}]},
		{"type":"llm","key":"unloaded@q4","max_context_length":8192},
		{"type":"llm","key":"unloaded@q4"},
		{"type":"embedding","key":"embedding"},
		{"type":"llm","key":""}]}`)
	}))
	defer server.Close()
	models, err := DiscoverLMStudio(context.Background(), server.URL+"/v1/", "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("models = %+v", models)
	}
	m := models[0]
	if m.ID != "instance" || m.ContextWindow != 50176 || m.DisplayName != "Local Model" || m.BaseURL != server.URL+"/v1" || m.Provider != LMStudioProviderID || m.Reasoning || m.PriceInput != 0 {
		t.Fatalf("unexpected metadata: %+v", m)
	}
	if models[1].ID != "unloaded@q4" || models[1].MaxOutput != 2048 {
		t.Fatalf("unloaded model = %+v", models[1])
	}
}

func TestDiscoverLMStudioFallbackAndErrors(t *testing.T) {
	for _, status := range []int{404, 405, 501, 401, 500, 200} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			fallbackCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("keyless request sent authorization")
				}
				if r.URL.Path == "/api/v1/models" {
					w.WriteHeader(status)
					fmt.Fprint(w, `{`)
					return
				}
				fallbackCalls++
				fmt.Fprint(w, `{"data":[{"id":"qwen/local@q4"},{"id":"qwen/local@q4"},{"id":"text-embedding-test"}]}`)
			}))
			defer server.Close()
			models, err := DiscoverLMStudio(context.Background(), server.URL, "")
			if status == 404 || status == 405 || status == 501 {
				if err != nil || fallbackCalls != 1 || len(models) != 1 || models[0].ContextWindow != 32768 {
					t.Fatalf("models=%+v error=%v calls=%d", models, err, fallbackCalls)
				}
			} else if err == nil || fallbackCalls != 0 {
				t.Fatalf("error=%v fallback calls=%d", err, fallbackCalls)
			}
		})
	}
}

func TestDiscoverLMStudioEmptyAndCanceled(t *testing.T) {
	for _, body := range []string{`{"models":[]}`, `{}`, `null`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			models, err := DiscoverLMStudio(context.Background(), server.URL, "")
			if body == `{"models":[]}` {
				if err != nil || len(models) != 0 {
					t.Fatalf("models=%v error=%v", models, err)
				}
			} else if err == nil {
				t.Fatal("missing model list accepted")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := DiscoverLMStudio(ctx, server.URL, ""); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel error=%v", err)
			}
		})
	}
}

func TestNormalizeLMStudioURL(t *testing.T) {
	for _, suffix := range []string{"", "/", "/v1", "/api/v1/"} {
		got, err := NormalizeLMStudioURL("http://localhost:1234/proxy" + suffix)
		if err != nil || got != "http://localhost:1234/proxy" {
			t.Fatalf("normalize %q: %q, %v", suffix, got, err)
		}
	}
	for _, value := range []string{"", "localhost:1234", "file:///tmp", "https://secret@example.com", "http://localhost?token=secret"} {
		if _, err := NormalizeLMStudioURL(value); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("invalid URL error: %v", err)
		}
	}
}

func TestManagedLocalProvidersRemainIndependent(t *testing.T) {
	SetManagedModels(nil)
	t.Cleanup(func() { SetManagedModels(nil) })
	SetManagedModelsForProvider(LlamaCPPProviderID, []Model{{Provider: LlamaCPPProviderID, ID: "llama"}})
	SetManagedModelsForProvider(LMStudioProviderID, []Model{{Provider: LMStudioProviderID, ID: "studio"}})
	SetManagedModelsForProvider(LlamaCPPProviderID, nil)
	if _, err := FindModel(LMStudioProviderID, "studio"); err != nil {
		t.Fatal(err)
	}
	if _, err := FindModel(LlamaCPPProviderID, "llama"); err == nil {
		t.Fatal("stale llama model")
	}
	SetUserModels([]Model{{Provider: LMStudioProviderID, ID: "studio", ContextWindow: 12345, Source: "user"}})
	t.Cleanup(func() { SetLiveModels(nil) })
	model, err := FindModel(LMStudioProviderID, "studio")
	if err != nil || model.ContextWindow != 12345 {
		t.Fatalf("user override lost: %+v, %v", model, err)
	}
}
