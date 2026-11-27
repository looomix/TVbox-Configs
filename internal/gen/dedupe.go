package gen

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/looomix/tvbox-configs/internal/model"
)

// pathNoise are the segments that only carry request parameters of one and the
// same endpoint (/api.php/provide/vod, /api.php/provide/vod/at/xml, …).
var pathNoise = map[string]bool{
	"at": true, "from": true, "xml": true, "json": true, "ac": true, "wd": true,
	"pg": true, "p": true, "t": true, "h": true, "ids": true, "page": true,
}

// PathKey keeps the first three meaningful path segments, so two stations served
// from one host stay apart while param variants of one endpoint still collapse.
func PathKey(rawPath string) string {
	path := strings.ToLower(strings.Trim(strings.TrimSpace(rawPath), "/"))
	if path == "" {
		return ""
	}
	kept := make([]string, 0, 3)
	for _, seg := range strings.Split(path, "/") {
		if seg == "" || pathNoise[seg] {
			continue
		}
		kept = append(kept, seg)
		if len(kept) == 3 {
			break
		}
	}
	return strings.Join(kept, "/")
}

// NormalizeAPI builds the dedupe key: scheme, www, port, case and trailing
// slashes are ignored; host and path prefix decide which station it is.
func NormalizeAPI(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return strings.ToLower(strings.TrimSpace(raw))
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	// proxy style endpoints (…?url=…) may serve different stations on one host
	if target := u.Query().Get("url"); target != "" {
		if t, err := url.Parse(strings.TrimSpace(target)); err == nil && t.Host != "" {
			key := host + "|" + strings.TrimPrefix(strings.ToLower(t.Hostname()), "www.")
			if path := PathKey(t.Path); path != "" {
				key += "/" + path
			}
			return key
		}
	}
	if path := PathKey(u.Path); path != "" {
		return host + "/" + path
	}
	return host
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
