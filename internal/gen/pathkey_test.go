package gen

import "testing"

func TestPathKeyCollapsesCMSVariants(t *testing.T) {
	cases := [][2]string{
		{"https://jyzyapi.com/provide/vod/from/jinyingm3u8", "jyzyapi.com/provide/vod"},
		{"https://jyzyapi.com/provide/vod/from/jinyingyun/at/json", "jyzyapi.com/provide/vod"},
		{"https://jyzyapi.com/provide/vod", "jyzyapi.com/provide/vod"},
		{"https://jyzyapi.com/api.php/provide/vod", "jyzyapi.com/provide/vod"},
		{"https://jyzyapi.com/api.php/provide/vod/at/xml", "jyzyapi.com/provide/vod"},
		{"https://a.com/json/api.php/provide/vod/at/json", "a.com/provide/vod"},
	}
	for _, c := range cases {
		if got := NormalizeAPI(c[0]); got != c[1] {
			t.Fatalf("%s -> %s want %s", c[0], got, c[1])
		}
	}
	// different libraries on one host must stay apart
	if NormalizeAPI("https://a.com/provide/vod/tv") == NormalizeAPI("https://a.com/provide/vod/movie") {
		t.Fatal("distinct paths merged")
	}
}
