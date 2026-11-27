package gen

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode"

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

// NameKey normalizes a display name so mirrors of the same station collapse:
// the trailing "(host)" hint is dropped, decoration and punctuation are ignored
// and the usual station suffixes are trimmed.
func NameKey(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.LastIndex(name, "("); i > 0 && strings.HasSuffix(name, ")") {
		name = strings.TrimSpace(name[:i])
	}
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	key := b.String()
	for _, suffix := range []string{"影视资源站", "资源站", "采集站", "接口站", "影视站", "资源", "影视"} {
		if strings.HasSuffix(key, suffix) && len([]rune(key)) > len([]rune(suffix))+1 {
			key = strings.TrimSuffix(key, suffix)
		}
	}
	return key
}

// DedupeNames merges entries that show the same station name even when their
// api hosts differ, which is how most mirror domains are published. The
// survivor is chosen in this order: already verified ok in the previous report,
// manual entry, shorter api url, then lexical id for stable output.
func DedupeNames(list []model.Source, preferOK map[string]bool) ([]model.Source, int) {
	groups := make(map[string][]model.Source, len(list))
	order := make([]string, 0, len(list))
	for _, s := range list {
		key := NameKey(s.Name)
		if key == "" {
			key = "api:" + NormalizeAPI(s.API)
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], s)
	}

	removed := 0
	out := make([]model.Source, 0, len(list))
	for _, key := range order {
		items := groups[key]
		if len(items) > 1 {
			sort.SliceStable(items, func(i, j int) bool {
				a, b := items[i], items[j]
				aOK := preferOK[NormalizeAPI(a.API)]
				bOK := preferOK[NormalizeAPI(b.API)]
				if aOK != bOK {
					return aOK
				}
				aManual := a.Origin != "feed"
				bManual := b.Origin != "feed"
				if aManual != bManual {
					return aManual
				}
				if len(a.API) != len(b.API) {
					return len(a.API) < len(b.API)
				}
				return a.API < b.API
			})
			removed += len(items) - 1
		}
		survivor := items[0]
		survivor.Name = strings.TrimSpace(survivor.Name)
		if i := strings.LastIndex(survivor.Name, "("); i > 0 && strings.HasSuffix(survivor.Name, ")") {
			survivor.Name = strings.TrimSpace(survivor.Name[:i])
		}
		out = append(out, survivor)
	}
	return out, removed
}
