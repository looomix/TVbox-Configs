package probe

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/looomix/tvbox-configs/internal/model"
)

const okJSON = `{"list":[{"vod_name":"测试影片","vod_play_url":"第1集$https://cdn.example.com/1.m3u8"}]}`
const okXML = `<?xml version="1.0" encoding="utf-8"?><rss version="5.1"><list page="1"><video><name>测试影片</name><play_url>https://cdn.example.com/1.m3u8</play_url></video></list></rss>`

func serve(t *testing.T, h http.HandlerFunc) (*httptest.Server, *http.Client) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := NewClient(3 * time.Second)
	return srv, c
}

func api(srv *httptest.Server) string { return srv.URL + "/api.php/provide/vod" }

// A healthy JSON endpoint must verify, even when it ignores the query shape.
func TestVerifyJSONOK(t *testing.T) {
	srv, c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, okJSON)
	})
	r := Verify(c, api(srv), time.Time{})
	if r.State != "ok" || r.Vods != 1 || r.Reason != "" {
		t.Fatalf("want ok/1/empty, got %+v", r)
	}
	if r.LatencyMs <= 0 {
		t.Fatalf("ok result must carry a latency, got %d", r.LatencyMs)
	}
	if !r.Searchable {
		t.Fatalf("a keyword query that returned data means the site is searchable, got %+v", r)
	}
}

// A station that rejects the keyword query is usable but not searchable, and
// the published flag must say so instead of claiming every site supports search.
func TestVerifyListingOnlyIsNotSearchable(t *testing.T) {
	srv, c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("wd") != "" {
			fmt.Fprint(w, "<html>search disabled</html>")
			return
		}
		fmt.Fprint(w, okJSON)
	})
	r := Verify(c, api(srv), time.Time{})
	if r.State != "ok" || r.Searchable {
		t.Fatalf("want ok and not searchable, got %+v", r)
	}
}

// Many Apple CMS deployments answer with XML unless at=json is requested; that
// must count as alive instead of "not an api".
func TestVerifyXMLOK(t *testing.T) {
	srv, c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, okXML)
	})
	r := Verify(c, api(srv), time.Time{})
	if r.State != "ok" || r.Vods != 1 {
		t.Fatalf("xml list must verify, got %+v", r)
	}
}

// A search-only endpoint: listing returns junk, ?wd= returns data.
func TestVerifySearchOnly(t *testing.T) {
	srv, c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("wd") != "" {
			fmt.Fprint(w, okJSON)
			return
		}
		fmt.Fprint(w, "<html><body>error</body></html>")
	})
	r := Verify(c, api(srv), time.Time{})
	if r.State != "ok" {
		t.Fatalf("search-only endpoint must verify, got %+v", r)
	}
	if !r.Searchable {
		t.Fatalf("searchable flag must be set, got %+v", r)
	}
}

// An HTML page is not a vod list: the host is alive, usability is unproven.
func TestVerifyHTMLIsLimited(t *testing.T) {
	srv, c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<!doctype html><html><body>404</body></html>")
	})
	r := Verify(c, api(srv), time.Time{})
	if r.State != "limited" || r.Reason != ReasonNotAPI {
		t.Fatalf("want limited/not_api, got %+v", r)
	}
}

// An empty list proves the api works but has no content.
func TestVerifyEmptyIsLimited(t *testing.T) {
	srv, c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"list":[]}`)
	})
	r := Verify(c, api(srv), time.Time{})
	if r.State != "limited" || r.Reason != ReasonEmpty {
		t.Fatalf("want limited/empty, got %+v", r)
	}
}

// Entries without any playable link cannot be used by a client.
func TestVerifyNoPlayableIsLimited(t *testing.T) {
	srv, c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"list":[{"vod_name":"无链接"}]}`)
	})
	r := Verify(c, api(srv), time.Time{})
	if r.State != "limited" || r.Reason != ReasonNoPlay {
		t.Fatalf("want limited/no_playable, got %+v", r)
	}
}

// Only an explicit 404 counts as dead.
func TestVerify404IsDead(t *testing.T) {
	srv, c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	r := Verify(c, api(srv), time.Time{})
	if r.State != "down" || r.Reason != ReasonNotFound {
		t.Fatalf("want down/http_404, got %+v", r)
	}
}

// A repeated server error is a dead verdict too.
func TestVerify500IsDead(t *testing.T) {
	srv, c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	r := Verify(c, api(srv), time.Time{})
	if r.State != "down" || r.Reason != ReasonServer {
		t.Fatalf("want down/http_5xx, got %+v", r)
	}
}

// A single 5xx on one request shape is usually the gateway, not a dead api:
// every other shape fails at the network layer, so this must stay unverified.
func TestVerifySingleServerErrorIsUnverified(t *testing.T) {
	srv, c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery == "ac=detail&pg=1" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		panic(http.ErrAbortHandler) // drop the connection: a transport failure
	})
	r := Verify(c, api(srv), time.Time{})
	if r.State != "limited" || r.Reason != ReasonServer {
		t.Fatalf("want limited/http_5xx, got %+v", r)
	}
}

// A source record without an address is not a station that refused to serve.
func TestVerifyEmptyAddressIsUnverified(t *testing.T) {
	r := Verify(NewClient(time.Second), "   ", time.Time{})
	if r.State != "limited" || r.Reason != ReasonBadURL {
		t.Fatalf("want limited/bad_url, got %+v", r)
	}
}

// 403 / 429 usually means anti-bot or rate limiting, not a dead station.
func TestVerify403IsUnverified(t *testing.T) {
	srv, c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	r := Verify(c, api(srv), time.Time{})
	if r.State != "limited" || r.Reason != ReasonForbidden {
		t.Fatalf("want limited/forbidden, got %+v", r)
	}
}

// Timeouts describe the probe path, never the station.
func TestVerifyTimeoutIsUnverified(t *testing.T) {
	srv, c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(600 * time.Millisecond)
		fmt.Fprint(w, okJSON)
	})
	c.Timeout = 120 * time.Millisecond
	r := Verify(c, api(srv), time.Time{})
	if r.State != "limited" || r.Reason != ReasonTimeout {
		t.Fatalf("want limited/timeout, got %+v", r)
	}
	if r.LatencyMs != 0 {
		t.Fatalf("unreachable must not report latency, got %d", r.LatencyMs)
	}
}

// A refused connection is likewise "cannot verify", not "dead".
func TestVerifyConnRefusedIsUnverified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.URL
	srv.Close() // now nothing listens there
	c := NewClient(500 * time.Millisecond)
	r := Verify(c, addr+"/api.php/provide/vod", time.Time{})
	if r.State != "limited" {
		t.Fatalf("want limited, got %+v", r)
	}
	if r.Reason != ReasonConn && r.Reason != ReasonTimeout {
		t.Fatalf("want conn/timeout, got %q", r.Reason)
	}
}

// A self-signed certificate is common on these hosts and must not be fatal.
func TestVerifySelfSignedTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, okJSON)
	}))
	defer srv.Close()
	c := NewClient(3 * time.Second)
	r := Verify(c, srv.URL+"/api.php/provide/vod", time.Time{})
	if r.State != "ok" {
		t.Fatalf("self-signed host must verify insecurely, got %+v", r)
	}
}

// Proxy endpoints (?url=…) must receive the probe parameters inside url=,
// otherwise every request hits the proxy and never the station.
func TestCandidatesProxyURL(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner := r.URL.Query().Get("url")
		mu.Lock()
		seen = append(seen, inner)
		mu.Unlock()
		if strings.Contains(inner, "ac=detail") {
			fmt.Fprint(w, okJSON)
			return
		}
		fmt.Fprint(w, "<html>bad</html>")
	}))
	defer srv.Close()

	real := "https://real.example.com/api.php/provide/vod"
	target := srv.URL + "/?url=" + url.QueryEscape(real)
	c := NewClient(3 * time.Second)
	r := Verify(c, target, time.Time{})
	if r.State != "ok" {
		t.Fatalf("proxy endpoint must verify, got %+v", r)
	}
	// The parameters travel percent-encoded inside url=; the searchable flag
	// must still be detected from the decoded inner url.
	if !r.Searchable {
		t.Fatalf("proxy endpoint answering a keyword query must be searchable, got %+v", r)
	}
	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, s := range seen {
		if strings.Contains(s, "ac=detail") && strings.HasPrefix(s, real) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("parameters were not appended to the inner url, saw %v", seen)
	}
}

// Existing query parameters must not be duplicated across variants.
func TestVariantsDedupeQuery(t *testing.T) {
	got := variants("https://x.com/api.php/provide/vod?ac=detail&pg=1")
	for _, v := range got {
		if strings.Count(v, "ac=") > 1 {
			t.Fatalf("variant repeats ac parameter: %s", v)
		}
	}
	if len(got) == 0 {
		t.Fatal("no variants generated")
	}
}

// Run marks disabled entries off and never probes them.
func TestRunDisabledIsOff(t *testing.T) {
	sources := []model.Source{{Enabled: false, API: "https://example.com/api.php/provide/vod"}}
	res := Run(sources, 2, time.Second, time.Time{})
	if len(res) != 1 || res[0].State != "off" {
		t.Fatalf("want off, got %+v", res)
	}
}

// Run must classify a mixed list correctly.
func TestRunMixed(t *testing.T) {
	srvOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, okJSON)
	}))
	defer srvOK.Close()
	srvDown := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srvDown.Close()

	srcs := []model.Source{
		{Enabled: true, API: srvOK.URL + "/api.php/provide/vod"},
		{Enabled: true, API: srvDown.URL + "/api.php/provide/vod"},
	}
	res := Run(srcs, 4, 2*time.Second, time.Time{})
	if res[0].State != "ok" || res[1].State != "down" {
		t.Fatalf("want ok/down, got %+v / %+v", res[0], res[1])
	}
}

// A deadline that already passed must not produce a dead verdict.
func TestRunBudgetIsNotDead(t *testing.T) {
	srv, _ := serve(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, okJSON) })
	res := Run([]model.Source{{Enabled: true, API: api(srv)}}, 1, time.Second, time.Now().Add(-time.Second))
	if len(res) != 1 || res[0].State != "limited" || res[0].Reason != ReasonBudget {
		t.Fatalf("want limited/budget, got %+v", res[0])
	}
}

// result must stay JSON-serialisable for api/probe.json.
func TestResultJSON(t *testing.T) {
	b, err := json.Marshal(Result{State: "limited", Reason: ReasonTLS})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"Reason":"tls"`) {
		t.Fatalf("unexpected json: %s", b)
	}
}
