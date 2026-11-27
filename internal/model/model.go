package model

// Source is one editable video source entry stored in sources/sources.json.
type Source struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	API     string `json:"api"`
	Group   string `json:"group"`
	Note    string `json:"note,omitempty"`
	Enabled bool   `json:"enabled"`
	AddedAt string `json:"added_at,omitempty"`
}

type SourceFile struct {
	Version   int      `json:"version"`
	UpdatedAt string   `json:"updated_at,omitempty"`
	Sources   []Source `json:"sources"`
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
	Dead          int      `json:"dead"`
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
	LatencyMs int    `json:"latency_ms"`
	Vods      int    `json:"vods"`
	CheckedAt string `json:"checked_at"`
}

type Mirror struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Kind string `json:"kind"` // primary | backup | proxy
}

type MirrorFile struct {
	UpdatedAt string   `json:"updated_at"`
	File      string   `json:"file"`
	Mirrors   []Mirror `json:"mirrors"`
}

type IndexEntry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Index struct {
	Name string       `json:"name"`
	Urls []IndexEntry `json:"urls"`
}
