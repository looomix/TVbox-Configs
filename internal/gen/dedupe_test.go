package gen

import (
	"testing"

	"github.com/looomix/tvbox-configs/internal/model"
)

func TestNormalizeAPI(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"https://www.360zy.com/api.php/provide/vod", "360zy.com/provide/vod"},
		{"https://360zy.com/api.php/provide/vod/at/xml", "360zy.com/provide/vod"},
		{"https://360zy.com/API.php/provide/vod/", "360zy.com/provide/vod"},
		{"http://360zy.com:8080/api.php/provide/vod?ac=detail", "360zy.com/provide/vod"},
		{"https://360zy.com/api.php/vod2", "360zy.com/vod2"},
		{"https://360zy.com/inc/api.php", "360zy.com/inc"},
		{"https://360zy.com/", "360zy.com"},
		{"https://proxy.example.com/api?url=https://a.com/api.php/provide/vod", "proxy.example.com|a.com/provide/vod"},
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizeAPI(c.raw); got != c.want {
			t.Errorf("NormalizeAPI(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestDedupeKeepsSameNameDifferentEndpoints(t *testing.T) {
	in := []model.Source{
		{Name: "无尽资源", API: "https://a.example.com/api.php/provide/vod"},
		{Name: "无尽资源", API: "https://b.example.com/api.php/provide/vod"},
		{Name: "无尽资源", API: "https://a.example.com/api.php/provide/vod/at/xml"},
	}
	out, removed := Dedupe(in)
	if len(out) != 2 || removed != 1 {
		t.Fatalf("kept %d removed %d, want 2/1", len(out), removed)
	}
	if out[0].Name == out[1].Name {
		t.Errorf("display names must stay unique, got %q twice", out[0].Name)
	}
}
