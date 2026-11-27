package gen

import (
	"github.com/looomix/tvbox-configs/internal/model"
)

// OuonnkiSite is one entry of the OuonnkiTV (LibreTV based) source list.
type OuonnkiSite struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	DetailURL string `json:"detailUrl,omitempty"`
	Timeout   int    `json:"timeout"`
	IsEnabled bool   `json:"isEnabled"`
}

// KVideoSite is one entry of the KVideo source list.
type KVideoSite struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"baseUrl"`
	Group   string `json:"group"`
	Enabled bool   `json:"enabled"`
}

// LibreSite is one entry of the LibreTV / LibreSpark custom api list.
type LibreSite struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// OuonnkiTV builds the list shape OuonnkiTV imports as 视频源.
func OuonnkiTV(sites []model.Site) []OuonnkiSite {
	used := map[string]bool{}
	out := make([]OuonnkiSite, 0, len(sites))
	for _, s := range sites {
		detail := s.API
		out = append(out, OuonnkiSite{
			ID:        keyOf(s.Key, s.Name, s.API, used, true),
			Name:      s.Name,
			URL:       s.API,
			DetailURL: detail,
			Timeout:   5000,
			IsEnabled: true,
		})
	}
	return out
}

// KVideo builds the list shape KVideo reads.
func KVideo(sites []model.Site) []KVideoSite {
	used := map[string]bool{}
	out := make([]KVideoSite, 0, len(sites))
	for _, s := range sites {
		out = append(out, KVideoSite{
			ID:      keyOf(s.Key, s.Name, s.API, used, true),
			Name:    s.Name,
			BaseURL: s.API,
			Group:   "normal",
			Enabled: true,
		})
	}
	return out
}

// LibreTV builds the custom api list LibreTV / LibreSpark accepts.
func LibreTV(sites []model.Site) []LibreSite {
	out := make([]LibreSite, 0, len(sites))
	for _, s := range sites {
		out = append(out, LibreSite{Name: s.Name, URL: s.API})
	}
	return out
}

// Artifacts maps every published file to the client it serves. MoonTV and its
// three rebrands (KatelyaTV / DecoTV / MoonTVPlus) share one schema, so they get
// one file each: the url stays stable per app while the content stays identical.
func Artifacts(sites []model.Site) map[string]any {
	moon := MoonTV(sites)
	return map[string]any{
		"tvbox.json":      TVBox(sites),
		"moontv.json":     moon,
		"katelyatv.json":  moon,
		"decotv.json":     moon,
		"moontvplus.json": moon,
		"ouonnkitv.json":  OuonnkiTV(sites),
		"kvideo.json":     KVideo(sites),
		"libretv.json":    LibreTV(sites),
	}
}

// Labels are the human readable titles of every artifact, used by the mirror
// groups and the web page.
var Labels = map[string]string{
	"tvbox.json":      "TVBox / 影视仓 单仓配置",
	"moontv.json":     "MoonTV / LunaTV 视频源",
	"katelyatv.json":  "KatelyaTV 视频源",
	"decotv.json":     "DecoTV 视频源",
	"moontvplus.json": "MoonTVPlus 视频源",
	"ouonnkitv.json":  "OuonnkiTV 视频源",
	"kvideo.json":     "KVideo 视频源",
	"libretv.json":    "LibreTV 视频源",
	"sources.txt":     "纯文本源清单（可粘贴导入）",
}
