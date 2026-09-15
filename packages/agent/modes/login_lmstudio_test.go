package modes

import (
	"testing"

	"github.com/patriceckhart/zot/packages/tui"
)

func TestLoginLMStudioRequiresServerURL(t *testing.T) {
	d := newLoginDialog()
	d.step = loginStepProvider
	d.method = "apikey"
	d.status = map[string]string{}
	d.providerQuery = "lmstudio"
	d.HandleKey(tui.Key{Kind: tui.KeyEnter})
	if d.step != loginStepLlamaURL || d.provider != "lmstudio" {
		t.Fatalf("wrong login step: %v", d.step)
	}
	d.HandleKey(tui.Key{Kind: tui.KeyEnter})
	if d.step != loginStepLlamaURL {
		t.Fatal("empty URL enabled discovery")
	}
	d.llamaEd.SetValue("http://localhost:1234/api/v1")
	d.HandleKey(tui.Key{Kind: tui.KeyEnter})
	if d.step != loginStepLlamaKey {
		t.Fatalf("URL rejected: %s", d.message)
	}
	a := d.HandleKey(tui.Key{Kind: tui.KeyEnter})
	if !a.SaveLlama || a.Provider != "lmstudio" || a.LlamaURL != "http://localhost:1234" || a.LlamaAPIKey != "" {
		t.Fatalf("action=%+v", a)
	}
}
