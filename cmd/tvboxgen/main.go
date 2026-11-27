// Command tvboxgen merges local and remote source lists, verifies every
// endpoint and regenerates the published artifacts (TVBox config, MoonTV
// config, plain text list, health report and mirror list).
//
// Run it every 7 days from a schedule, or force a refresh with -force.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/looomix/tvbox-configs/internal/feed"
	"github.com/looomix/tvbox-configs/internal/gen"
	"github.com/looomix/tvbox-configs/internal/mirror"
	"github.com/looomix/tvbox-configs/internal/model"
	"github.com/looomix/tvbox-configs/internal/probe"
)

func main() {
	sourcesPath := flag.String("sources", "sources/sources.json", "master source list")
	feedsPath := flag.String("feeds", "sources/feeds.json", "remote source lists")
	blocklistPath := flag.String("blocklist", "sources/blocklist.json", "disallowed domains and words")
	outDir := flag.String("out", "api", "output directory")
	interval := flag.Duration("interval", 7*24*time.Hour, "minimum gap between two real updates")
	force := flag.Bool("force", false, "ignore the interval and update now")
	timeout := flag.Duration("timeout", 12*time.Second, "per request timeout")
	workers := flag.Int("workers", 10, "parallel probes")
	histLimit := flag.Int("history", 8, "check points kept per source")
	owner := flag.String("owner", "looomix", "github owner used for mirror urls")
	repo := flag.String("repo", "TVbox-Configs", "github repo used for mirror urls")
	branch := flag.String("branch", "main", "branch used for mirror urls")
	feedOnly := flag.Bool("feed-only", false, "only merge feeds into sources.json, do not probe")
	probeURL := flag.String("probe", "", "probe one api url only and exit (used by the cloud test workflow)")
	probeOut := flag.String("probe-out", "", "write the single probe result to this json file")
	flag.Parse()

	now := time.Now()
	rel := func(name string) string { return filepath.Join(*outDir, name) }

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
			LatencyMs: r.LatencyMs,
			Vods:      r.Vods,
			Playable:  r.Playable,
			CheckedAt: now.UTC().Format(time.RFC3339),
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
			if next := last.Add(*interval); now.Before(next) {
				fmt.Printf("skip: last update %s, next update %s\n",
					last.Local().Format("2006-01-02 15:04"), next.Local().Format("2006-01-02 15:04"))
				return
			}
		}
	}

	var bl model.Blocklist
	if err := gen.ReadJSON(*blocklistPath, &bl); err != nil {
		fmt.Printf("warn: blocklist %s: %v\n", *blocklistPath, err)
	}
	blocker := gen.NewBlocker(bl)

	// merge manual sources and remote feeds
	var raw []model.Source
	raw = append(raw, sf.Sources...)
	var ff model.FeedFile
	if err := gen.ReadJSON(*feedsPath, &ff); err == nil {
		for _, f := range ff.Feeds {
			if !f.Enabled || strings.TrimSpace(f.URL) == "" {
				continue
			}
			if feed.RefuseFeed(f.URL) {
				fmt.Printf("feed %s skipped: disallowed list\n", f.Name)
				continue
			}
			items, err := feed.Fetch(f.URL, 40*time.Second)
			if err != nil {
				fmt.Printf("feed %s failed: %v\n", f.Name, err)
				continue
			}
			fmt.Printf("feed %s -> %d entries\n", f.Name, len(items))
			raw = append(raw, items...)
		}
	} else if !os.IsNotExist(err) {
		fmt.Printf("warn: feed file %s: %v\n", *feedsPath, err)
	}

	removed := make(map[string]bool, len(sf.Removed))
	for _, k := range sf.Removed {
		removed[k] = true
	}

	kept := make([]model.Source, 0, len(raw))
	blocked, gone := 0, 0
	for _, s := range raw {
		if blocker.Match(s) {
			blocked++
			continue
		}
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
	fmt.Printf("sources: %d total, %d blocked, %d user removed, %d duplicates removed, %d from feeds\n",
		len(raw), blocked, gone, dup, fromFeed)

	newFile := model.SourceFile{Version: 1, UpdatedAt: now.Format("2006-01-02"), Removed: sf.Removed, Sources: list}
	changed, err := gen.WriteJSONIfChanged(*sourcesPath, newFile)
	if err != nil {
		fmt.Printf("write %s: %v\n", *sourcesPath, err)
		os.Exit(1)
	}
	if changed {
		fmt.Printf("updated %s\n", *sourcesPath)
	}
	if *feedOnly {
		return
	}

	fmt.Printf("checking %d sources ...\n", len(list))
	start := time.Now()
	results := probe.Run(list, *workers, *timeout)

	oldHistory := map[string][]model.Check{}
	for _, s := range prev.Sources {
		oldHistory[s.ID] = s.History
	}

	rep := &model.Report{
		UpdatedAt:     now.UTC().Format(time.RFC3339),
		NextUpdateAt:  now.Add(*interval).UTC().Format(time.RFC3339),
		IntervalHours: int(interval.Hours()),
		Total:         len(list),
		FromFeed:      fromFeed,
		Deduped:       dup,
		Sources:       make([]model.Status, 0, len(list)),
	}
	checkedAt := now.UTC().Format(time.RFC3339)
	for i, s := range list {
		r := results[i]
		st := model.Status{
			ID:        s.ID,
			Name:      s.Name,
			API:       s.API,
			Group:     s.Group,
			Note:      s.Note,
			Origin:    s.Origin,
			Enabled:   s.Enabled,
			State:     r.State,
			LatencyMs: r.LatencyMs,
			Vods:      r.Vods,
		}
		st.Searchable = r.Searchable
		st.CheckedAt = checkedAt
		if s.Enabled {
			st.History = gen.AppendCheck(oldHistory[s.ID], r.State == "ok", r.LatencyMs, now, *histLimit)
		} else {
			st.History = oldHistory[s.ID]
		}
		switch {
		case !s.Enabled:
			// disabled sources are not part of the health numbers
		case r.State == "ok":
			rep.Alive++
		case r.State == "limited":
			rep.Unverified++
		default:
			rep.Dead++
		}
		rep.Sources = append(rep.Sources, st)
	}
	rep.DurationMs = time.Since(start).Milliseconds()

	sites := gen.SitesOf(rep)
	cfg := gen.TVBox(sites)
	moon := gen.MoonTV(sites)
	txt := gen.TXT(sites)
	mf := &model.MirrorFile{
		UpdatedAt: rep.UpdatedAt,
		Files: []model.MirrorGroup{
			mirror.Group(*owner, *repo, *branch, "api/tvbox.json", "TVBox / 影视仓 单仓配置", "config"),
			mirror.Group(*owner, *repo, *branch, "api/moontv.json", "MoonTV / LunaTV 视频源", "config"),
			mirror.Group(*owner, *repo, *branch, "api/sources.txt", "纯文本源清单（可粘贴导入）", "text"),
		},
	}
	idx := mirror.Index(*owner, *repo, *branch, "api/tvbox.json")

	for name, v := range map[string]any{
		"tvbox.json":  cfg,
		"moontv.json": moon,
		"status.json": rep,
		"mirror.json": mf,
		"index.json":  idx,
	} {
		if err := gen.WriteJSON(rel(name), v); err != nil {
			fmt.Printf("write %s: %v\n", name, err)
			os.Exit(1)
		}
	}
	if err := gen.WriteText(rel("sources.txt"), txt); err != nil {
		fmt.Printf("write sources.txt: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("done: %d verified / %d unverified / %d dead, %d sites published, %d ms\n",
		rep.Alive, rep.Unverified, rep.Dead, len(sites), rep.DurationMs)
	next, _ := time.Parse(time.RFC3339, rep.NextUpdateAt)
	fmt.Printf("next update: %s\n", next.Local().Format("2006-01-02 15:04"))
}
