package hooks

import (
	"encoding/json"
	settings2 "github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v2"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

type LanguageContainer struct {
	attribute AttributeContainer
	rtl       []string
	lang      map[string][]string
}

type AttributeContainer struct {
	nativeName int
}

var langs = LanguageContainer{
	attribute: AttributeContainer{nativeName: 0},
	rtl:       []string{"ar", "dv", "fa", "ha", "he", "ks", "ku", "ps", "ur", "yi"},
	lang: map[string][]string{
		"sxu":          {"Säggssch"},
		"rtm":          {"Faeag Rotuma"},
		"wls":          {"Faka'uvea"},
		"twd":          {"Tweants"},
		"trp":          {"Kokborok (Tripuri)"},
		"pko":          {"Pökoot"},
		"pru":          {"Prūsiskan"},
		"test":         {"Test (site admin only)"},
		"swb":          {"Shikomoro"},
		"njo":          {"Ao"},
		"mni":          {"মেইতেই লোন্"},
		"ttt":          {"Tati"},
		"yrl":          {"ñe'engatú"},
		"krl":          {"Karjala"},
		"mwv":          {"Behase Mentawei"},
		"niu":          {"ko e vagahau Niuē"},
		"bew":          {"Bahasa Betawi"},
		"rw":           {"Kinyarwanda"},
		"slr":          {"Salırça"},
		"ryu":          {"ʔucināguci"},
		"gom":          {"कोंकणी/Konknni "},
		"gom-deva":     {"कोंकणी"},
		"gom-latn":     {"Konknni"},
		"akz":          {"Albaamo innaaɬiilka"},
		"kgp":          {"Kaingáng"},
		"hu-formal":    {"Magyar (magázó)"},
		"kea":          {"Kabuverdianu"},
		"ady":          {"Адыгэбзэ / Adygabze"},
		"ady-cyrl":     {"Адыгэбзэ"},
		"tsd":          {"Τσακωνικά"},
		"arq":          {"Dziri"},
		"gcf":          {"Guadeloupean Creole French"},
		"lld":          {"Ladin"},
		"ruq-grek":     {"Megleno-Romanian (Greek script)"},
		"ydd":          {"Eastern Yiddish"},
		"tzm":          {"ⵜⴰⵎⴰⵣⵉⵖⵜ"},
		"bto":          {"Iriga Bicolano"},
		"rap":          {"arero rapa nui"},
		"bfq":          {"படகா"},
		"guc":          {"Wayúu"},
		"mui":          {"Musi"},
		"kbd-latn":     {"Qabardjajəbza"},
		"ase":          {"American sign language"},
		"es-419":       {"español de America Latina"},
		"mnc":          {"ᠮᠠᠨᠵᡠ ᡤᡳᠰᡠᠨ"},
		"aro":          {"Araona"},
		"hif-deva":     {"फ़ीजी हिन्दी"},
		"gah":          {"Alekano"},
		"rki":          {"ရခိုင်"},
		"es-formal":    {"español (formal)"},
		"nqo":          {"ߒߞߏ"},
		"gbz":          {"Dari"},
		"gur":          {"Gurenɛ"},
		"yrk":          {"Ненэцяʼ вада"},
		"esu":          {"Yup'ik"},
		"saz":          {"ꢱꣃꢬꢵꢯ꣄ꢡ꣄ꢬꢵ"},
		"hsn":          {"湘语"},
		"yua":          {"Maaya T'aan"},
		"tkr":          {"ЦӀаьхна миз"},
		"aeb":          {"   زَوُن"},
		"pis":          {"Pijin"},
		"ppl":          {"Nawat"},
		"shn":          {"လိၵ်ႈတႆး"},
		"bbc":          {"Batak Toba/Batak autonym unknown"},
		"bbc-latn":     {"Batak Toba"},
		"mfe":          {"Morisyen"},
		"ksf":          {"Bafia"},
		"hne":          {"छत्तीसगढ़ी"},
		"sly":          {"Bahasa Selayar"},
		"ahr":          {"अहिराणी"},
		"mic":          {"Mi'kmaq"},
		"mnw":          {"ဘာသာ မန်"},
		"rut":          {"мыхIабишды чIел"},
		"acf":          {"Saint Lucian Creole French"},
		"azb":          {"تورکجه"},
		"izh":          {"ižoran keel"},
		"ban":          {"ᬩᬲᬩᬮᬶ"},
		"nl-be":        {"nl-be"},
		"qqq":          {"Message documentation"},
		"ike":          {"ᐃᓄᒃᑎᑐᑦ/inuktitut"},
		"aa":           {"Qafár af"},
		"ab":           {"Аҧсшәа"},
		"ace":          {"Acèh"},
		"af":           {"Afrikaans"},
		"ak":           {"Akan"},
		"aln":          {"Gegë"},
		"als":          {"Tosk"},
		"am":           {"አማርኛ"},
		"an":           {"aragonés"},
		"ang":          {"Ænglisc"},
		"anp":          {"अङ्गिका"},
		"ar":           {"العربية"},
		"arc":          {"ܐܪܡܝܐ"},
		"arn":          {"mapudungun"},
		"ary":          {"Maġribi"},
		"arz":          {"مصرى"},
		"as":           {"অসমীয়া"},
		"ast":          {"asturianu"},
		"av":           {"авар"},
		"avk":          {"Kotava"},
		"ay":           {"Aymar aru"},
		"az":           {"azərbaycanca"},
		"ba":           {"башҡортса"},
		"bar":          {"Boarisch"},
		"bat-smg":      {"žemaitėška"},
		"bcc":          {"بلوچی مکرانی"},
		"bcl":          {"Bikol Central"},
		"be":           {"беларуская"},
		"be-tarask":    {"беларуская (тарашкевіца)‎"},
		"be-x-old":     {"беларуская (тарашкевіца)‎"},
		"bg":           {"български"},
		"bh":           {"भोजपुरी"},
		"bho":          {"भोजपुरी"},
		"bi":           {"Bislama"},
		"bjn":          {"Bahasa Banjar"},
		"bm":           {"bamanankan"},
		"bn":           {"বাংলা"},
		"bo":           {"བོད་ཡིག"},
		"bpy":          {"বিষ্ণুপ্রিয়া মণিপুরী"},
		"bqi":          {"بختياري"},
		"br":           {"brezhoneg"},
		"brh":          {"Bráhuí"},
		"bs":           {"bosanski"},
		"bug":          {"ᨅᨔ ᨕᨘᨁᨗ"},
		"bxr":          {"буряад"},
		"ca":           {"català"},
		"cbk-zam":      {"Chavacano de Zamboanga"},
		"cdo":          {"Mìng-dĕ̤ng-ngṳ̄"},
		"ce":           {"нохчийн"},
		"ceb":          {"Cebuano"},
		"ch":           {"Chamoru"},
		"cho":          {"Choctaw"},
		"chr":          {"ᏣᎳᎩ"},
		"chy":          {"Tsetsêhestâhese"},
		"ckb":          {"کوردی"},
		"co":           {"corsu"},
		"cps":          {"Capiceño"},
		"cr":           {"Nēhiyawēwin / ᓀᐦᐃᔭᐍᐏᐣ"},
		"crh":          {"qırımtatarca"},
		"crh-latn":     {"qırımtatarca (Latin)‎"},
		"crh-cyrl":     {"къырымтатарджа (Кирилл)‎"},
		"cs":           {"česky"},
		"csb":          {"kaszëbsczi"},
		"cu":           {"словѣ́ньскъ / ⰔⰎⰑⰂⰡⰐⰠⰔⰍⰟ"},
		"cv":           {"Чӑвашла"},
		"cy":           {"Cymraeg"},
		"da":           {"dansk"},
		"de":           {"Deutsch"},
		"de-at":        {"Österreichisches Deutsch"},
		"de-ch":        {"Schweizer Hochdeutsch"},
		"de-formal":    {"Deutsch (Sie-Form)‎"},
		"diq":          {"Zazaki"},
		"dsb":          {"dolnoserbski"},
		"dtp":          {"Dusun Bundu-liwan"},
		"dv":           {"ދިވެހިބަސް"},
		"dz":           {"ཇོང་ཁ"},
		"ee":           {"eʋegbe"},
		"egl":          {"Emiliàn"},
		"el":           {"Ελληνικά"},
		"eml":          {"emiliàn e rumagnòl"},
		"en":           {"English"},
		"en-ca":        {"Canadian English"},
		"en-gb":        {"British English"},
		"eo":           {"Esperanto"},
		"es":           {"español"},
		"et":           {"eesti"},
		"eu":           {"euskara"},
		"ext":          {"estremeñu"},
		"fa":           {"فارسی"},
		"ff":           {"Fulfulde"},
		"fi":           {"suomi"},
		"fit":          {"meänkieli"},
		"fiu-vro":      {"Võro"},
		"fj":           {"Na Vosa Vakaviti"},
		"fo":           {"føroyskt"},
		"fr":           {"français"},
		"frc":          {"français cadien"},
		"frp":          {"arpetan"},
		"frr":          {"Nordfriisk"},
		"fur":          {"furlan"},
		"fy":           {"Frysk"},
		"ga":           {"Gaeilge"},
		"gag":          {"Gagauz"},
		"gan":          {"贛語"},
		"gan-hans":     {"赣语（简体）‎"},
		"gan-hant":     {"贛語（繁體）‎"},
		"gd":           {"Gàidhlig"},
		"gl":           {"galego"},
		"glk":          {"گیلکی"},
		"gn":           {"Avañe'ẽ"},
		"got":          {"Gothic"},
		"grc":          {"Ἀρχαία ἑλληνικὴ"},
		"gsw":          {"Alemannisch"},
		"gu":           {"ગુજરાતી"},
		"gv":           {"Gaelg"},
		"ha":           {"Hausa"},
		"hak":          {"Hak-kâ-fa"},
		"haw":          {"Hawai`i"},
		"he":           {"עברית"},
		"hi":           {"हिन्दी"},
		"hif":          {"Fiji Hindi"},
		"hif-latn":     {"Fiji Hindi"},
		"hil":          {"Ilonggo"},
		"ho":           {"Hiri Motu"},
		"hr":           {"hrvatski"},
		"hsb":          {"hornjoserbsce"},
		"ht":           {"Kreyòl ayisyen"},
		"hu":           {"magyar"},
		"hy":           {"Հայերեն"},
		"hz":           {"Otsiherero"},
		"ia":           {"Interlingua"},
		"id":           {"Bahasa Indonesia"},
		"ie":           {"Interlingue"},
		"ig":           {"Igbo"},
		"ii":           {"ꆇꉙ / 四川彝语"},
		"ik":           {"Iñupiaq"},
		"ilo":          {"Ilokano"},
		"inh":          {"гӀалгӀай"},
		"io":           {"Ido"},
		"is":           {"íslenska"},
		"it":           {"italiano"},
		"iu":           {"ᐃᓄᒃᑎᑐᑦ / Inuktitut"},
		"ja":           {"日本語"},
		"jam":          {"Patois"},
		"jbo":          {"la lojban."},
		"jv":           {"ꦧꦱꦗꦮ / Basa Jawa"},
		"ka":           {"ქართული"},
		"kaa":          {"Qaraqalpaqsha"},
		"kab":          {"Taqbaylit"},
		"kbd":          {"къэбэрдеибзэ"},
		"kbp":          {"Kabɩyɛ"},
		"kcg":          {"Tyap"},
		"kg":           {"Kikongo"},
		"ki":           {"Gĩkũyũ"},
		"kj":           {"Kuanyama"},
		"kk":           {"қазақша"},
		"kl":           {"kalaallisut"},
		"km":           {"ភាសាខ្មែរ"},
		"kn":           {"ಕನ್ನಡ"},
		"ko":           {"한국어"},
		"koi":          {"перем коми"},
		"kr":           {"Kanuri"},
		"krc":          {"къарачай-малкъар"},
		"ks":           {"कॉशुर / كٲشُر"},
		"ksh":          {"Kölsch"},
		"ku":           {"kurdî"},
		"kv":           {"коми кыв"},
		"kw":           {"Kernewek"},
		"ky":           {"Кыргызча"},
		"la":           {"latine"},
		"lad":          {"Ladino"},
		"lb":           {"Lëtzebuergesch"},
		"lbe":          {"лакку"},
		"lez":          {"лезги"},
		"lg":           {"Luganda"},
		"li":           {"Limburgs"},
		"lij":          {"Lìgure"},
		"lmo":          {"Lombard"},
		"ln":           {"lingála"},
		"lo":           {"ພາສາລາວ"},
		"lrc":          {"لوری شمالی"},
		"lt":           {"lietuvių"},
		"ltg":          {"latgalīšu"},
		"lu":           {"Tshiluba"},
		"lua":          {"Tshiluba"},
		"lui":          {"Luiseño"},
		"lun":          {"Lunda"},
		"luo":          {"Dholuo"},
		"lus":          {"Mizo ṭawng"},
		"luy":          {"Luhya"},
		"lv":           {"latviešu"},
		"mad":          {"Madhura"},
		"mai":          {"मैथिली"},
		"mak":          {"Bahasa Makassar"},
		"man":          {"Mande"},
		"map-bms":      {"Basa Banyumasan"},
		"mas":          {"Maa"},
		"mdf":          {"мокшень"},
		"mg":           {"Malagasy"},
		"mh":           {"Kajin M̧ajeļ"},
		"mhr":          {"олык марий"},
		"mi":           {"te reo Māori"},
		"min":          {"Baso Minangkabau"},
		"mk":           {"македонски"},
		"ml":           {"മലയാളം"},
		"mn":           {"монгол"},
		"mo":           {"молдовеняскэ"},
		"mr":           {"मराठी"},
		"mrj":          {"малӹрӹм"},
		"ms":           {"Bahasa Melayu"},
		"mt":           {"Malti"},
		"mus":          {"Mvskoke"},
		"mwl":          {"Mirandês"},
		"my":           {"ဗမာစာ"},
		"myv":          {"эрзянь"},
		"mzn":          {"مازِرونی"},
		"na":           {"Dorerin Naoero"},
		"nah":          {"Nāhuatl"},
		"nap":          {"Nnapulitano"},
		"nds":          {"Plattdüütsch"},
		"nds-nl":       {"Nedersaksies"},
		"ne":           {"नेपाली"},
		"new":          {"नेपाल भाषा"},
		"ng":           {"Oshiwambo"},
		"nia":          {"Li Niha"},
		"nl":           {"Nederlands"},
		"nl-informal":  {"Nederlands (informeel)"},
		"nn":           {"nynorsk"},
		"no":           {"norsk"},
		"nov":          {"Novial"},
		"nr":           {"isiNdebele"},
		"nso":          {"Sesotho sa Leboa"},
		"nus":          {"Thok Nath"},
		"nv":           {"Diné bizaad"},
		"ny":           {"chiCheŵa"},
		"nys":          {"Noongar"},
		"oc":           {"occitan"},
		"olo":          {"Livvi"},
		"om":           {"Afaan Oromoo"},
		"or":           {"ଓଡ଼ିଆ"},
		"os":           {"ирон æвзаг"},
		"pa":           {"ਪੰਜਾਬੀ / پنجابی"},
		"pag":          {"Pangasinan"},
		"pam":          {"Kapampangan"},
		"pap":          {"Papiamentu"},
		"pau":          {"Palauan"},
		"pdc":          {"Deitsch"},
		"pfl":          {"Pfälzisch"},
		"pi":           {"पाऴि"},
		"pih":          {"Norfolk"},
		"pl":           {"polski"},
		"pms":          {"Piemontèis"},
		"pnt":          {"ποντιακά"},
		"pon":          {"Pohnpeian"},
		"prg":          {"Prūsiskan"},
		"ps":           {"پښتو"},
		"pt":           {"português"},
		"pt-br":        {"português do Brasil"},
		"qu":           {"Runa Simi"},
		"qug":          {"Quichua de Chimborazo"},
		"raj":          {"राजस्थानी"},
		"rar":          {"Cook Islands Māori"},
		"rgn":          {"Romagnol"},
		"rif":          {"Tarifit"},
		"rm":           {"rumantsch"},
		"rmy":          {"Romani"},
		"rn":           {"Ikirundi"},
		"ro":           {"română"},
		"roa-rup":      {"armãneashce"},
		"roa-tara":     {"tarandíne"},
		"ru":           {"русский"},
		"rue":          {"руськый язык"},
		"sa":           {"संस्कृतम्"},
		"sah":          {"саха тыла"},
		"sat":          {"ᱥᱟᱱᱛᱟᱲᱤ"},
		"sc":           {"sardu"},
		"scn":          {"sicilianu"},
		"sco":          {"Scots"},
		"sd":           {"سنڌي"},
		"se":           {"davvisámegiella"},
		"sg":           {"Sängö"},
		"sh":           {"srpskohrvatski"},
		"shi":          {"ⵜⴰⵛⵍⵃⵉⵜ"},
		"si":           {"සිංහල"},
		"sk":           {"slovenčina"},
		"sl":           {"slovenščina"},
		"sm":           {"gagana fa'a Samoa"},
		"sn":           {"chiShona"},
		"so":           {"Soomaaliga"},
		"sq":           {"shqip"},
		"sr":           {"српски"},
		"srn":          {"Sranantongo"},
		"ss":           {"SiSwati"},
		"st":           {"Sesotho"},
		"stq":          {"Saterfriesisch"},
		"su":           {"Basa Sunda"},
		"sv":           {"svenska"},
		"sw":           {"Kiswahili"},
		"szl":          {"ślōnskŏ gŏdka"},
		"ta":           {"தமிழ்"},
		"tcy":          {"Tulu"},
		"te":           {"తెలుగు"},
		"tet":          {"Tetun"},
		"tg":           {"тоҷикӣ"},
		"th":           {"ไทย"},
		"ti":           {"ትግርኛ"},
		"tk":           {"Türkmençe"},
		"tl":           {"Tagalog"},
		"tpi":          {"Tok Pisin"},
		"tn":           {"Setswana"},
		"to":           {"faka Tonga"},
		"tr":           {"Türkçe"},
		"ts":           {"Xitsonga"},
		"tt":           {"татарча"},
		"tum":          {"chiTumbuka"},
		"tw":           {"Twi"},
		"ty":           {"Reo Tahiti"},
		"tyv":          {"Тыва дыл"},
		"udm":          {"удмурт кыл"},
		"ug":           {"ئۇيغۇرچە"},
		"uk":           {"українська"},
		"ur":           {"اردو"},
		"uz":           {"oʻzbekcha"},
		"ve":           {"Tshivenḓa"},
		"vec":          {"vèneto"},
		"vep":          {"vepsän kel’"},
		"vi":           {"Tiếng Việt"},
		"vls":          {"West-Vlams"},
		"vmf":          {"Mainfränkisch"},
		"vo":           {"Volapük"},
		"vot":          {"Vaďďa"},
		"vro":          {"võro"},
		"wa":           {"walon"},
		"war":          {"Winaray"},
		"wo":           {"Wolof"},
		"wuu":          {"吴语"},
		"xal":          {"Хальмг"},
		"xh":           {"isiXhosa"},
		"xmf":          {"მარგალური"},
		"xsy":          {"Saisiyat"},
		"yi":           {"ייִדיש"},
		"yo":           {"Yorùbá"},
		"yue":          {"粵語"},
		"za":           {"Sawndip"},
		"zea":          {"Zeêuws"},
		"zh":           {"中文"},
		"zh-classical": {"文言"},
		"zh-min-nan":   {"Bân-lâm-gú"},
		"zh-yue":       {"粵語"},
		"zu":           {"isiZulu"},
	}}

func IsValid(langcode string) bool {
	_, ok := langs.lang[langcode]
	return ok
}

type LanguageInfo struct {
	LanguageCode string
	Direction    string
	Attribute    string
}

func getLanguageInfo(languageCode string) LanguageInfo {
	var langInfo = LanguageInfo{
		LanguageCode: languageCode,
		Direction:    "ltr",
		Attribute:    "",
	}

	if IsValid(languageCode) {
		if slices.Contains(langs.rtl, languageCode) {
			langInfo.Direction = "rtl"
		}
		if langs.attribute.nativeName == 1 {
			langInfo.Attribute = langs.lang[languageCode][0]
		}
	}
	return langInfo
}

func generateLocaleIndex(locales Locales) Locales {
	var localeIndex = Locales{}
	for langcode := range locales {
		if langcode != "en" {
			localeIndex[langcode] = "locales/" + langcode + ".json"
		} else {
			localeIndex[langcode] = locales[langcode]
		}
	}
	return localeIndex
}

var AvailableLangs = map[string]LanguageInfo{}

func ExpressPreSession(app *fiber.App) {
	var locales = getAllLocales()
	var localeIndex = generateLocaleIndex(locales)
	AvailableLangs = getAvailableLangs(locales)

	for key, value := range locales["en"].(map[string]string) {
		localeIndex["en"].(map[string]string)[key] = value
	}

	app.Get("/locales/:lang", func(c *fiber.Ctx) error {
		var localesToSend = make(Locales)
		var lang = c.Params("lang")
		lang = strings.Replace(lang, ".json", "", -1)
		var respHeader = c.GetRespHeaders()
		respHeader["Content-Type"] = []string{"application/json"}
		respHeader["Cache-Control"] = []string{"public, max-age=86400"}
		if value, ok := locales[lang]; ok {
			localesToSend[lang] = value
			return c.JSON(localesToSend)
		}
		return c.SendStatus(404)
	})

	app.Get("/locales.json", func(c *fiber.Ctx) error {
		var respHeader = c.GetRespHeaders()
		respHeader["Content-Type"] = []string{"application/json"}
		respHeader["Cache-Control"] = []string{"public, max-age=86400"}

		return c.JSON(localeIndex)
	})
}

type Locales = map[string]interface{}

func getAllLocales() Locales {
	var locales2paths = map[string][]string{}

	var extractLangs = func(dir string) {
		if !utils.ExistsSync(dir) {
			return
		}
		stat, _ := os.Stat(dir)

		if !stat.IsDir() {
			return
		}

		var readDirEntres, _ = os.ReadDir(dir)

		for _, entry := range readDirEntres {
			fileEntry, err := os.Stat(dir + "/" + entry.Name())
			if err != nil {
				continue
			}

			if fileEntry.IsDir() {
				continue
			}
			var extension = filepath.Ext(entry.Name())
			locale := entry.Name()[0 : len(entry.Name())-len(extension)]
			var _, ok = langs.lang[locale]
			if extension == ".json" && ok {
				if _, ok = locales2paths[locale]; !ok {
					locales2paths[locale] = []string{}
				}

				locales2paths[locale] = append(locales2paths[locale], dir+"/"+entry.Name())
			}
		}
		locales2paths["en"] = append(locales2paths["en"], dir+"/en-gb.json")
	}

	var joinedLocalesPath = path.Join(*settings2.SettingsDisplayed.Root, "assets/locales")

	extractLangs(joinedLocalesPath)
	type Metadata struct {
		Authors []string `json:"authors"`
	}
	type Locale = map[string]interface{}

	var locales = make(Locales)

	for key, val := range locales2paths {
		for _, pathToFile := range val {
			var fileContent, _ = os.ReadFile(pathToFile)
			locales[key] = map[string]string{}
			var mapOfStrings = Locale{}

			err := json.Unmarshal(fileContent, &mapOfStrings)
			if err != nil {
				println("Error reading locale from" + pathToFile + err.Error())
				continue
			}

			for keyInMap, ValInMap := range mapOfStrings {
				switch valString := ValInMap.(type) {
				case string:
					var rawInterface = locales[key]
					switch valueType := rawInterface.(type) {
					case map[string]string:
						valueType[keyInMap] = valString
					default:
						valueType = map[string]string{}
					}
				default:
					continue
				}
			}
		}
	}

	switch val := locales["en"].(type) {
	case map[string]string:
		{
			var val, _ = json.Marshal(val)
			println("en locale is a map" + string(val))
		}
	}

	return locales
}

func getAvailableLangs(locales Locales) map[string]LanguageInfo {
	var availableLangs = make(map[string]LanguageInfo)
	for key := range locales {
		availableLangs[key] = getLanguageInfo(key)
	}
	return availableLangs
}
