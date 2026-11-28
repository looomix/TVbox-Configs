// Package feed fetches and parses remote video source lists. It understands the
// shapes commonly used by tvbox style projects: plain `名称|API` lines, flat
// json arrays ({name,url|api}) and api_site objects ({api_site:{key:{name,api}}}).
package feed

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/looomix/tvbox-configs/internal/model"
)

const ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"

var stripSuffix = []string{"影视资源站", "资源站", "采集站", "接口站", "资源", "影视站"}

// Fetch downloads one remote list (text or json) and returns its sources.
func Fetch(rawURL string, timeout time.Duration) ([]model.Source, error) {
	if timeout <= 0 {
		timeout = 40 * time.Second
	}
	// http2 negotiation against some chinese CDNs times out, so keep it off.
	c := &http.Client{Timeout: timeout, Transport: &http.Transport{
		TLSHandshakeTimeout:   15 * time.Second,
		ForceAttemptHTTP2:     false,
		ResponseHeaderTimeout: timeout,
		// #nosec G402 -- a public source list carries no secret. Refusing an
		// otherwise readable list only because its certificate expired would
		// drop every source behind it.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Second)
		}
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "text/plain,application/json,*/*")
		resp, err := c.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("http %d", resp.StatusCode)
			continue
		}
		return Parse(body), nil
	}
	return nil, lastErr
}

// Parse detects json or line based lists and converts them to sources.
func Parse(data []byte) []model.Source {
	text := strings.TrimPrefix(string(data), "\ufeff")
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		// 必须传去 BOM、去空白后的内容：encoding/json 不跳过 UTF-8 BOM，
		// 传原始 data 会让带 BOM 的 JSON 解析失败并回退成 0 条。
		if items, ok := parseJSON([]byte(trimmed)); ok {
			return items
		}
	}
	return parseLines(text)
}

type item struct {
	ID       string `json:"id"`
	Key      string `json:"key"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	API      string `json:"api"`
	BaseURL  string `json:"baseUrl"`
	Enabled  *bool  `json:"enabled"`
	Enabled2 *bool  `json:"isEnabled"`
}

// disabled reports an explicit upstream kill switch; nil means enabled.
func (it item) disabled() bool {
	if it.Enabled != nil {
		return !*it.Enabled
	}
	if it.Enabled2 != nil {
		return !*it.Enabled2
	}
	return false
}

func (it item) api() string {
	for _, v := range []string{it.API, it.URL, it.BaseURL} {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (it item) label() string {
	for _, v := range []string{it.Name, it.Title, it.Key} {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseJSON(data []byte) ([]model.Source, bool) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, false
	}
	fromItems := func(raw []json.RawMessage) []model.Source {
		out := make([]model.Source, 0, len(raw))
		for _, r := range raw {
			var it item
			if json.Unmarshal(r, &it) != nil {
				continue
			}
			if it.disabled() {
				continue
			}
			api := it.api()
			if !usableURL(api) {
				continue
			}
			out = append(out, source(it.label(), api))
		}
		return out
	}

	switch v := root.(type) {
	case []any:
		raw := make([]json.RawMessage, 0, len(v))
		for _, e := range v {
			if b, err := json.Marshal(e); err == nil {
				raw = append(raw, b)
			}
		}
		return fromItems(raw), true
	case map[string]any:
		// api_site / apiSite groups
		for _, name := range []string{"api_site", "apiSite", "sites_map"} {
			if m, ok := v[name].(map[string]any); ok {
				out := make([]model.Source, 0, len(m))
				// 必须按 key 排序：Go 的 map 遍历顺序随机，不排序会让同一次
				// 抓取产生不同顺序的源清单，进而让去重保留的条目每次翻跳，
				// 生成文件天天变动。
				keys := make([]string, 0, len(m))
				for key := range m {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					entry, ok := m[key].(map[string]any)
					if !ok {
						continue
					}
					var it item
					if b, err := json.Marshal(entry); err == nil {
						_ = json.Unmarshal(b, &it)
					}
					if it.disabled() {
						continue
					}
					api := it.api()
					if !usableURL(api) {
						continue
					}
					label := it.label()
					if label == "" {
						label = key
					}
					out = append(out, source(label, api))
				}
				return out, true
			}
		}
		// arrays under sites / list
		for _, name := range []string{"sites", "list", "sources"} {
			if arr, ok := v[name].([]any); ok {
				raw := make([]json.RawMessage, 0, len(arr))
				for _, e := range arr {
					if b, err := json.Marshal(e); err == nil {
						raw = append(raw, b)
					}
				}
				return fromItems(raw), true
			}
		}
	}
	return nil, false
}

func parseLines(text string) []model.Source {
	out := make([]model.Source, 0, 64)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "|")
		name, api := "", ""
		if len(cols) >= 2 {
			name = strings.TrimSpace(cols[0])
			api = strings.TrimSpace(cols[1])
		} else {
			api = strings.TrimSpace(cols[0])
		}
		api = strings.TrimRight(api, "/")
		if !usableURL(api) {
			continue
		}
		out = append(out, source(name, api))
	}
	return out
}

func usableURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToLower(raw), "http") {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && u.Host != ""
}

func source(name, api string) model.Source {
	name = cleanName(name, api)
	return model.Source{
		Name:    name,
		API:     api,
		Group:   groupFor(name),
		Enabled: true,
		Origin:  "feed",
	}
}

// cleanName strips the decorative prefixes and suffixes upstream lists use.
func cleanName(name, api string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		if u, err := url.Parse(api); err == nil {
			name = strings.TrimPrefix(u.Hostname(), "www.")
		}
	}
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	name = strings.TrimSpace(strings.Trim(strings.TrimSpace(b.String()), "-_"))
	for trimmed := true; trimmed; {
		trimmed = false
		for _, s := range stripSuffix {
			if strings.HasSuffix(name, s) && len([]rune(name)) > len([]rune(s)) {
				name = strings.TrimSpace(strings.TrimSuffix(name, s))
				trimmed = true
			}
		}
	}
	if name == "" {
		name = api
	}
	return name
}

func groupFor(name string) string {
	for _, g := range []string{"动漫", "综艺", "短剧"} {
		if strings.Contains(name, g) {
			return g
		}
	}
	return "综合"
}
