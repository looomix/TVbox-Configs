package gen

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/looomix/tvbox-configs/internal/model"
)

// NormalizeAPI builds the dedupe key: scheme, www, trailing slashes, default
// ports and letter case are ignored, the path itself is kept.
func NormalizeAPI(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return strings.ToLower(strings.TrimSpace(raw))
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	// proxy style endpoints (…?url=…) may serve different stations on one host
	key := host
	if target := u.Query().Get("url"); target != "" {
		if t, err := url.Parse(strings.TrimSpace(target)); err == nil && t.Host != "" {
			key = host + "|" + strings.TrimPrefix(strings.ToLower(t.Hostname()), "www.")
			if path := strings.Trim(t.Path, "/"); path != "" {
				key += "/" + strings.ToLower(path)
			}
		}
	}
	return key
}

// Dedupe removes repeated api endpoints, keeps the first occurrence (manual
// entries are ordered before scraped ones) and makes ids and names unique.
func Dedupe(list []model.Source) ([]model.Source, int) {
	seen := make(map[string]bool, len(list))
	names := make(map[string]int, len(list))
	ids := make(map[string]bool, len(list))
	out := make([]model.Source, 0, len(list))
	removed := 0

	for _, s := range list {
		key := NormalizeAPI(s.API)
		if key == "" || seen[key] {
			removed++
			continue
		}
		seen[key] = true

		name := strings.TrimSpace(s.Name)
		if name == "" {
			name = HostOf(s.API)
		}
		lower := strings.ToLower(name)
		if names[lower] > 0 {
			names[lower]++
			name = fmt.Sprintf("%s (%s)", name, HostOf(s.API))
		} else {
			names[lower] = 1
		}

		id := strings.TrimSpace(s.ID)
		if id == "" {
			id = IDFor(s.API)
		}
		base := id
		for i := 2; ids[id]; i++ {
			id = fmt.Sprintf("%s-%d", base, i)
		}
		ids[id] = true

		s.ID = id
		s.Name = name
		if s.Group == "" {
			s.Group = "综合"
		}
		out = append(out, s)
	}
	return out, removed
}
