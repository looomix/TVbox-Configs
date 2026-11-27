// Package mirror builds the public subscription urls (mirrors) of the artifacts.
package mirror

import (
	"fmt"

	"github.com/looomix/tvbox-configs/internal/model"
)

// List returns every mirror of one artifact, primary source first.
func List(owner, repo, branch, file string) []model.Mirror {
	raw := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, file)
	return []model.Mirror{
		{Name: "jsDelivr 主线路", Kind: "primary", URL: fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s/%s@%s/%s", owner, repo, branch, file)},
		{Name: "GitHub Pages 备用", Kind: "backup", URL: fmt.Sprintf("https://%s.github.io/%s/%s", owner, repo, file)},
		{Name: "Raw 直连", Kind: "backup", URL: raw},
		{Name: "Statically CDN", Kind: "backup", URL: fmt.Sprintf("https://cdn.statically.io/gh/%s/%s/%s/%s", owner, repo, branch, file)},
		{Name: "ghproxy 加速", Kind: "proxy", URL: "https://ghproxy.net/" + raw},
		{Name: "gh-proxy 加速", Kind: "proxy", URL: "https://gh-proxy.com/" + raw},
	}
}

// Index builds the multi-repository index pointing at three independent routes.
func Index(owner, repo, branch, file string) *model.Index {
	wanted := map[string]bool{"jsDelivr 主线路": true, "GitHub Pages 备用": true, "ghproxy 加速": true}
	idx := &model.Index{Name: "TVbox-Configs 订阅总入口"}
	for _, m := range List(owner, repo, branch, file) {
		if wanted[m.Name] {
			idx.Urls = append(idx.Urls, model.IndexEntry{Name: m.Name, URL: m.URL})
		}
	}
	return idx
}
