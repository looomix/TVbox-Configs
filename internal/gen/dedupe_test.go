package gen

import "testing"

func TestNormalizeAPI(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"https://www.360zy.com/api.php/provide/vod", "360zy.com/api.php/provide/vod"},
		{"https://360zy.com/api.php/provide/vod/at/xml", "360zy.com/api.php/provide/vod"},
		{"https://360zy.com/API.php/provide/vod/", "360zy.com/api.php/provide/vod"},
		{"http://360zy.com:8080/api.php/provide/vod?ac=detail", "360zy.com/api.php/provide/vod"},
		{"https://360zy.com/api.php/vod2", "360zy.com/api.php/vod2"},
		{"https://360zy.com/inc/api.php", "360zy.com/inc/api.php"},
		{"https://360zy.com/", "360zy.com"},
		{"https://proxy.example.com/api?url=https://a.com/api.php/provide/vod", "proxy.example.com|a.com/api.php/provide/vod"},
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizeAPI(c.raw); got != c.want {
			t.Errorf("NormalizeAPI(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestNameKeyMergesMirrors(t *testing.T) {
	same := []string{"无尽资源", "无尽资源(api.wujin.example)", "无尽资源3", "无尽资源 采集站", "1无尽资源"}
	base := NameKey(same[0])
	for _, n := range same[1:] {
		if NameKey(n) != base {
			t.Errorf("NameKey(%q) = %q, want %q", n, NameKey(n), base)
		}
	}
	if NameKey("无尽资源") == NameKey("非凡资源") {
		t.Error("different stations must not collapse")
	}
}
