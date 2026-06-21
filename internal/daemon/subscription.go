package daemon

import (
	"fmt"
	"sort"
	"strings"
)

func subscriptionBasePath(id string) string {
	return "/subscriptions/" + id
}

func normalizeBillets(names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("at least one billet is required")
	}

	seen := make(map[string]struct{}, len(names))
	var out []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one billet is required")
	}
	sort.Strings(out)
	return out, nil
}

func subscriptionID(billets []string) string {
	parts := make([]string, len(billets))
	for i, b := range billets {
		parts[i] = sanitizeBilletName(b)
	}
	return strings.Join(parts, "__")
}

func billetsAvailable(billets []string, available map[string]struct{}) (bool, []string) {
	var missing []string
	for _, b := range billets {
		if _, ok := available[b]; !ok {
			missing = append(missing, b)
		}
	}
	return len(missing) == 0, missing
}

func availableSet(names []string) map[string]struct{} {
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[n] = struct{}{}
	}
	return set
}
