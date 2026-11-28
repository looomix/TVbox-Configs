// Package probe checks that a video CMS api endpoint answers with a real vod
// list. One failed request never decides a verdict: two keywords plus the latest
// listing are tried before a source is called dead.
package probe

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/looomix/tvbox-configs/internal/model"
)

const (
	ua      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"
	maxBody = 2 << 20
)

// keywords are tried in order; a second title avoids false negatives when the
// first one is simply missing from that site's search index.
var keywords = []string{"庆余年", "哪吒"}

// Result is the verdict of one endpoint.
type Result struct {
	State      string // ok | limited | down | off
	LatencyMs  int
	Vods       int
	Searchable bool
}

// NewClient builds one reusable http client for probing.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        128,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}
}

func join(api, query string) string {
	u := strings.TrimSpace(api)
	sep := "?"
	if strings.Contains(u, "?") {
		sep = "&"
	}
	return u + sep + query
}

func testURL(api, keyword string) string {
	return join(api, "wd="+url.QueryEscape(keyword)+"&ac=detail")
}

// listingURL asks for the site's latest vod list, which works even for apis
// whose search index is switched off.
func listingURL(api string) string {
	return join(api, "ac=detail&pg=1")
}

type payload struct {
	List []struct {
		Name string `json:"vod_name"`
		Play string `json:"vod_play_url"`
		Down string `json:"vod_down_url"`
		URL  string `json:"vod_url"`
	} `json:"list"`
}

func fetchOnce(c *http.Client, target string, deadline time.Time) Result {
	start := time.Now()
	ctx := context.Background()
	if !deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Result{State: "down"}
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	resp, err := c.Do(req)
	if err != nil {
		return Result{State: "down", LatencyMs: int(time.Since(start).Milliseconds())}
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	ms := int(time.Since(start).Milliseconds())
	if resp.StatusCode != http.StatusOK || readErr != nil {
		return Result{State: "down", LatencyMs: ms}
	}
	var data payload
	if err := json.Unmarshal(body, &data); err != nil {
		// reachable, but not a json vod api
		return Result{State: "limited", LatencyMs: ms}
	}
	if len(data.List) == 0 {
		// reachable json, but no result for this keyword
		return Result{State: "limited", LatencyMs: ms}
	}
	named, playable := 0, 0
	for _, item := range data.List {
		if strings.TrimSpace(item.Name) != "" {
			named++
		}
		links := item.Play + item.Down + item.URL
		if strings.Contains(strings.ToLower(links), "http") {
			playable++
		}
	}
	if named == 0 || playable == 0 {
		return Result{State: "limited", LatencyMs: ms, Vods: len(data.List)}
	}
	return Result{State: "ok", LatencyMs: ms, Vods: len(data.List), Searchable: true}
}

// Verify decides the state of one endpoint: search first, latest listing last.
// A non-zero deadline stops the remaining requests, so a loose -budget is never
// overshot by more than a single request.
func Verify(c *http.Client, api string, deadline time.Time) Result {
	if strings.TrimSpace(api) == "" {
		return Result{State: "down"}
	}
	start := time.Now()
	expired := func() bool { return !deadline.IsZero() && time.Now().After(deadline) }
	var best *Result
	for _, kw := range keywords {
		if expired() {
			break
		}
		r := fetchOnce(c, testURL(api, kw), deadline)
		if r.State == "ok" {
			r.LatencyMs = int(time.Since(start).Milliseconds())
			return r
		}
		if best == nil || rank(r.State) > rank(best.State) {
			cp := r
			best = &cp
		}
	}
	if !expired() {
		if r := fetchOnce(c, listingURL(api), deadline); r.State == "ok" {
			r.Searchable = false
			r.LatencyMs = int(time.Since(start).Milliseconds())
			return r
		}
	}
	out := Result{State: "down"}
	if best != nil {
		out = *best
	}
	out.LatencyMs = int(time.Since(start).Milliseconds())
	return out
}

func rank(state string) int {
	switch state {
	case "ok":
		return 3
	case "limited":
		return 2
	default:
		return 1
	}
}

// Run probes every source in parallel. Sources still unprobed when the deadline
// passes are reported as limited, so one slow site can never stall a run.
func Run(sources []model.Source, workers int, timeout time.Duration, deadline time.Time) []Result {
	if workers < 1 {
		workers = 1
	}
	results := make([]Result, len(sources))
	idx := make(chan int)
	wg := sync.WaitGroup{}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := NewClient(timeout)
			for j := range idx {
				if !sources[j].Enabled || strings.TrimSpace(sources[j].API) == "" {
					results[j] = Result{State: "off"}
					continue
				}
				if !deadline.IsZero() && time.Now().After(deadline) {
					results[j] = Result{State: "limited"}
					continue
				}
				r := Verify(c, sources[j].API, deadline)
				if r.State != "ok" && !deadline.IsZero() && time.Now().After(deadline) {
					// cut short by the budget: report unverified instead of dead
					r.State = "limited"
				}
				results[j] = r
			}
		}()
	}
	for i := range sources {
		idx <- i
	}
	close(idx)
	wg.Wait()
	return results
}
