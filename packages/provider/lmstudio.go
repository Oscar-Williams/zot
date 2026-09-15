package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const LMStudioProviderID = "lmstudio"

// NormalizeLMStudioURL accepts a server root or either API base URL.
func NormalizeLMStudioURL(value string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("LM Studio server URL must be an http or https URL without credentials, query, or fragment")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(u.Path, "/api/v1") {
		u.Path = strings.TrimSuffix(u.Path, "/api/v1")
	} else {
		u.Path = strings.TrimSuffix(u.Path, "/v1")
	}
	u.RawPath = ""
	return strings.TrimRight(u.String(), "/"), nil
}

// DiscoverLMStudio lists models without loading them. Only older servers that
// lack the native endpoint fall back to the less descriptive OpenAI endpoint.
func DiscoverLMStudio(ctx context.Context, serverURL, apiKey string) ([]Model, error) {
	root, err := NormalizeLMStudioURL(serverURL)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	get := func(path string, out any) (int, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, root+path, nil)
		if err != nil {
			return 0, err
		}
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}
			return 0, fmt.Errorf("LM Studio discovery request failed")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return resp.StatusCode, fmt.Errorf("LM Studio discovery HTTP %d", resp.StatusCode)
		}
		if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 8<<20)).Decode(out); err != nil {
			return resp.StatusCode, fmt.Errorf("LM Studio discovery returned invalid model data")
		}
		return resp.StatusCode, nil
	}
	var native struct {
		Models []struct {
			Type    string `json:"type"`
			Key     string `json:"key"`
			Name    string `json:"display_name"`
			Context int    `json:"max_context_length"`
			Loaded  []struct {
				ID     string `json:"id"`
				Config struct {
					Context int `json:"context_length"`
				} `json:"config"`
			} `json:"loaded_instances"`
		} `json:"models"`
	}
	status, err := get("/api/v1/models", &native)
	var models []Model
	seen := map[string]bool{}
	add := func(id, name string, contextWindow int) {
		if strings.TrimSpace(id) == "" || seen[id] {
			return
		}
		seen[id] = true
		if name == "" {
			name = id
		}
		if contextWindow <= 0 {
			contextWindow = 32768
		}
		maxOutput := min(4096, max(1, contextWindow/4))
		models = append(models, Model{Provider: LMStudioProviderID, ID: id, DisplayName: name, ContextWindow: contextWindow, MaxOutput: maxOutput, BaseURL: root + "/v1", Source: "live"})
	}
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed || status == http.StatusNotImplemented {
		var fallback struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if _, err := get("/v1/models", &fallback); err != nil {
			return nil, err
		}
		if fallback.Data == nil {
			return nil, fmt.Errorf("LM Studio discovery response is missing data")
		}
		for _, m := range fallback.Data {
			if strings.Contains(strings.ToLower(m.ID), "embed") {
				continue
			}
			add(m.ID, m.ID, 0)
		}
		return models, nil
	}
	if err != nil {
		return nil, err
	}
	if native.Models == nil {
		return nil, fmt.Errorf("LM Studio discovery response is missing models")
	}
	for _, m := range native.Models {
		if m.Type != "llm" {
			continue
		}
		if len(m.Loaded) == 0 {
			add(m.Key, m.Name, m.Context)
			continue
		}
		for _, instance := range m.Loaded {
			id := instance.ID
			if id == "" {
				id = m.Key
			}
			// Unknown loaded context uses a conservative default, not the
			// model's theoretical training limit.
			add(id, m.Name, instance.Config.Context)
		}
	}
	return models, nil
}
