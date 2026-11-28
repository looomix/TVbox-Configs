package model

// Source is one editable video source entry stored in sources/sources.json.
type Source struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	API     string `json:"api"`
	Group   string `json:"group"`
	Note    string `json:"note,omitempty"`
	Enabled bool   `json:"enabled"`
	Origin  string `json:"origin,omitempty"` // manual | feed
	AddedAt string `json:"added_at,omitempty"`
}

type SourceFile struct {
	Version int      `json:"version"`
	Sources []Source `json:"sources"`
	// Removed keeps user deletions alive: feeds must never re-add these keys.
	Removed []string `json:"removed,omitempty"`
}

// Check is one point of history for a source.
type Check struct {
	T  string `json:"t"`
	OK bool   `json:"ok"`
	Ms int    `json:"ms"`
}

// Status is the per-source health record written to api/status.json.
type Status struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	API        string  `json:"api"`
	Group      string  `json:"group"`
	Note       string  `json:"note,omitempty"`
	Origin     string  `json:"origin,omitempty"`
	Enabled    bool    `json:"enabled"`
	State      string  `json:"state"` // ok | limited | down | off
	LatencyMs  int     `json:"latency_ms"`
	Searchable bool    `json:"searchable"`
	Vods       int     `json:"vods"`
	CheckedAt  string  `json:"checked_at"`
	History    []Check `json:"history"`
}

// Report is the whole api/status.json document.
type Report struct {
	UpdatedAt     string   `json:"updated_at"`
	NextUpdateAt  string   `json:"next_update_at"`
	IntervalHours int      `json:"interval_hours"`
	Total         int      `json:"total"`
	Alive         int      `json:"alive"`
	Unverified    int      `json:"unverified"`
	Dead          int      `json:"dead"`
	FromFeed      int      `json:"from_feed"`
	Deduped       int      `json:"deduped"`
	Filtered      int      `json:"filtered"` // adult-only stations excluded by default
	DurationMs    int64    `json:"duration_ms"`
	Sources       []Status `json:"sources"`
}

// Site is one TVBox site entry.
type Site struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Type        int    `json:"type"`
	API         string `json:"api"`
	Searchable  int    `json:"searchable"`
	QuickSearch int    `json:"quickSearch"`
	Filterable  int    `json:"filterable"`
	Ext         string `json:"ext"`
	Group       string `json:"group"`
}

// TVBox is a single-repository config consumable by TVBox / TVBox-like clients.
type TVBox struct {
	Spider  string   `json:"spider"`
	Sites   []Site   `json:"sites"`
	Lives   []any    `json:"lives"`
	Parses  []any    `json:"parses"`
	Flags   []string `json:"flags"`
	Version string   `json:"version"`
}

// ProbeResult is the single-source result written by the cloud probe workflow.
type ProbeResult struct {
	URL       string `json:"url"`
	State     string `json:"state"`
	Vods      int    `json:"vods"`
	LatencyMs int    `json:"latency_ms"`
	CheckedAt string `json:"checked_at"`
}

type Mirror struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Kind string `json:"kind"` // primary | backup | proxy
}

type MirrorFile struct {
	UpdatedAt string        `json:"updated_at"`
	Files     []MirrorGroup `json:"files"`
}

type IndexEntry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Index struct {
	Name string       `json:"name"`
	Urls []IndexEntry `json:"urls"`
}

// Feed is one remote source list that is merged into sources/sources.json.
type Feed struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

type FeedFile struct {
	Version int    `json:"version"`
	Feeds   []Feed `json:"feeds"`
}

// MoonSite is one entry of the MoonTV / LunaTV api_site object.
type MoonSite struct {
	Name string `json:"name"`
	API  string `json:"api"`
}

// MoonConfig is the MoonTV config.json shape, importable in its 视频源配置 page.
type MoonConfig struct {
	CacheTime int                 `json:"cache_time"`
	APISite   map[string]MoonSite `json:"api_site"`
}

// MirrorGroup lists the mirrors of one published artifact.
type MirrorGroup struct {
	File    string   `json:"file"`
	Label   string   `json:"label"`
	Kind    string   `json:"kind"` // config | text
	Primary string   `json:"primary"`
	Mirrors []Mirror `json:"mirrors"`
}
