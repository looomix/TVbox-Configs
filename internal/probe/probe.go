// Package probe decides whether a video CMS endpoint answers with a real vod
// list.
//
// The verdict is deliberately conservative, because a probe host and a user are
// rarely on the same network: a station reachable from a home connection in
// China is often slow, geo-restricted or behind a broken certificate when
// reached from a cloud runner abroad. Judging such a station "dead" is wrong.
//
// Rules:
//
//	ok      – a valid list with playable links was returned.
//	limited – the host answered but not with a usable list (anti-bot page,
//	          wrong parameters, empty result, XML/JSON mismatch). Presence of
//	          the host is proven, usability is not, so it stays unverified.
//	down    – the station explicitly refuses to serve the endpoint: HTTP
//	          404/410 or a repeated 5xx.
//	off     – the source is disabled by the user.
//
// Network-layer failures (DNS, TLS handshake, timeout, connection reset) are
// never reported as down: they describe the path between the probe and the
// station, not the station itself. They become limited with a reason code.
package probe

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net"
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
	// perSourceCap bounds one endpoint so a handful of slow candidates can
	// never eat the whole run budget.
	perSourceCap = 45 * time.Second
)

// Reason codes are machine readable and rendered as a hint in the admin page.
const (
	ReasonBadURL    = "bad_url"
	ReasonNotAPI    = "not_api"     // answered, but not a vod api
	ReasonEmpty     = "empty"       // valid api, no entries
	ReasonNoPlay    = "no_playable" // entries without playable links
	ReasonForbidden = "forbidden"   // 403/429: anti-bot or rate limit
	ReasonNotFound  = "http_404"    // 404/410: endpoint does not exist
	ReasonServer    = "http_5xx"    // server error
	ReasonTimeout   = "timeout"
	ReasonDNS       = "dns"
	ReasonTLS       = "tls"
	ReasonConn      = "conn"
	ReasonBudget    = "budget" // run budget ran out before a verdict
)

// keywords are tried in order; a second title avoids false negatives when the
// first one is simply missing from that site's index.
var keywords = []string{"庆余年", "哪吒"}

// Result is the verdict of one endpoint.
type Result struct {
	State      string // ok | limited | down | off
	LatencyMs  int
	Vods       int
	Searchable bool
	Reason     string // empty when ok
}

// NewClient builds one reusable http client for probing.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: newTransport(false)}
}

func newTransport(insecure bool) *http.Transport {
	return &http.Transport{
		MaxIdleConns:        128,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 8 * time.Second,
		// Some chinese CDNs stall on the http/2 upgrade, plain http/1.1 is safer.
		ForceAttemptHTTP2: false,
		// #nosec G402 -- only third-party video endpoints are probed; no
		// credential or personal data is ever sent over this connection.
		// Expired / self-signed certificates are common on these hosts and
		// must not be reported as "dead".
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
}

// plainClient clones c with certificate verification disabled, used as a second
// chance when the TLS handshake fails for a certificate reason.
func plainClient(c *http.Client) *http.Client {
	tr, ok := c.Transport.(*http.Transport)
	if !ok || tr == nil {
		return &http.Client{Timeout: c.Timeout, Transport: newTransport(true)}
	}
	clone := tr.Clone()
	clone.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 -- see newTransport
	return &http.Client{Timeout: c.Timeout, Transport: clone}
}

type item struct {
	Name string `json:"vod_name"`
	Play string `json:"vod_play_url"`
	Down string `json:"vod_down_url"`
	URL  string `json:"vod_url"`
}

type payload struct {
	List []item `json:"list"`
	Data struct {
		List []item `json:"list"`
	} `json:"data"`
}

// xmlVideo is one <video> node of the Apple CMS XML output.
type xmlVideo struct {
	Name string   `xml:"name"`
	Play string   `xml:"play_url"`
	DDs  []string `xml:"dl>dd"`
}

// outcome is the internal, more granular result used while probing. It keeps
// the difference between "refused", "wrong content" and "unreachable" so the
// final verdict never collapses them into a single "dead".
type outcome struct {
	kind   string // ok | limited | notfound | server | unreachable
	reason string
	rank   int
	vods   int
	ms     int
	search uint8 // 1 = keyword search worked
}

func (o outcome) better(than outcome) bool { return o.rank > than.rank }

func join(api, query string) string {
	sep := "?"
	if strings.Contains(api, "?") {
		sep = "&"
	}
	return api + sep + query
}

// variants lists the request shapes worth trying for one endpoint, most likely
// first. Many CMS deployments accept only a subset of them (search disabled,
// videolist only, xml by default), so trying several is what stops a healthy
// station from being called dead.
func variants(api string) []string {
	base := strings.TrimSpace(api)
	if base == "" {
		return nil
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return []string{base}
	}
	// Drop parameters that only make sense for one probe shape; they are added
	// again per variant. The path stays untouched.
	q := u.Query()
	for _, k := range []string{"wd", "ac", "pg", "pagesize", "at"} {
		q.Del(k)
	}
	clean := *u
	clean.RawQuery = q.Encode()
	base = strings.TrimRight(clean.String(), "/")

	out := []string{
		join(base, "ac=detail&pg=1"),
		join(base, "wd="+url.QueryEscape(keywords[0])+"&ac=detail"),
		join(base, "ac=videolist&pg=1"),
		join(base, "ac=detail&pg=1&at=json"),
		base,
	}
	return dedupeStrings(out)
}

// candidates resolves the request shapes for one configured API value. Proxy
// style endpoints (…?url=<real api>) must receive the parameters inside the
// url= target, otherwise every request hits the proxy and never the station.
func candidates(api string) []string {
	base := strings.TrimSpace(api)
	if base == "" {
		return nil
	}
	u, err := url.Parse(base)
	if err != nil {
		return variants(base)
	}
	inner := strings.TrimSpace(u.Query().Get("url"))
	if inner == "" {
		return variants(base)
	}
	var out []string
	for _, v := range variants(inner) {
		q := u.Query()
		q.Set("url", v)
		nu := *u
		nu.RawQuery = q.Encode()
		out = append(out, nu.String())
	}
	if len(out) == 0 {
		return []string{base}
	}
	return dedupeStrings(out)
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func isCertError(err error) bool {
	if err == nil {
		return false
	}
	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "x509:") ||
		strings.Contains(s, "certificate is not valid") ||
		strings.Contains(s, "certificate signed by unknown authority") ||
		strings.Contains(s, "certificate has expired")
}

// netOutcome classifies a transport error. None of these are reported as dead.
func netOutcome(err error) outcome {
	if isCertError(err) {
		return outcome{kind: "unreachable", reason: ReasonTLS, rank: 1, ms: 0}
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return outcome{kind: "unreachable", reason: ReasonDNS, rank: 1, ms: 0}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return outcome{kind: "unreachable", reason: ReasonTimeout, rank: 1, ms: 0}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return outcome{kind: "unreachable", reason: ReasonTimeout, rank: 1, ms: 0}
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "no such host"), strings.Contains(s, "server misbehaving"):
		return outcome{kind: "unreachable", reason: ReasonDNS, rank: 1, ms: 0}
	case strings.Contains(s, "timeout"), strings.Contains(s, "deadline exceeded"):
		return outcome{kind: "unreachable", reason: ReasonTimeout, rank: 1, ms: 0}
	case strings.Contains(s, "connection refused"), strings.Contains(s, "connection reset"),
		strings.Contains(s, "broken pipe"), strings.Contains(s, "unexpected eof"):
		return outcome{kind: "unreachable", reason: ReasonConn, rank: 1, ms: 0}
	}
	return outcome{kind: "unreachable", reason: ReasonConn, rank: 1, ms: 0}
}

// parseBody turns a response body into an outcome when it carries a vod list.
// It understands the JSON shape and the XML shape Apple CMS serves by default.
func parseBody(body []byte) outcome {
	text := strings.TrimSpace(strings.TrimPrefix(string(body), "\ufeff"))
	switch {
	case strings.HasPrefix(text, "{"):
		var p payload
		if err := json.Unmarshal([]byte(text), &p); err != nil {
			return outcome{kind: "limited", reason: ReasonNotAPI, rank: 4}
		}
		list := p.List
		if len(list) == 0 {
			list = p.Data.List
		}
		return judgeItems(list)
	case strings.HasPrefix(text, "<"):
		if o, ok := parseXML([]byte(text)); ok {
			return o
		}
		return outcome{kind: "limited", reason: ReasonNotAPI, rank: 4}
	}
	return outcome{kind: "limited", reason: ReasonNotAPI, rank: 4}
}

func parseXML(data []byte) (outcome, bool) {
	videos := []xmlVideo{}
	var wrapped struct {
		Videos []xmlVideo `xml:"list>video"`
	}
	if err := xml.Unmarshal(data, &wrapped); err == nil && len(wrapped.Videos) > 0 {
		videos = wrapped.Videos
	} else {
		var flat struct {
			Videos []xmlVideo `xml:"video"`
		}
		if err := xml.Unmarshal(data, &flat); err != nil || len(flat.Videos) == 0 {
			return outcome{}, false
		}
		videos = flat.Videos
	}
	list := make([]item, 0, len(videos))
	for _, v := range videos {
		play := v.Play + strings.Join(v.DDs, "#")
		list = append(list, item{Name: v.Name, Play: play})
	}
	return judgeItems(list), true
}

func judgeItems(list []item) outcome {
	named, playable := 0, 0
	for _, it := range list {
		if strings.TrimSpace(it.Name) != "" {
			named++
		}
		links := it.Play + it.Down + it.URL
		if strings.Contains(strings.ToLower(links), "http") {
			playable++
		}
	}
	if len(list) == 0 || named == 0 {
		return outcome{kind: "limited", reason: ReasonEmpty, rank: 5, vods: len(list)}
	}
	if playable == 0 {
		return outcome{kind: "limited", reason: ReasonNoPlay, rank: 5, vods: len(list)}
	}
	return outcome{kind: "ok", reason: "", rank: 6, vods: len(list), search: 1}
}

// fetchOnce performs one request. Certificate failures are retried once with
// verification disabled, because those hosts are commonly usable in practice.
func fetchOnce(c, plain *http.Client, target string, deadline time.Time) outcome {
	start := time.Now()
	ctx := context.Background()
	if !deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}
	resp, err := do(c, ctx, target)
	if err != nil && isCertError(err) && plain != nil {
		resp, err = do(plain, ctx, target)
	}
	if err != nil {
		return netOutcome(err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	ms := int(time.Since(start).Milliseconds())
	if ms < 1 {
		// A response always means a measurable delay. 0 is reserved for
		// "no response", which the admin page renders as "—".
		ms = 1
	}
	if readErr != nil {
		return outcome{kind: "unreachable", reason: ReasonConn, rank: 1, ms: 0}
	}
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return outcome{kind: "notfound", reason: ReasonNotFound, rank: 2, ms: ms}
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		// anti-bot or rate limiting: the station exists, we just cannot read it
		return outcome{kind: "limited", reason: ReasonForbidden, rank: 3, ms: ms}
	case resp.StatusCode >= 500:
		return outcome{kind: "server", reason: ReasonServer, rank: 2, ms: ms}
	case resp.StatusCode != http.StatusOK:
		return outcome{kind: "limited", reason: ReasonNotAPI, rank: 4, ms: ms}
	}
	o := parseBody(body)
	o.ms = ms
	if strings.Contains(target, "wd=") && o.kind == "ok" {
		o.search = 1
	}
	return o
}

func do(c *http.Client, ctx context.Context, target string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json, text/plain, application/xml, text/xml, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.5")
	if u, err := url.Parse(target); err == nil && u.Host != "" {
		req.Header.Set("Referer", u.Scheme+"://"+u.Host+"/")
	}
	return c.Do(req)
}

// Verify decides the state of one endpoint. It tries the candidate shapes in
// order and stops at the first usable one. A non-zero deadline stops the
// remaining requests, so a loose budget is never overshot by a single source.
func Verify(c *http.Client, api string, deadline time.Time) Result {
	if strings.TrimSpace(api) == "" {
		return Result{State: "down", Reason: ReasonBadURL}
	}
	start := time.Now()
	if deadline.IsZero() || deadline.After(start.Add(perSourceCap)) {
		deadline = start.Add(perSourceCap)
	}
	plain := plainClient(c)

	var best outcome
	found := false
	unreachable := 0
	for _, target := range candidates(api) {
		if time.Now().After(deadline) {
			break
		}
		o := fetchOnce(c, plain, target, deadline)
		if !found || o.better(best) {
			best, found = o, true
		}
		if o.kind == "ok" {
			return finalize(o)
		}
		// 同一个目标的连续网络层失败（DNS/证书/超时）换请求形态也不会有
		// 不同结果，确认两次后收手，把预算留给其他源。
		if o.kind == "unreachable" {
			unreachable++
			if unreachable >= 2 {
				break
			}
		} else {
			unreachable = 0
		}
	}
	if !found {
		return Result{State: "limited", Reason: ReasonBudget}
	}
	// Down is reserved for explicit refusals only.
	if best.kind == "notfound" {
		return Result{State: "down", Reason: ReasonNotFound, LatencyMs: best.ms}
	}
	if best.kind == "server" {
		return Result{State: "down", Reason: ReasonServer, LatencyMs: best.ms}
	}
	return finalize(best)
}

func finalize(o outcome) Result {
	r := Result{
		LatencyMs:  o.ms,
		Vods:       o.vods,
		Searchable: o.search == 1,
		Reason:     o.reason,
	}
	switch o.kind {
	case "ok":
		r.State = "ok"
	case "notfound", "server":
		r.State = "down"
	default:
		r.State = "limited"
	}
	// Without a response there is no meaningful latency.
	if o.kind == "unreachable" {
		r.LatencyMs = 0
	}
	return r
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
					results[j] = Result{State: "limited", Reason: ReasonBudget}
					continue
				}
				r := Verify(c, sources[j].API, deadline)
				if r.State != "ok" && !deadline.IsZero() && time.Now().After(deadline) {
					// cut short by the budget: never report a partial probe as dead
					r.State = "limited"
					if r.Reason == "" || r.Reason == ReasonNotFound || r.Reason == ReasonServer {
						r.Reason = ReasonBudget
					}
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
