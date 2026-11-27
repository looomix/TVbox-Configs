// Command tvboxgen merges the local source list with optional remote lists,
// checks every endpoint and writes the published artifacts (one config per
// client, a plain text list, the health report and the mirror list).
//
// The whole run is capped by -budget so a scheduled job always finishes fast.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
	owner := flag.String("owner", "looomix", "github owner used for mirror urls")
	repo := flag.String("repo", "TVbox-Configs", "github repo used for mirror urls")
	branch := flag.String("branch", "main", "branch used for mirror urls")
	feedOnly := flag.Bool("feed-only", false, "only merge feeds into sources.json, do not probe")
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
		r := probe.Verify(probe.NewClient(*timeout), *probeURL)
		res := model.ProbeResult{
			URL:       *probeURL,
			State:     r.State,
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
		fmt.Printf("probe %s -> %s, %d ms, %d results\n", res.URL, res.State, res.LatencyMs, res.Vods)
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
	raw := append([]model.Source{}, sf.Sources...)
	var ff model.FeedFile
	if err := gen.ReadJSON(*feedsPath, &ff); err == nil {
		raw = append(raw, fetchFeeds(ff, *timeout, deadline)...)
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
	list, dup := gen.Dedupe(kept)

	// prefer the mirror that verified fine last round when names collide
	prevOK := make(map[string]bool, len(prev.Sources))
	for _, st := range prev.Sources {
		if st.Enabled && st.State == "ok" {
			prevOK[gen.NormalizeAPI(st.API)] = true
		}
	}
	list, nameDup := gen.DedupeNames(list, prevOK)
	dup += nameDup

	fromFeed := 0
	for _, s := range list {
		if s.Origin == "feed" {
			fromFeed++
		}
	}

	newFile := model.SourceFile{Version: 1, UpdatedAt: start.Format("2006-01-02"), Removed: sf.Removed, Sources: list}
	changed, err := gen.WriteJSONIfChanged(*sourcesPath, newFile)
	if err != nil {
		fmt.Printf("write %s: %v\n", *sourcesPath, err)
		os.Exit(1)
	}
	if changed {
		fmt.Printf("updated %s\n", *sourcesPath)
	}
	fmt.Printf("sources: %d read, %d deleted by user, %d duplicates removed, %d from feeds, %d to check\n",
		len(raw), gone, dup, fromFeed, len(list))
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
		IntervalHours: int(interval.Hours()),
		Total:         len(list),
		FromFeed:      fromFeed,
		Deduped:       dup,
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
	if !deadline.IsZero() && time.Now().After(deadline) {
		fmt.Printf("warn: run passed the %s budget, lower -timeout or raise -workers\n", *budget)
	}
}

// fetchFeeds downloads every enabled remote list in parallel and caps each
// request by the remaining budget, so one slow list cannot stall the run.
func fetchFeeds(ff model.FeedFile, timeout time.Duration, deadline time.Time) []model.Source {
	jobs := make([]model.Feed, 0, len(ff.Feeds))
	for _, f := range ff.Feeds {
		if f.Enabled && strings.TrimSpace(f.URL) != "" {
			jobs = append(jobs, f)
		}
	}
	if len(jobs) == 0 {
		return nil
	}
	limit := timeout * 5
	if limit < 10*time.Second {
		limit = 10 * time.Second
	}
	results := make([][]model.Source, len(jobs))
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
			results[i] = items
		}(i)
	}
	wg.Wait()
	var out []model.Source
	for _, r := range results {
		out = append(out, r...)
	}
	return out
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

	files := []struct{ file, kind string }{
		{"tvbox.json", "config"},
		{"moontv.json", "config"},
		{"katelyatv.json", "config"},
		{"decotv.json", "config"},
		{"moontvplus.json", "config"},
		{"ouonnkitv.json", "config"},
		{"kvideo.json", "config"},
		{"libretv.json", "config"},
		{"sources.txt", "text"},
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
