package gen

import (
	"strings"

	"github.com/looomix/tvbox-configs/internal/model"
)

// The public artifacts should stay usable on every client. A number of public
// CMS lists contain adult-only stations; some TVBox builds hide or reject them,
// which makes a config look empty even though it was loaded correctly.
// Those stations are dropped by default. Use -allow-nsfw to opt in locally.
var adultNameWords = []string{
	"黄色", "色仓库", "色X", "X色", "老色", "番号", "杏吧", "nsfw",
	"白嫖", "a-v", "av女", "av成人", "麻豆", "小鸡", "新极品", "精品x",
	"大x子", "美少女", "奶香香", "黑料", "清水源", "香系源", "樱花",
	"成人", "福利", "露点", "gav", "nii", "xingba", "souav", "kxgav",
}

var adultHostWords = []string{
	"hsckzy", "apilsbzy", "fhapi9", "xingba", "kxgav", "pgxdy",
	"souavzy", "91md.", "xiaojizy", "jipinvip", "jingpinx", "gdlsp",
	"msnii", "apidanaizi", "naixxzy", "heiliaozy", "xrbsp", "yhzy",
}

func adultText(name, api string) bool {
	label := strings.ToLower(name)
	for _, w := range adultNameWords {
		if strings.Contains(label, strings.ToLower(w)) {
			return true
		}
	}
	host := strings.ToLower(api)
	for _, w := range adultHostWords {
		if strings.Contains(host, w) {
			return true
		}
	}
	return false
}

// NSFW reports whether one source is labelled as an adult-only station.
func NSFW(s model.Source) bool { return adultText(s.Name, s.API) }

// FilterNSFW removes adult-only stations, keeping the order and returning the
// number of entries that were dropped.
func FilterNSFW(list []model.Source) ([]model.Source, int) {
	out := make([]model.Source, 0, len(list))
	for _, s := range list {
		if NSFW(s) {
			continue
		}
		out = append(out, s)
	}
	return out, len(list) - len(out)
}
