package pipeline

// flagDict maps a lowercase keyword found in a node tag to an ISO 3166-1
// alpha-2 region code. It is intentionally a shipped, curated table: operators
// never edit it. Airport-specific names that are missing here are handled by
// composing a rename step before the flag step (the pipeline runs top to
// bottom), so this dictionary only has to cover common spellings, not every
// node ever written.
//
// Matching is split by key length in init(): keys of 4+ bytes are matched as a
// substring of the tag with spaces/digits removed (so "UnitedStates",
// "Los Angeles" and "美国" all hit), while 2-3 byte keys are matched only as a
// whole word (so "US"/"UK"/"JP" never fire inside "because" or "nusa").
var flagDict = map[string]string{
	// ---- English country names ----
	"unitedstates": "US", "unitedkingdom": "GB", "greatbritain": "GB", "england": "GB",
	"canada": "CA", "australia": "AU", "germany": "DE", "france": "FR", "netherlands": "NL",
	"holland": "NL", "russia": "RU", "italy": "IT", "spain": "ES", "sweden": "SE",
	"switzerland": "CH", "austria": "AT", "belgium": "BE", "poland": "PL", "turkey": "TR",
	"brazil": "BR", "india": "IN", "thailand": "TH", "vietnam": "VN", "malaysia": "MY",
	"indonesia": "ID", "philippines": "PH", "kazakhstan": "KZ", "ukraine": "UA",
	"argentina": "AR", "mexico": "MX", "southafrica": "ZA", "emirates": "AE",
	"israel": "IL", "portugal": "PT", "denmark": "DK", "finland": "FI", "norway": "NO",
	"czech": "CZ", "greece": "GR", "hungary": "HU", "romania": "RO", "ireland": "IE",
	"iceland": "IS", "newzealand": "NZ", "chile": "CL", "colombia": "CO", "peru": "PE",
	"pakistan": "PK", "bangladesh": "BD", "nepal": "NP", "srilanka": "LK",
	"cambodia": "KH", "myanmar": "MM", "laos": "LA", "mongolia": "MN",
	"saudiarabia": "SA", "qatar": "QA", "kuwait": "KW", "oman": "OM", "jordan": "JO",
	"egypt": "EG", "morocco": "MA", "nigeria": "NG", "kenya": "KE",
	"singapore": "SG", "japan": "JP", "korea": "KR", "china": "CN", "america": "US",

	// ---- English city / hub names ----
	"losangeles": "US", "newyork": "US", "sanjose": "US", "siliconvalley": "US",
	"dallas": "US", "seattle": "US", "chicago": "US", "washington": "US", "miami": "US",
	"atlanta": "US", "oregon": "US", "portland": "US", "sanfrancisco": "US", "houston": "US",
	"denver": "US", "boston": "US", "ashburn": "US", "newjersey": "US", "california": "US",
	"tokyo": "JP", "osaka": "JP", "yokohama": "JP", "nagoya": "JP", "fukuoka": "JP",
	"seoul": "KR", "busan": "KR",
	"changi": "SG", "jurong": "SG", "tamases": "SG",
	"hongkong": "HK", "kowloon": "HK",
	"macau": "MO", "macao": "MO",
	"taipei": "TW", "taoyuan": "TW", "kaohsiung": "TW", "hsinchu": "TW", "kee lung": "TW",
	"frankfurt": "DE", "dusseldorf": "DE", "berlin": "DE", "munich": "DE", "hamburg": "DE",
	"amsterdam": "NL", "schiphol": "NL",
	"london": "GB", "manchester": "GB",
	"toronto": "CA", "vancouver": "CA", "montreal": "CA", "ottawa": "CA",
	"sydney": "AU", "melbourne": "AU", "perth": "AU", "brisbane": "AU", "adelaide": "AU",
	"paris": "FR", "marseille": "FR",
	"moscow": "RU", "saintpetersburg": "RU",
	"milan": "IT", "milano": "IT",
	"madrid": "ES", "barcelona": "ES",
	"istanbul": "TR", "saopaulo": "BR", "riodejaneiro": "BR",
	"mumbai": "IN", "delhi": "IN", "chennai": "IN", "bangalore": "IN", "kolkata": "IN",
	"bangkok": "TH", "hanoi": "VN", "hochiminh": "VN", "saigon": "VN",
	"kualalumpur": "MY", "jakarta": "ID", "bali": "ID", "manila": "PH",
	"almaty": "KZ", "astana": "KZ", "kyiv": "UA", "kiev": "UA",
	"dubai": "AE", "abudhabi": "AE", "auckland": "NZ", "ulaanbaatar": "MN",
	"beijing": "CN", "shanghai": "CN", "guangzhou": "CN", "shenzhen": "CN",

	// ---- 中文国家 / 地区 ----
	"美国": "US", "日本": "JP", "韩国": "KR", "南韩": "KR", "新加坡": "SG", "星加坡": "SG",
	"香港": "HK", "澳门": "MO", "台湾": "TW", "英国": "GB", "德国": "DE", "法国": "FR",
	"荷兰": "NL", "俄罗斯": "RU", "加拿大": "CA", "澳大利亚": "AU", "澳洲": "AU",
	"意大利": "IT", "西班牙": "ES", "瑞典": "SE", "瑞士": "CH", "奥地利": "AT",
	"比利时": "BE", "波兰": "PL", "土耳其": "TR", "巴西": "BR", "印度": "IN",
	"泰国": "TH", "越南": "VN", "马来西亚": "MY", "马来": "MY", "印尼": "ID",
	"印度尼西亚": "ID", "菲律宾": "PH", "哈萨克": "KZ", "乌克兰": "UA", "阿根廷": "AR",
	"墨西哥": "MX", "南非": "ZA", "阿联酋": "AE", "迪拜": "AE", "以色列": "IL",
	"葡萄牙": "PT", "丹麦": "DK", "芬兰": "FI", "挪威": "NO", "捷克": "CZ",
	"希腊": "GR", "匈牙利": "HU", "罗马尼亚": "RO", "爱尔兰": "IE", "冰岛": "IS",
	"新西兰": "NZ", "智利": "CL", "哥伦比亚": "CO", "秘鲁": "PE", "巴基斯坦": "PK",
	"柬埔寨": "KH", "老挝": "LA", "缅甸": "MM", "孟加拉": "BD", "尼泊尔": "NP",
	"斯里兰卡": "LK", "沙特": "SA", "卡塔尔": "QA", "科威特": "KW", "阿曼": "OM",
	"约旦": "JO", "埃及": "EG", "摩洛哥": "MA", "尼日利亚": "NG", "肯尼亚": "KE",
	"蒙古": "MN", "中国": "CN", "大陆": "CN",

	// ---- 中文城市 ----
	"东京": "JP", "大阪": "JP", "横滨": "JP", "首尔": "KR", "釜山": "KR",
	"洛杉矶": "US", "纽约": "US", "旧金山": "US", "圣何塞": "US", "硅谷": "US",
	"芝加哥": "US", "华盛顿": "US", "迈阿密": "US", "亚特兰大": "US", "俄勒冈": "US",
	"法兰克福": "DE", "柏林": "DE", "慕尼黑": "DE", "阿姆斯特丹": "NL", "伦敦": "GB",
	"多伦多": "CA", "温哥华": "CA", "蒙特利尔": "CA", "悉尼": "AU", "墨尔本": "AU",
	"巴黎": "FR", "莫斯科": "RU", "罗马": "IT", "米兰": "IT", "马德里": "ES",
	"巴塞罗那": "ES", "伊斯坦布尔": "TR", "圣保罗": "BR", "孟买": "IN", "新德里": "IN",
	"曼谷": "TH", "河内": "VN", "胡志明": "VN", "吉隆坡": "MY", "雅加达": "ID",
	"巴厘岛": "ID", "马尼拉": "PH", "阿拉木图": "KZ", "基辅": "UA", "台北": "TW",
	"台中": "TW", "高雄": "TW", "奥克兰": "NZ", "乌兰巴托": "MN", "北京": "CN",
	"上海": "CN", "广州": "CN", "深圳": "CN",

	// ---- short abbreviations (word-boundary matched) ----
	"us": "US", "usa": "US", "uk": "GB", "gb": "GB", "ca": "CA", "au": "AU",
	"de": "DE", "fr": "FR", "nl": "NL", "ru": "RU", "it": "IT", "es": "ES",
	"jp": "JP", "kr": "KR", "sg": "SG", "hk": "HK", "mo": "MO", "tw": "TW",
	"cn": "CN", "th": "TH", "vn": "VN", "my": "MY", "id": "ID", "ph": "PH",
	"tr": "TR", "br": "BR", "in": "IN", "se": "SE", "ch": "CH", "at": "AT",
	"be": "BE", "pl": "PL", "ua": "UA", "kz": "KZ", "mx": "MX", "ar": "AR",
	"za": "ZA", "ae": "AE", "il": "IL", "pt": "PT", "dk": "DK", "fi": "FI",
	"no": "NO", "cz": "CZ", "gr": "GR", "hu": "HU", "ro": "RO", "ie": "IE",
	"is": "IS", "nz": "NZ", "cl": "CL", "co": "CO", "pe": "PE", "pk": "PK",
	"kh": "KH", "mm": "MM", "la": "US", "mn": "MN", "sa": "SA", "qa": "QA",
	"kw": "KW", "eg": "EG", "ke": "KE", "ng": "NG", "bd": "BD", "np": "NP",
	"lk": "LK", "jo": "JO", "om": "OM", "ma": "MA",
}

// flagLong holds keys matched as a substring of the compacted tag; flagAbbr
// holds short keys matched only as a whole word.
var (
	flagLong []flagEntry
	flagAbbr map[string]string
)

type flagEntry struct {
	key  string
	code string
}

func init() {
	flagAbbr = make(map[string]string)
	for k, code := range flagDict {
		norm := normalizeFlagKey(k)
		if len(norm) <= 3 {
			flagAbbr[norm] = code
			continue
		}
		flagLong = append(flagLong, flagEntry{key: norm, code: code})
	}
	// Longest key first so "unitedstates" wins over a shorter overlapping key.
	for i := 1; i < len(flagLong); i++ {
		for j := i; j > 0 && len(flagLong[j].key) > len(flagLong[j-1].key); j-- {
			flagLong[j], flagLong[j-1] = flagLong[j-1], flagLong[j]
		}
	}
}
