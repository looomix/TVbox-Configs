// Package probe verifies that a video CMS api endpoint is alive, searchable and
// actually returns playable episodes. The judgement is deliberately strict:
// one failed request is never enough to declare a source dead, and a source is
// only reported ok when a real vod list with play urls comes back.
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
	ua      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"
	maxBody = 4 << 20
)

// keywords are tried in order; the second keyword avoids false negatives when
// the first title is simply missing from that site's search index.
var keywords = []string{"庆余年", "哪吒"}

type Result struct {
	State      string // ok | limited | down | off
	LatencyMs  int
	Vods       int
	Searchable bool
	Playable   bool
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
	} `json:"list"`
}

func fetchOnce(c *http.Client, target string) Result {
	start := time.Now()
	resp, err := c.Get(target)
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
		// Reachable, but not a json vod api.
		return Result{State: "limited", LatencyMs: ms}
	}
	if len(data.List) == 0 {
		// Reachable json, but no result for this keyword.
		return Result{State: "limited", LatencyMs: ms}
	}
	named, playable := 0, 0
	for _, item := range data.List {
		if strings.TrimSpace(item.Name) != "" {
			named++
		}
		if strings.Contains(strings.ToLower(item.Play), "http") {
			playable++
		}
	}
	if named == 0 || playable == 0 {
		// Names without play urls: playable content cannot be confirmed.
		return Result{State: "limited", LatencyMs: ms, Vods: len(data.List)}
	}
	return Result{State: "ok", LatencyMs: ms, Vods: len(data.List), Searchable: true, Playable: true}
}

// Verify probes one source up to three times (second keyword included) and only
// returns down when every attempt failed, so a short network hiccup cannot mark
// a healthy source as dead.
func Verify(c *http.Client, api string) Result {
	if strings.TrimSpace(api) == "" {
		return Result{State: "down"}
	}
	start := time.Now()
	sequence := []string{keywords[0], keywords[0], keywords[1]}
	best := Result{State: "down"}
	for i, kw := range sequence {
		if i > 0 {
			time.Sleep(time.Duration(i) * time.Second)
		}
		r := fetchOnce(c, testURL(api, kw))
		if r.State == "ok" {
			return r
		}
		if r.State == "limited" || best.State != "limited" {
			best = r
		}
	}
	if best.State != "ok" {
		// search index may be off or the search endpoint may be blocked; the
		// latest list still proves the site serves playable content.
		if r := fetchOnce(c, listingURL(api)); r.State == "ok" {
			r.Searchable = false
			r.LatencyMs = int(time.Since(start).Milliseconds())
			return r
		}
	}
	if best.State == "" {
		best.State = "down"
	}
	best.LatencyMs = int(time.Since(start).Milliseconds())
	return best
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
				results[j] = Verify(c, sources[j].API)
			}
		}()
	}
	for i := range sources {
		idx <- i
	}
	close(idx)
	wg.Wait()

	// second chance: sources that just failed get one more round with a fresh
	// client and a longer timeout, so a transient outage is never reported dead.
	retry := make([]int, 0, len(sources))
	for i := range results {
		if results[i].State == "down" {
			retry = append(retry, i)
		}
	}
	if len(retry) > 0 {
		slowWorkers := workers / 2
		if slowWorkers < 2 {
			slowWorkers = 2
		}
		jobs := make(chan int)
		wg2 := sync.WaitGroup{}
		for i := 0; i < slowWorkers; i++ {
			wg2.Add(1)
			go func() {
				defer wg2.Done()
				c := NewClient(timeout * 2)
				for j := range jobs {
					if r := Verify(c, sources[j].API); r.State != "down" {
						results[j] = r
					}
				}
			}()
		}
		for _, j := range retry {
			jobs <- j
		}
		close(jobs)
		wg2.Wait()
	}
	return results
}
