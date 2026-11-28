// Package gen turns the source list and probe results into the published
// artifacts (TVBox config, MoonTV config, plain text list, reports).
package gen

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/looomix/tvbox-configs/internal/model"
)

// allowedGroups are the group labels written into the generated config.
var allowedGroups = map[string]bool{
	"综合": true, "影视": true, "动漫": true, "综艺": true, "短剧": true, "其他": true,
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func groupOf(g string) string {
	g = strings.TrimSpace(g)
	if allowedGroups[g] {
		return g
	}
	return "综合"
}

// HostOf returns the lowercased hostname without a leading www.
func HostOf(api string) string {
	u, err := url.Parse(strings.TrimSpace(api))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// IDFor builds a stable id from the api url, so history survives re-runs.
func IDFor(api string) string {
	host := HostOf(api)
	var b strings.Builder
	for _, r := range strings.ToLower(host) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "source"
	}
	return fmt.Sprintf("%s-%04x", slug, crc32.ChecksumIEEE([]byte(NormalizeAPI(api)))%0x10000)
}

func keyOf(id, name, api string, used map[string]bool, strict bool) string {
	base := strings.ToLower(strings.TrimSpace(id))
	if base == "" {
		base = strings.ToLower(strings.TrimSpace(name))
	}
	if base == "" {
		base = HostOf(api)
	}
	var b strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (!strict && r == '-') {
			b.WriteRune(r)
		}
	}
	k := strings.Trim(b.String(), "-")
	if k == "" {
		k = "cms"
	}
	keyBase := k
	for i := 2; used[k]; i++ {
		k = fmt.Sprintf("%s%d", keyBase, i)
	}
	used[k] = true
	return k
}

// SitesOf builds the site list shared by every published format: only sources
// that were positively verified (state ok) are included.
func SitesOf(rep *model.Report) []model.Site {
	list := append([]model.Status{}, rep.Sources...)
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].State != list[j].State {
			return list[i].State == "ok"
		}
		if list[i].LatencyMs != list[j].LatencyMs {
			return list[i].LatencyMs < list[j].LatencyMs
		}
		return list[i].Name < list[j].Name
	})
	sites := make([]model.Site, 0, len(list))
	used := map[string]bool{}
	for _, s := range list {
		if s.Origin == "" {
			s.Origin = "manual"
		}
		if !s.Enabled || s.State != "ok" || StatusNSFW(s) {
			continue
		}
		sites = append(sites, model.Site{
			Key: keyOf(s.ID, s.Name, s.API, used, false),
			Name: s.Name,
			// All sources here are Apple CMS provide/vod JSON endpoints.
			// In TVBox, type 1 = CMS JSON API; type 3 = Spider plugin (needs a
			// csp_* class + jar). Using 3 makes the app load these as plugins
			// and show empty lists, so it must be 1.
			Type:        1,
			API:         s.API,
			Searchable:  boolInt(s.Searchable),
			QuickSearch: boolInt(s.Searchable),
			Filterable:  1,
			Ext:         "",
			Group:       groupOf(s.Group),
		})
	}
	return sites
}

// TVBox builds the single-repository config consumable by TVBox / 影视仓.
func TVBox(sites []model.Site) *model.TVBox {
	return &model.TVBox{
		Sites:   sites,
		Lives:   []any{},
		Parses:  []any{},
		Flags:   []string{"youku", "qq", "iqiyi", "qiyi", "letv", "sohu", "tudou", "pptv", "mgtv", "wasu", "bilibili"},
		Version: "1.0",
	}
}

// AppendCheck appends one check point and keeps only the newest limit points.
func AppendCheck(prev []model.Check, ok bool, ms int, now time.Time, limit int) []model.Check {
	out := append(append([]model.Check{}, prev...), model.Check{
		T:  now.Format("2006-01-02T15:04:05Z07:00"),
		OK: ok,
		Ms: ms,
	})
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// WriteJSON writes v with indentation through a temp file so runs never leave
// half written artifacts behind.
func WriteJSON(path string, v any) error {
	data, err := encode(v)
	if err != nil {
		return err
	}
	return writeFile(path, data)
}

// WriteJSONIfChanged writes v only when the bytes differ, and reports a change.
func WriteJSONIfChanged(path string, v any) (bool, error) {
	data, err := encode(v)
	if err != nil {
		return false, err
	}
	if old, err := os.ReadFile(path); err == nil && string(old) == string(data) {
		return false, nil
	}
	return true, writeFile(path, data)
}

// WriteText writes a plain text artifact.
func WriteText(path, text string) error {
	return writeFile(path, []byte(text))
}

func encode(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
