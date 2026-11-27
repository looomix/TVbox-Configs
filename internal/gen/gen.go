// Package gen turns probe results into the published json artifacts.
package gen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/looomix/tvbox-configs/internal/model"
)

// Groups allowed in the generated config. Anything else falls back to 综合.
var allowedGroups = map[string]bool{
	"综合": true, "影视": true, "动漫": true, "综艺": true, "短剧": true, "其他": true,
}

var blockedWords = []string{"nsfw", "adult", "porn", "18+", "成人", "福利", "里番", "色情"}

// Blocked reports whether a source must never reach the published config.
func Blocked(s model.Source) bool {
	hay := strings.ToLower(s.Group + " " + s.Name + " " + s.ID + " " + s.API)
	for _, w := range blockedWords {
		if strings.Contains(hay, strings.ToLower(w)) {
			return true
		}
	}
	return false
}

func groupOf(g string) string {
	g = strings.TrimSpace(g)
	if allowedGroups[g] {
		return g
	}
	return "综合"
}

func keyOf(id, name string, used map[string]bool) string {
	base := strings.ToLower(strings.TrimSpace(id))
	if base == "" {
		base = strings.ToLower(strings.TrimSpace(name))
	}
	var b strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	k := b.String()
	k = strings.Trim(k, "-")
	if k == "" {
		k = "cms"
	}
	for i := 2; used[k]; i++ {
		k = fmt.Sprintf("%s-%d", strings.TrimPrefix(k, "-"), i)
	}
	used[k] = true
	return k
}

// TVBox builds the single-repository config from a health report.
func TVBox(rep *model.Report) *model.TVBox {
	cfg := &model.TVBox{
		Sites:   []model.Site{},
		Lives:   []any{},
		Parses:  []any{},
		Version: "1.0",
		Flags:   []string{"youku", "qq", "iqiyi", "qiyi", "letv", "sohu", "tudou", "pptv", "mgtv", "wasu", "bilibili"},
	}
	list := append([]model.Status{}, rep.Sources...)
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.State != b.State {
			return a.State == "ok"
		}
		if a.LatencyMs != b.LatencyMs {
			return a.LatencyMs < b.LatencyMs
		}
		return a.Name < b.Name
	})
	used := map[string]bool{}
	for _, s := range list {
		if !s.Enabled || (s.State != "ok" && s.State != "limited") {
			continue
		}
		cfg.Sites = append(cfg.Sites, model.Site{
			Key:         keyOf(s.ID, s.Name, used),
			Name:        s.Name,
			Type:        3,
			API:         s.API,
			Searchable:  1,
			QuickSearch: 1,
			Filterable:  1,
			Ext:         "",
			Group:       groupOf(s.Group),
		})
	}
	return cfg
}

// WriteJSON writes v with indentation through a temp file so runs never leave
// half written artifacts behind.
func WriteJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
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
