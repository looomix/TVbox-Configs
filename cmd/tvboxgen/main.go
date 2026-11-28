// Command tvboxgen merges the local source list with optional remote lists,
// checks every endpoint and writes the published artifacts (one config per
// client, a plain text list, the health report and the mirror list).
//
// The whole run is capped by -budget so a scheduled job always finishes fast.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/looomix/tvbox-configs/internal/feed"
	"github.com/looomix/tvbox-configs/internal/gen"
	"github.com/looomix/tvbox-configs/internal/mirror"
	"github.com/looomix/tvbox-configs/internal/model"
	"github.com/looomix/tvbox-configs/internal/probe"
)

func main() {
	sourcesPath := flag.String("sources", "sources/sources.json", "master source list")
	feedsPath := flag.String("feeds", "sources/feeds.json", "optional remote source lists")
	outDir := flag.String("out", "api", "output directory")
	interval := flag.Duration("interval", 7*24*time.Hour, "minimum gap between two real updates")
	force := flag.Bool("force", false, "ignore the interval and update now")
	timeout := flag.Duration("timeout", 8*time.Second, "per request timeout")
	workers := flag.Int("workers", 24, "parallel probes")
	budget := flag.Duration("budget", 50*time.Second, "hard cap for the whole run, 0 disables it")
	histLimit := flag.Int("history", 8, "check points kept per source")
	dOwner, dRepo, dBranch := repoDefaults()
	owner := flag.String("owner", dOwner, "github owner used for mirror urls")
	repo := flag.String("repo", dRepo, "github repo used for mirror urls")
	branch := flag.String("branch", dBranch, "branch used for mirror urls")
	feedOnly := flag.Bool("feed-only", false, "only merge feeds into sources.json, do not probe")
	allowNSFW := flag.Bool("allow-nsfw", false, "keep adult-labelled stations in the published artifacts")
	probeURL := flag.String("probe", "", "probe one api url only and exit")
	probeOut := flag.String("probe-out", "", "write the single probe result to this json file")
	flag.Parse()

	start := time.Now()
	rel := func(name string) string { return filepath.Join(*outDir, name) }
	var deadline time.Time
	if *budget > 0 {
		deadline = start.Add(*budget)
	}

	// single source mode: used by the web page through the probe workflow
	if *probeURL != "" {
		if !strings.HasPrefix(*probeURL, "http://") && !strings.HasPrefix(*probeURL, "https://") {
			fmt.Println("probe url must start with http:// or https://")
			os.Exit(1)
		}
		r := probe.Verify(probe.NewClient(*timeout), *probeURL, time.Time{})
		res := model.ProbeResult{
			URL:       *probeURL,
			State:     r.State,
			Reason:    r.Reason,
			Vods:      r.Vods,
			LatencyMs: r.LatencyMs,
			CheckedAt: start.UTC().Format(time.RFC3339),
		}
		if *probeOut != "" {
			if err := gen.WriteJSON(*probeOut, res); err != nil {
				fmt.Printf("write %s: %v\n", *probeOut, err)
				os.Exit(1)
			}
		}
		fmt.Printf("probe %s -> %s, %d ms, %d results, reason=%s\n", res.URL, res.State, res.LatencyMs, res.Vods, res.Reason)
		return
	}

	var sf model.SourceFile
	if err := gen.ReadJSON(*sourcesPath, &sf); err != nil {
		fmt.Printf("read %s: %v\n", *sourcesPath, err)
		os.Exit(1)
	}

	var prev model.Report
	hasPrev := gen.ReadJSON(rel("status.json"), &prev) == nil
	if !*force && !*feedOnly && hasPrev && prev.UpdatedAt != "" {
		if last, err := time.Parse(time.RFC3339, prev.UpdatedAt); err == nil {
			if next := last.Add(*interval); start.Before(next) {
				fmt.Printf("skip: last update %s, next update %s\n",
					last.Local().Format("2006-01-02 15:04"), next.Local().Format("2006-01-02 15:04"))
				return
			}
		}
	}

	removed := make(map[string]bool, len(sf.Removed))
	for _, k := range sf.Removed {
		removed[k] = true
	}

	// remote lists are optional; the project ships with none enabled
	feedDeadline := deadline
	if !deadline.IsZero() {
		feedDeadline = start.Add(*budget * 2 / 5)
	}
	raw := append([]model.Source{}, sf.Sources...)
	var ff model.FeedFile
	var consumed []string
	if err := gen.ReadJSON(*feedsPath, &ff); err == nil {
		var fetched []model.Source
		fetched, consumed = fetchFeeds(ff, *timeout, feedDeadline)
		raw = append(raw, fetched...)
	} else if !os.IsNotExist(err) {
		fmt.Printf("warn: feed file %s: %v\n", *feedsPath, err)
	}

	kept := make([]model.Source, 0, len(raw))
	gone := 0
	for _, s := range raw {
		if removed[gen.NormalizeAPI(s.API)] {
			// the user deleted this endpoint, so feeds must not bring it back
			gone++
			continue
		}
		kept = append(kept, s)
	}
	// one entry per endpoint: the link decides, display names are never merged
	allList, dup := gen.Dedupe(kept)

	// 过滤只在"探活 + 发布"这条链路上生效，不能写回 sources.json：
	// 否则用户手动加的源一旦命中成人关键词就会被永久删除，-allow-nsfw 也找不回。
	list := allList
	filtered := 0
	if !*allowNSFW {
		list, filtered = gen.FilterNSFW(allList)
	}

	fromFeed := 0
	for _, s := range list {
		if s.Origin == "feed" {
			fromFeed++
		}
	}

	// 源清单始终写全量，保持用户数据完整；发布产物由 list 决定。
	newFile := model.SourceFile{Version: 1, Removed: sf.Removed, Sources: allList}
	changed, err := gen.WriteJSONIfChanged(*sourcesPath, newFile)
	if err != nil {
		fmt.Printf("write %s: %v\n", *sourcesPath, err)
		os.Exit(1)
	}
	if changed {
		fmt.Printf("updated %s\n", *sourcesPath)
	}
	fmt.Printf("sources: %d read, %d deleted by user, %d duplicates removed, %d filtered, %d from feeds, %d to check\n",
		len(raw), gone, dup, filtered, fromFeed, len(list))

	// 订阅链接是一次性导入队列：抓取合并进 sources.json 后立刻清空。
	// 永久保存会让文件无限累积，还会在用户删源后把同一批源重新抓回来。
	consumeFeeds(*feedsPath, ff, consumed)
	if *feedOnly {
		return
	}

	probeStart := time.Now()
	results := probe.Run(list, *workers, *timeout, deadline)

	oldHistory := map[string][]model.Check{}
	for _, s := range prev.Sources {
		oldHistory[s.ID] = s.History
	}

	rep := &model.Report{
		UpdatedAt:     start.UTC().Format(time.RFC3339),
		NextUpdateAt:  start.Add(*interval).UTC().Format(time.RFC3339),
		IntervalHours: int(math.Round(interval.Hours())),
		Total:         len(list),
		FromFeed:      fromFeed,
		Deduped:       dup,
		Filtered:      filtered,
		Sources:       make([]model.Status, 0, len(list)),
	}
	for i, s := range list {
		r := results[i]
		st := model.Status{
			ID:         s.ID,
			Name:       s.Name,
			API:        s.API,
			Group:      s.Group,
			Note:       s.Note,
			Origin:     s.Origin,
			Enabled:    s.Enabled,
			State:      r.State,
			Reason:     r.Reason,
			LatencyMs:  r.LatencyMs,
			Vods:       r.Vods,
			Searchable: r.Searchable,
			CheckedAt:  start.UTC().Format(time.RFC3339),
		}
		if s.Enabled {
			st.History = gen.AppendCheck(oldHistory[s.ID], r.State == "ok", r.LatencyMs, start, *histLimit)
		} else {
			st.History = oldHistory[s.ID]
		}
		switch {
		case !s.Enabled:
			// disabled sources are not part of the health numbers
		case st.State == "ok":
			rep.Alive++
		case st.State == "limited":
			rep.Unverified++
		default:
			rep.Dead++
		}
		rep.Sources = append(rep.Sources, st)
	}
	rep.DurationMs = time.Since(probeStart).Milliseconds()

	sites := gen.SitesOf(rep)
	if err := writeArtifacts(*outDir, *owner, *repo, *branch, sites, rep.UpdatedAt); err != nil {
		fmt.Printf("write artifacts: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteJSON(rel("status.json"), rep); err != nil {
		fmt.Printf("write status.json: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("done: %d verified / %d unverified / %d dead, %d sites published, %.1fs total\n",
		rep.Alive, rep.Unverified, rep.Dead, len(sites), time.Since(start).Seconds())
	if reasons := reasonSummary(rep); reasons != "" {
		fmt.Printf("not-ok reasons: %s\n", reasons)
	}
	if !deadline.IsZero() && time.Now().After(deadline) {
		fmt.Printf("budget %s reached: sources without a verdict stay unverified, raise -budget or -workers for full coverage\n", *budget)
	}
}

// reasonSummary counts why enabled sources did not verify, so one cloud run is
// diagnosable from its log without opening every record.
func reasonSummary(rep *model.Report) string {
	counts := map[string]int{}
	for _, s := range rep.Sources {
		if s.Enabled && s.State != "ok" && s.Reason != "" {
			counts[s.Reason]++
		}
	}
	if len(counts) == 0 {
		return ""
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, counts[k]))
	}
	return strings.Join(parts, " ")
}

// fetchFeeds downloads every enabled remote list in parallel and caps each
// request by the remaining budget, so one slow list cannot stall the run.
// It also returns the normalized URLs that were fetched successfully.
func fetchFeeds(ff model.FeedFile, timeout time.Duration, deadline time.Time) ([]model.Source, []string) {
	var consumed []string
	// one job per normalized url: pasted lists often carry a trailing slash or
	// repeat the same address twice
	jobs := make([]model.Feed, 0, len(ff.Feeds))
	seen := make(map[string]bool, len(ff.Feeds))
	for _, f := range ff.Feeds {
		u := strings.TrimRight(strings.TrimSpace(f.URL), "/")
		if !f.Enabled || u == "" || seen[u] {
			continue
		}
		seen[u] = true
		f.URL = u
		jobs = append(jobs, f)
	}
	if len(jobs) == 0 {
		return nil, nil
	}
	limit := timeout * 5
	if limit < 10*time.Second {
		limit = 10 * time.Second
	}
	results := make([][]model.Source, len(jobs))
	fetched := make([]bool, len(jobs))
	var wg sync.WaitGroup
	slots := make(chan struct{}, 6)
	for i := range jobs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			t := limit
			if !deadline.IsZero() {
				if left := time.Until(deadline); left < t {
					if left < 2*time.Second {
						return
					}
					t = left
				}
			}
			items, err := feed.Fetch(jobs[i].URL, t)
			if err != nil {
				fmt.Printf("feed %s failed: %v\n", jobs[i].Name, err)
				return
			}
			fmt.Printf("feed %s -> %d entries\n", jobs[i].Name, len(items))
			// 只有真正解析出可用源才算消费成功；空内容/错误页保留在队列里
			// 下次重试，否则一次性队列会静默丢失这条订阅。
			if len(items) > 0 {
				results[i] = items
				fetched[i] = true
			}
		}(i)
	}
	wg.Wait()
	var out []model.Source
	for i := range jobs {
		if fetched[i] {
			consumed = append(consumed, jobs[i].URL)
		}
	}
	for _, r := range results {
		out = append(out, r...)
	}
	return out, consumed
}

// consumeFeeds removes successfully fetched feeds from the one-shot queue and
// writes the rest back. Failed feeds stay so the next run can retry them; once
// every feed is consumed the file becomes {"feeds":[]} instead of accumulating.
func consumeFeeds(path string, ff model.FeedFile, consumed []string) {
	if len(ff.Feeds) == 0 || len(consumed) == 0 {
		return
	}
	done := make(map[string]bool, len(consumed))
	for _, u := range consumed {
		done[u] = true
	}
	left := make([]model.Feed, 0, len(ff.Feeds))
	for _, f := range ff.Feeds {
		if done[strings.TrimRight(strings.TrimSpace(f.URL), "/")] {
			continue
		}
		left = append(left, f)
	}
	if len(left) == len(ff.Feeds) {
		return
	}
	doc := model.FeedFile{Version: ff.Version, Feeds: left}
	if doc.Version == 0 {
		doc.Version = 1
	}
	if err := gen.WriteJSON(path, doc); err != nil {
		fmt.Printf("warn: update %s: %v\n", path, err)
	}
}

// repoDefaults reads the repository identity from the environment, so a fork's
// own subscription links are generated instead of the upstream ones.
func repoDefaults() (owner, repo, branch string) {
	owner, repo, branch = "looomix", "TVbox-Configs", "main"
	if full := strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY")); full != "" {
		if i := strings.Index(full, "/"); i > 0 {
			owner, repo = full[:i], full[i+1:]
		}
	}
	if ref := strings.TrimSpace(os.Getenv("GITHUB_REF_NAME")); ref != "" && !strings.Contains(ref, "/") {
		branch = ref
	}
	return owner, repo, branch
}

// writeArtifacts renders every published file from one verified site list.
func writeArtifacts(outDir, owner, repo, branch string, sites []model.Site, updatedAt string) error {
	for name, v := range gen.Artifacts(sites) {
		if err := gen.WriteJSON(filepath.Join(outDir, name), v); err != nil {
			return err
		}
	}
	if err := gen.WriteText(filepath.Join(outDir, "sources.txt"), gen.TXT(sites)); err != nil {
		return err
	}
	// A plain multi-warehouse list is accepted by TVBox builds that do not read
	// the JSON index. Each line is "<name>,<config url>".
	mirrors := mirror.List(owner, repo, branch, "api/tvbox.json")
	var multi strings.Builder
	multi.WriteString("# TVbox-Configs 多仓入口（名称,单仓配置地址）\n")
	limit := 3
	if len(mirrors) < limit {
		limit = len(mirrors)
	}
	for _, m := range mirrors[:limit] {
		multi.WriteString(m.Name + "," + m.URL + "\n")
	}
	if err := gen.WriteText(filepath.Join(outDir, "tvbox.txt"), multi.String()); err != nil {
		return err
	}

	files := []struct{ file, kind string }{
		{"tvbox.json", "config"},
		{"moontv.json", "config"},
		{"list.json", "config"},
		{"sources.txt", "text"},
		{"tvbox.txt", "text"},
	}
	groups := make([]model.MirrorGroup, 0, len(files))
	for _, f := range files {
		groups = append(groups, mirror.Group(owner, repo, branch, "api/"+f.file, gen.Labels[f.file], f.kind))
	}
	mf := &model.MirrorFile{UpdatedAt: updatedAt, Files: groups}
	if err := gen.WriteJSON(filepath.Join(outDir, "mirror.json"), mf); err != nil {
		return err
	}
	return gen.WriteJSON(filepath.Join(outDir, "index.json"), mirror.Index(owner, repo, branch, "api/tvbox.json"))
}
