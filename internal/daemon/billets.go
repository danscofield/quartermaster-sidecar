package daemon

import (
	"context"
	"fmt"

	"github.com/dscof/qm-agent/internal/identity"
	"github.com/dscof/qm-agent/qmclient"
)

func unionDiscoveredBillets(resp *qmclient.BilletDiscoveryResponse) []string {
	if resp == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var names []string
	for _, list := range [][]string{resp.Billets, resp.ImplicitBillets, resp.CedarBillets} {
		for _, name := range list {
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	return names
}

func (d *Daemon) resolveBillets(ctx context.Context, cred *identity.Credential) ([]string, error) {
	if len(d.cfg.Exchange.Billets) > 0 {
		return d.cfg.Exchange.Billets, nil
	}

	form := qmclient.BilletDiscoveryForm{}
	if cred != nil && cred.SubjectToken != "" {
		form.SubjectToken = &cred.SubjectToken
		tokenType := cred.SubjectTokenType
		if d.cfg.Exchange.SubjectTokenType != "" {
			tokenType = d.cfg.Exchange.SubjectTokenType
		}
		form.SubjectTokenType = &tokenType
	}

	resp, err := d.api.DiscoverBillets(ctx, form)
	if err != nil {
		return nil, fmt.Errorf("billet discovery: %w", err)
	}
	return unionDiscoveredBillets(resp), nil
}
