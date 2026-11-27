package gen

import (
	"strings"

	"github.com/looomix/tvbox-configs/internal/model"
)

// Blocker keeps adult and otherwise disallowed sites out of every artifact.
type Blocker struct {
	domains []string
	words   []string
}

func NewBlocker(bl model.Blocklist) *Blocker {
	b := &Blocker{}
	for _, d := range bl.Domains {
		d = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(d, "www.")))
		if d != "" {
			b.domains = append(b.domains, d)
		}
	}
	for _, w := range bl.Words {
		w = strings.ToLower(strings.TrimSpace(w))
		if w != "" {
			b.words = append(b.words, w)
		}
	}
	return b
}

// Match reports whether a source must never reach the published artifacts.
func (b *Blocker) Match(s model.Source) bool {
	host := HostOf(s.API)
	for _, d := range b.domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	hay := strings.ToLower(s.Name + " " + s.ID + " " + host + " " + s.API)
	for _, w := range b.words {
		if strings.Contains(hay, w) {
			return true
		}
	}
	return false
}
