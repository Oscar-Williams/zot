package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/patriceckhart/zot/packages/provider"
)

func TestLMStudioDiscoveryRequiresRegisteredURL(t *testing.T) {
	t.Setenv("ZOT_HOME", t.TempDir())
	provider.SetManagedModels(nil)
	t.Cleanup(func() { provider.SetManagedModels(nil) })
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{"models":[{"type":"llm","key":"local-model","max_context_length":8192}]}`)
	}))
	defer server.Close()
	// Merely knowing an endpoint through an environment variable is not registration.
	t.Setenv("LMSTUDIO_BASE_URL", server.URL)
	if LMStudioConfigured() {
		t.Fatal("unexpected default registration")
	}
	if err := RefreshLMStudioModels(context.Background()); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 {
		t.Fatal("unregistered discovery made a request")
	}
	if err := AuthStoreFor().SetAPIKey(provider.LMStudioProviderID, "key-only"); err != nil {
		t.Fatal(err)
	}
	if err := RefreshLMStudioModels(context.Background()); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 {
		t.Fatal("key without URL enabled discovery")
	}
	if err := AuthStoreFor().SetEndpointCredential(provider.LMStudioProviderID, server.URL, ""); err != nil {
		t.Fatal(err)
	}
	if !LMStudioConfigured() {
		t.Fatal("missing registration")
	}
	if err := RefreshLMStudioModels(context.Background()); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests=%d", requests.Load())
	}
	m, err := provider.FindModel(provider.LMStudioProviderID, "local-model")
	if err != nil || m.ContextWindow != 8192 {
		t.Fatalf("model=%+v, err=%v", m, err)
	}
	r, err := Resolve(Args{Provider: provider.LMStudioProviderID, Model: "local-model", NoSkill: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.BaseURL != server.URL+"/v1" || r.Credential != "local" {
		t.Fatalf("incorrect local routing: %s", r.BaseURL)
	}
	if err := AuthStoreFor().Clear(provider.LMStudioProviderID); err != nil {
		t.Fatal(err)
	}
	if err := RefreshLMStudioModels(context.Background()); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 {
		t.Fatal("logout still fetched models")
	}
	if _, err := provider.FindModel(provider.LMStudioProviderID, "local-model"); err == nil {
		t.Fatal("logout retained discovered model")
	}
}

func TestLMStudioConfigSurvivesTransientCatalog(t *testing.T) {
	t.Setenv("ZOT_HOME", t.TempDir())
	provider.SetManagedModels(nil)
	t.Cleanup(func() { provider.SetManagedModels(nil) })
	if err := SaveConfig(Config{Provider: provider.LMStudioProviderID, Model: "gpt-5"}); err != nil {
		t.Fatal(err)
	}
	ValidateAndRepairConfig()
	cfg, err := LoadConfig()
	if err != nil || cfg.Provider != provider.LMStudioProviderID || cfg.Model != "gpt-5" {
		t.Fatalf("config=%+v error=%v", cfg, err)
	}
}

func TestLMStudioRefreshPreservesSnapshotOnFailure(t *testing.T) {
	t.Setenv("ZOT_HOME", t.TempDir())
	provider.SetManagedModels([]provider.Model{{Provider: provider.LMStudioProviderID, ID: "previous", BaseURL: "http://unused.invalid/v1", Source: "live"}})
	t.Cleanup(func() { provider.SetManagedModels(nil) })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer server.Close()
	if err := AuthStoreFor().SetEndpointCredential(provider.LMStudioProviderID, server.URL, ""); err != nil {
		t.Fatal(err)
	}
	if err := RefreshLMStudioModels(context.Background()); err == nil {
		t.Fatal("missing discovery error")
	}
	if _, err := provider.FindModel(provider.LMStudioProviderID, "previous"); err != nil {
		t.Fatal(err)
	}
	r, err := Resolve(Args{Provider: provider.LMStudioProviderID, Model: "previous", NoSkill: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.BaseURL != server.URL+"/v1" {
		t.Fatalf("stale endpoint used for inference: %s", r.BaseURL)
	}
}
