package daemon

import (
	"regexp"
	"strings"
)

var unsafeBilletName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizeBilletName(name string) string {
	s := strings.TrimSpace(name)
	if s == "" {
		return "_"
	}
	return unsafeBilletName.ReplaceAllString(s, "_")
}

func billetBasePath(billetName string) string {
	return "/billets/" + sanitizeBilletName(billetName)
}
