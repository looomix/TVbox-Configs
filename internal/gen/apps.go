package gen

import (
	"net/url"
	"strings"

	"github.com/looomix/tvbox-configs/internal/model"
)

// ClientSource is one superset entry accepted by OuonnkiTV, KVideo and LibreTV.
// Each client reads the fields it knows and ignores the rest, so all three can
// subscribe to the same file.
type ClientSource struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	BaseURL   string `json:"baseUrl"`
	DetailURL string `json:"detailUrl"`
	Timeout   int    `json:"timeout"`
	Group     string `json:"group"`
	Enabled   bool   `json:"enabled"`
	IsEnabled bool   `json:"isEnabled"`
}

// siteRoot keeps only scheme + host of an API url. LibreTV / OuonnkiTV use the
// detail field as the site root for scraping detail pages when the list API
// does not return play urls; pointing it at the API root is wrong.
func siteRoot(api string) string {
	u, err := url.Parse(api)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// ClientList builds the shared list for the three JSON-array clients.
func ClientList(sites []model.Site) []ClientSource {
	used := map[string]bool{}
	out := make([]ClientSource, 0, len(sites))
	for _, s := range sites {
		api := strings.TrimRight(s.API, "/")
		out = append(out, ClientSource{
			ID:        keyOf(s.Key, s.Name, s.API, used, true),
			Name:      s.Name,
			URL:       api,
			BaseURL:   api,
			DetailURL: siteRoot(api),
			Timeout:   5000,
			Group:     "normal",
			Enabled:   true,
			IsEnabled: true,
		})
	}
	return out
}

// Artifacts maps each published config file to its content. MoonTV and its
// rebrands share one schema, and the three array clients share one list.
func Artifacts(sites []model.Site) map[string]any {
	return map[string]any{
		"tvbox.json":  TVBox(sites),
		"moontv.json": MoonTV(sites),
		"list.json":   ClientList(sites),
	}
}

// Labels are the human readable titles used by the web page and mirror.json.
var Labels = map[string]string{
	"tvbox.json":  "TVBox / 影视仓 单仓配置（type 1）",
	"moontv.json": "MoonTV / LunaTV / KatelyaTV / DecoTV / MoonTVPlus 视频源",
	"list.json":   "OuonnkiTV / KVideo / LibreTV 通用源清单",
	"sources.txt": "纯文本源清单（可粘贴导入）",
	"tvbox.txt":   "TVBox / 影视仓 多仓 TXT 入口",
}
