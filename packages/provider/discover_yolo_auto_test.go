package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
