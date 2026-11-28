// Package mirror builds the public subscription urls of each artifact.
// ghproxy is the primary route; jsDelivr is intentionally not used.
package mirror

import (
	"fmt"

	"github.com/looomix/tvbox-configs/internal/model"
)

// List returns the available routes, preferred one first.
func List(owner, repo, branch, file string) []model.Mirror {
	raw := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, file)
	ms := []model.Mirror{
		{Name: "ghproxy 加速", Kind: "primary", URL: "https://ghproxy.net/" + raw},
		{Name: "gh-proxy 加速", Kind: "backup", URL: "https://gh-proxy.com/" + raw},
		{Name: "Raw 直连", Kind: "backup", URL: raw},
		{Name: "GitHub Pages 备用", Kind: "backup", URL: fmt.Sprintf("https://%s.github.io/%s/%s", owner, repo, file)},
	}
	return ms
}

// Group wraps every route of one artifact with its label.
func Group(owner, repo, branch, file, label, kind string) model.MirrorGroup {
	ms := List(owner, repo, branch, file)
	return model.MirrorGroup{
		File:    file,
		Label:   label,
		Kind:    kind,
		Primary: ms[0].URL,
		Mirrors: ms,
	}
}

// Index lists three independent entry points for the TVBox config.
func Index(owner, repo, branch, file string) *model.Index {
	ms := List(owner, repo, branch, file)
	pick := func(i int) model.IndexEntry {
		return model.IndexEntry{Name: ms[i].Name, URL: ms[i].URL}
	}
	return &model.Index{
		Name: "TVbox-Configs 订阅总入口",
		Urls: []model.IndexEntry{pick(0), pick(2), pick(3)},
	}
}
