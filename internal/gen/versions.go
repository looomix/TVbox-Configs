package gen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/looomix/tvbox-configs/internal/model"
)

// SnapshotDir is where the per-run version files live, relative to the repo root.
const SnapshotDir = "versions"

// KeepSnapshots is how many version files are kept around for rollback.
const KeepSnapshots = 14

// BuildSnapshot turns the published site list into a rollback-ready record.
func BuildSnapshot(date, updatedAt string, total, alive int, sites []model.Site) *model.Snapshot {
	out := &model.Snapshot{Date: date, UpdatedAt: updatedAt, Total: total, Alive: alive}
	for _, s := range sites {
		out.Sites = append(out.Sites, model.SnapshotSite{Key: s.Key, Name: s.Name, API: s.API, Group: s.Group})
	}
	return out
}

// WriteSnapshot stores versions/<date>.json and refreshes versions/index.json,
// pruning everything older than KeepSnapshots entries.
func WriteSnapshot(dir string, snap *model.Snapshot) (model.VersionRow, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return model.VersionRow{}, err
	}
	if err := WriteJSON(filepath.Join(dir, snap.Date+".json"), snap); err != nil {
		return model.VersionRow{}, err
	}

	var idx model.VersionIndex
	_ = ReadJSON(filepath.Join(dir, "index.json"), &idx)

	row := model.VersionRow{Date: snap.Date, Alive: snap.Alive, Total: snap.Total}
	// diff against the newest snapshot that is not this one
	var prev *model.Snapshot
	for _, r := range idx.Versions {
		if r.Date == snap.Date {
			continue
		}
		var old model.Snapshot
		if err := ReadJSON(filepath.Join(dir, r.Date+".json"), &old); err == nil {
			prev = &old
			break
		}
	}
	if prev != nil {
		now := map[string]bool{}
		for _, s := range snap.Sites {
			now[s.Name] = true
		}
		before := map[string]bool{}
		for _, s := range prev.Sites {
			before[s.Name] = true
			if !now[s.Name] {
				row.Removed = append(row.Removed, s.Name)
			}
		}
		for _, s := range snap.Sites {
			if !before[s.Name] {
				row.Added = append(row.Added, s.Name)
			}
		}
	}

	rows := make([]model.VersionRow, 0, len(idx.Versions)+1)
	for _, r := range idx.Versions {
		if r.Date != snap.Date {
			rows = append(rows, r)
		}
	}
	rows = append(rows, row)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Date > rows[j].Date })
	if len(rows) > KeepSnapshots {
		for _, drop := range rows[KeepSnapshots:] {
			_ = os.Remove(filepath.Join(dir, drop.Date+".json"))
		}
		rows = rows[:KeepSnapshots]
	}
	idx.Versions = rows
	if err := WriteJSON(filepath.Join(dir, "index.json"), idx); err != nil {
		return row, err
	}
	return row, nil
}

// LoadSnapshot reads one dated snapshot for rollback.
func LoadSnapshot(dir, date string) (*model.Snapshot, error) {
	date = strings.TrimSpace(date)
	if date == "" {
		return nil, fmt.Errorf("empty snapshot date")
	}
	var snap model.Snapshot
	if err := ReadJSON(filepath.Join(dir, date+".json"), &snap); err != nil {
		return nil, err
	}
	if len(snap.Sites) == 0 {
		return nil, fmt.Errorf("snapshot %s has no sites", date)
	}
	return &snap, nil
}

// SitesOfSnapshot converts a snapshot back into the published site list.
func SitesOfSnapshot(snap *model.Snapshot) []model.Site {
	used := map[string]bool{}
	out := make([]model.Site, 0, len(snap.Sites))
	for _, s := range snap.Sites {
		group := s.Group
		if group == "" {
			group = "综合"
		}
		out = append(out, model.Site{
			Key:         keyOf(s.Key, s.Name, s.API, used, false),
			Name:        s.Name,
			Type:        3,
			API:         s.API,
			Searchable:  1,
			QuickSearch: 1,
			Filterable:  1,
			Group:       groupOf(group),
		})
	}
	return out
}
