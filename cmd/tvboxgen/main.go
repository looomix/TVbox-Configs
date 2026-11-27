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
	vantage := flag.String("vantage", "", "probe only and write api/status-<vantage>.json (us / cn)")
	rollback := flag.String("rollback", "", "rebuild every artifact from versions/<date>.json")
	snapDir := flag.String("versions", "versions", "version snapshot directory")
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

	// rollback mode: rebuild the published files from a dated snapshot
	if *rollback != "" {
		snap, err := gen.LoadSnapshot(*snapDir, *rollback)
		if err != nil {
			fmt.Printf("rollback: %v\n", err)
			os.Exit(1)
		}
		sites := gen.SitesOfSnapshot(snap)
		if err := writeArtifacts(*outDir, *owner, *repo, *branch, sites, snap.UpdatedAt); err != nil {
			fmt.Printf("rollback write: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("rollback done: %s, %d sites\n", snap.Date, len(sites))
		return
	}

	// vantage mode: re-probe the current published list from another network and
	// store the verdicts separately, so the main run can merge two viewpoints.
	if *vantage != "" {
		var rep model.Report
		if err := gen.ReadJSON(rel("status.json"), &rep); err != nil {
			fmt.Printf("vantage: read status.json: %v\n", err)
			os.Exit(1)
		}
		list := make([]model.Source, 0, len(rep.Sources))
		for _, st := range rep.Sources {
			list = append(list, model.Source{ID: st.ID, Name: st.Name, API: st.API, Group: st.Group, Enabled: st.Enabled})
		}
		fmt.Printf("vantage %s: probing %d sources ...\n", *vantage, len(list))
		results := probe.Run(list, *workers, *timeout)
		vf := model.VantageFile{Vantage: *vantage, CheckedAt: now.UTC().Format(time.RFC3339)}
		ok := 0
		for i, src := range list {
			r := results[i]
			vf.Sources = append(vf.Sources, model.VantageEntry{
				ID: src.ID, API: src.API, State: r.State, Vods: r.Vods, LatencyMs: r.LatencyMs, PlayOK: r.PlayOK,
			})
			if r.State == "ok" {
				ok++
			}
		}
		if err := gen.WriteJSON(rel("status-"+*vantage+".json"), vf); err != nil {
			fmt.Printf("write vantage: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("vantage %s done: %d/%d ok\n", *vantage, ok, len(list))
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

	cnByID, cnChecked := loadVantage(filepath.Join(*outDir, "status-cn.json"))

	rep := &model.Report{
		UpdatedAt:     now.UTC().Format(time.RFC3339),
		NextUpdateAt:  now.Add(*interval).UTC().Format(time.RFC3339),
		IntervalHours: int(interval.Hours()),
		Total:         len(list),
		FromFeed:      fromFeed,
		Deduped:       dup,
		Sources:       make([]model.Status, 0, len(list)),
	}
	rep.Vantages = []string{"primary"}
	if len(cnByID) > 0 {
		rep.Vantages = append(rep.Vantages, "cn")
	}
	checkedAt := now.UTC().Format(time.RFC3339)
	cnRescued := 0
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
		st.PlayOK = r.PlayOK
		st.PlaySample = r.PlaySample
		st.PlayNote = r.PlayNote
		if cn, ok := cnByID[s.ID]; ok {
			st.CNState = cn.State
			st.CNLatency = cn.LatencyMs
			st.CNChecked = cnChecked
			if r.State != "ok" && cn.State == "ok" {
				// the overseas vantage failed but the domestic one passed: publish
				// it, the subscription is consumed from inside china. the domestic
				// verdict already includes the play check, so trust it fully.
				st.State = "ok"
				st.LatencyMs = cn.LatencyMs
				st.Searchable = true
				st.PlayOK = true
				st.PlayNote = ""
				if st.Vods == 0 {
					st.Vods = cn.Vods
				}
				cnRescued++
			}
		}
		if s.Enabled {
			st.History = gen.AppendCheck(oldHistory[s.ID], st.State == "ok", st.LatencyMs, now, *histLimit)
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
	rep.DurationMs = time.Since(start).Milliseconds()

	sites := gen.SitesOf(rep)
	if err := writeArtifacts(*outDir, *owner, *repo, *branch, sites, rep.UpdatedAt); err != nil {
		fmt.Printf("write artifacts: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteJSON(rel("status.json"), rep); err != nil {
		fmt.Printf("write status.json: %v\n", err)
		os.Exit(1)
	}
	if row, err := gen.WriteSnapshot(*snapDir, gen.BuildSnapshot(now.Format("2006-01-02"), rep.UpdatedAt, rep.Total, rep.Alive, sites)); err == nil {
		fmt.Printf("snapshot %s: +%d / -%d\n", row.Date, len(row.Added), len(row.Removed))
	} else {
		fmt.Printf("warn: snapshot: %v\n", err)
	}

	if cnRescued > 0 {
		fmt.Printf("vantages merged: %d sources rescued by the cn vantage\n", cnRescued)
	}
	fmt.Printf("done: %d verified / %d unverified / %d dead, %d sites published, %d ms\n",
		rep.Alive, rep.Unverified, rep.Dead, len(sites), rep.DurationMs)
	next, _ := time.Parse(time.RFC3339, rep.NextUpdateAt)
	fmt.Printf("next update: %s\n", next.Local().Format("2006-01-02 15:04"))
}

// writeArtifacts renders every published file from one verified site list, so a
// normal run and a rollback produce exactly the same set of outputs.
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

// loadVantage reads a secondary vantage report and ignores stale files, so an
// old domestic snapshot never overrides today's verdicts.
func loadVantage(path string) (map[string]model.VantageEntry, string) {
	var vf model.VantageFile
	if err := gen.ReadJSON(path, &vf); err != nil || vf.CheckedAt == "" {
		return nil, ""
	}
	at, err := time.Parse(time.RFC3339, vf.CheckedAt)
	if err != nil || time.Since(at) > 30*24*time.Hour {
		return nil, ""
	}
	out := make(map[string]model.VantageEntry, len(vf.Sources))
	for _, e := range vf.Sources {
		out[e.ID] = e
	}
	return out, vf.CheckedAt
}
