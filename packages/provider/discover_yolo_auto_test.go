package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestDiscoverYoloAutoListsKeyModels(t *testing.T) {
	const body = `{"object":"list","data":[
		{"id":"yolo","object":"model","context_length":262144,"max_model_len":262144,
		 "thinking":["minimal","low","medium","high","xhigh"]},
		{"id":"yolo-small","object":"model","context_length":262144,"max_model_len":262144,
		 "thinking":null},
		{"id":"text-embedding-3-small","object":"model"}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("request path = %q", r.URL.Path)
		}
		if got := r.Header.Get("authorization"); got != "Bearer yolo_test" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("content-type", "application/json")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	models, err := DiscoverYoloAuto(context.Background(), "yolo_test", srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("models = %+v", models)
	}

	yolo := models[0]
	if yolo.Provider != "yolo-auto" || yolo.ID != "yolo" || yolo.DisplayName != "yolo" ||
		yolo.ContextWindow != 262144 || yolo.MaxOutput != yoloAutoMaxOutput || !yolo.Reasoning ||
		yolo.BaseURL != srv.URL || yolo.Source != "live" {
		t.Fatalf("yolo model = %+v", yolo)
	}

	small := models[1]
	if small.ID != "yolo-small" || small.ContextWindow != 262144 || small.Reasoning {
		t.Fatalf("second model = %+v", small)
	}
}

func TestDiscoverYoloAutoFallsBackToDocumentedLimits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"yolo"}]}`)
	}))
	defer srv.Close()

	models, err := DiscoverYoloAuto(context.Background(), "yolo_test", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ContextWindow != yoloAutoContextWindow {
		t.Fatalf("models = %+v", models)
	}
}

func TestYoloAutoDiscoveryReachesActiveCatalogAndWire(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"data":[
			{"id":"yolo","context_length":262144,"thinking":["minimal","low","medium","high","xhigh"]},
			{"id":"qwen3.8-flash","context_length":65536,"thinking":["high"]},
			{"id":"yolo-small","max_model_len":65536,"thinking":null}
		]}`)
	}))
	defer srv.Close()
	models, err := DiscoverYoloAuto(context.Background(), "test", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	SetLiveModels(models)
	t.Cleanup(func() { SetLiveModels(nil) })
	for _, discovered := range models {
		active, err := FindModel("yolo-auto", discovered.ID)
		if err != nil {
			t.Fatal(err)
		}
		if active.ContextWindow != discovered.ContextWindow || active.Reasoning != discovered.Reasoning || active.BaseURL != srv.URL {
			t.Errorf("active metadata does not match discovery: %+v", active)
		}
		if active.ID == "yolo" && !slices.Equal(AvailableReasoningLevels(active), []string{"", "minimum", "low", "medium", "high", "xhigh"}) {
			t.Errorf("available levels = %q", AvailableReasoningLevels(active))
		}
	}
	client := NewYoloAuto("test", srv.URL).(*openaiClient)
	for _, tc := range []struct{ model, level, want string }{
		{"yolo", "minimum", "minimal"},
		{"yolo", "xhigh", "xhigh"},
		{"yolo", "max", "xhigh"},
		{"yolo", "off", ""},
		{"qwen3.8-flash", "low", "high"},
		{"yolo-small", "high", ""},
	} {
		wire, err := client.buildRequest(Request{Model: tc.model, Reasoning: tc.level})
		if err != nil {
			t.Fatal(err)
		}
		if wire.ReasoningEffort != tc.want {
			t.Errorf("%s/%s effort = %q, want %q", tc.model, tc.level, wire.ReasoningEffort, tc.want)
		}
	}
}

func TestDiscoverYoloAutoReportsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := DiscoverYoloAuto(context.Background(), "yolo_test", srv.URL)
	if err == nil {
		t.Fatal("expected discovery error")
	}
}
