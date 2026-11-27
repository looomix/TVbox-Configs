// Package probe checks whether a video CMS api endpoint is alive and searchable.
package probe

import (
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
	// testKeyword is used to verify that the api can return playable list data.
	testKeyword = "庆余年"
	maxBody     = 4 << 20
	ua          = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"
)

type Result struct {
	State     string
	LatencyMs int
	Vods      int
}

// NewClient builds one reusable http client for probing.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        128,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     60 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

func testURL(api string) string {
	u := strings.TrimSpace(api)
	if u == "" {
		return ""
	}
	sep := "?"
	if strings.Contains(u, "?") {
		sep = "&"
	}
	return u + sep + "wd=" + url.QueryEscape(testKeyword) + "&ac=detail"
}

func once(c *http.Client, api string) Result {
	start := time.Now()
	resp, err := c.Get(testURL(api))
	if err != nil {
		return Result{State: "down", LatencyMs: int(time.Since(start).Milliseconds())}
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	ms := int(time.Since(start).Milliseconds())
	if resp.StatusCode != http.StatusOK || readErr != nil {
		return Result{State: "down", LatencyMs: ms}
	}
	var payload struct {
		List []struct {
			Name string `json:"vod_name"`
		} `json:"list"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		// Reachable but not a standard json cms payload.
		return Result{State: "limited", LatencyMs: ms}
	}
	if len(payload.List) == 0 {
		// Reachable json but no search result for the keyword.
		return Result{State: "limited", LatencyMs: ms}
	}
	return Result{State: "ok", LatencyMs: ms, Vods: len(payload.List)}
}

// Test probes one source, retrying once when the first attempt fails hard.
func Test(c *http.Client, api string) Result {
	r := once(c, api)
	if r.State == "down" {
		time.Sleep(300 * time.Millisecond)
		r2 := once(c, api)
		if r2.State != "down" {
			return r2
		}
		if r2.LatencyMs > r.LatencyMs {
			r.LatencyMs = r2.LatencyMs
		}
	}
	return r
}

// Run probes all sources with a bounded worker pool and keeps the input order.
func Run(sources []model.Source, workers int, timeout time.Duration) []Result {
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
				results[j] = Test(c, sources[j].API)
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
