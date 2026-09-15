package modes

import (
	"context"
	"time"

	"github.com/patriceckhart/zot/packages/provider"
)

func (i *Interactive) saveLMStudioLogin(baseURL, apiKey string) {
	if i.cfg.AuthManager == nil || i.cfg.AuthManager.Store() == nil {
		i.dialog.ShowResult(false, "auth store is unavailable")
		return
	}
	if err := i.cfg.AuthManager.Store().SetEndpointCredential(provider.LMStudioProviderID, baseURL, apiKey); err != nil {
		i.dialog.ShowResult(false, err.Error())
		return
	}
	provider.SetManagedModelsForProvider(provider.LMStudioProviderID, nil)
	i.dialog.ShowResult(true, "")
	if i.cfg.RefreshLMStudioModels != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			// Connection errors are reported when /model is opened. Saving
			// the endpoint does not require the server to be online.
			_ = i.cfg.RefreshLMStudioModels(ctx)
		}()
	}
}
