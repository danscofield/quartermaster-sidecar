package daemon

import (
	"testing"

	"github.com/dscof/qm-agent/qmclient"
)

func TestUnionDiscoveredBillets(t *testing.T) {
	names := unionDiscoveredBillets(&qmclient.BilletDiscoveryResponse{
		Billets:         []string{"a", "b"},
		ImplicitBillets: []string{"b", "c"},
		CedarBillets:    []string{"c", "d"},
	})
	if len(names) != 4 {
		t.Fatalf("got %d names: %v", len(names), names)
	}
}

func TestSanitizeBilletName(t *testing.T) {
	if got := sanitizeBilletName("pay/ments"); got != "pay_ments" {
		t.Fatalf("got %q", got)
	}
}
