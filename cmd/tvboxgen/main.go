// Command tvboxgen probes the configured video sources and regenerates the
// published TVBox config, status report and mirror list.
//
// Run it every 7 days (default) from a schedule, or force a refresh with -force.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/looomix/tvbox-configs/internal/gen"
	"github.com/looomix/tvbox-configs/internal/mirror"
	"github.com/looomix/tvbox-configs/internal/model"
	"github.com/looomix/tvbox-configs/internal/probe"
)

func main() {
	sourcesPath := flag.String("sources", "sources/sources.json", "master source list")
	outDir := flag.String("out", "api", "output directory")
	interval := flag.Duration("interval", 7*24*time.Hour, "minimum gap between two real updates")
	force := flag.Bool("force", false, "ignore the interval and update now")
	timeout := flag.Duration("timeout", 12*time.Second, "per request timeout")
	workers := flag.Int("workers", 10, "parallel probes")
	histLimit := flag.Int("history", 8, "check points kept per source")
	owner := flag.String("owner", "looomix", "github owner used for mirror urls")
	repo := flag.String("repo", "TVbox-Configs", "github repo used for mirror urls")
	branch := flag.String("branch", "main", "branch used for mirror urls")
	flag.Parse()

	now := time.Now()
	rel := func(name string) string { return filepath.Join(*outDir, name) }

	var sf model.SourceFile
	if err := gen.ReadJSON(*sourcesPath, &sf); err != nil {
		fmt.Printf("read %s: %v\n", *sourcesPath, err)
		os.Exit(1)
	}

	var prev model.Report
	hasPrev := gen.ReadJSON(rel("status.json"), &prev) == nil
	if !*force && hasPrev && prev.UpdatedAt != "" {
		if last, err := time.Parse(time.RFC3339, prev.UpdatedAt); err == nil {
			if next := last.Add(*interval); now.Before(next) {
				fmt.Printf("skip: last update %s, next update %s\n",
					last.Local().Format("2006-01-02 15:04"), next.Local().Format("2006-01-02 15:04"))
				return
			}
		}
	}

	list := make([]model.Source, 0, len(sf.Sources))
	for _, s := range sf.Sources {
		if gen.Blocked(s) {
			continue
		}
		list = append(list, s)
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
			Enabled:   s.Enabled,
			State:     r.State,
			LatencyMs: r.LatencyMs,
			Vods:      r.Vods,
			CheckedAt: checkedAt,
		}
		st.Searchable = r.Vods > 0
		if s.Enabled {
			st.History = gen.AppendCheck(oldHistory[s.ID], r.State == "ok" || r.State == "limited", r.LatencyMs, now, *histLimit)
		} else {
			st.History = oldHistory[s.ID]
		}
		switch r.State {
		case "ok", "limited":
			rep.Alive++
		default:
			rep.Dead++
		}
		rep.Sources = append(rep.Sources, st)
	}
	rep.DurationMs = time.Since(start).Milliseconds()

	cfg := gen.TVBox(rep)
	mf := &model.MirrorFile{
		UpdatedAt: rep.UpdatedAt,
		File:      "api/tvbox.json",
		Mirrors:   mirror.List(*owner, *repo, *branch, "api/tvbox.json"),
	}
	idx := mirror.Index(*owner, *repo, *branch, "api/tvbox.json")

	for name, v := range map[string]any{
		"tvbox.json":  cfg,
		"status.json": rep,
		"mirror.json": mf,
		"index.json":  idx,
	} {
		if err := gen.WriteJSON(rel(name), v); err != nil {
			fmt.Printf("write %s: %v\n", name, err)
			os.Exit(1)
		}
	}

	fmt.Printf("done: %d alive / %d dead, %d sites published, %d ms\n",
		rep.Alive, rep.Dead, len(cfg.Sites), rep.DurationMs)
	next, _ := time.Parse(time.RFC3339, rep.NextUpdateAt)
	fmt.Printf("next update: %s\n", next.Local().Format("2006-01-02 15:04"))
}
