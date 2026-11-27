package gen

import (
	"fmt"
	"strings"

	"github.com/looomix/tvbox-configs/internal/model"
)

// MoonTV builds the MoonTV / LunaTV config.json shape, so its 视频源配置 page can
// import or paste this file directly.
func MoonTV(sites []model.Site) *model.MoonConfig {
	used := map[string]bool{}
	apiSite := make(map[string]model.MoonSite, len(sites))
	for _, s := range sites {
		key := keyOf(s.Key, s.Name, s.API, used, true)
		apiSite[key] = model.MoonSite{Name: s.Name, API: s.API}
	}
	return &model.MoonConfig{CacheTime: 7200, APISite: apiSite}
}

// TXT builds the `名称|API|类型|状态` plain text list, importable by MoonTV,
// TVBox tooling and anything else that eats simple line lists.
func TXT(sites []model.Site) string {
	var b strings.Builder
	b.WriteString("# TVbox-Configs 已验证视频源\n")
	b.WriteString(fmt.Sprintf("# 共 %d 条，每行：名称|API|类型|状态\n", len(sites)))
	for _, s := range sites {
		b.WriteString(fmt.Sprintf("%s|%s|json|verified\n", s.Name, s.API))
	}
	return b.String()
}
