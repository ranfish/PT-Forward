package site

import (
	"fmt"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

type SelectOption struct {
	Value string
	Label string
}

type SiteFormConfig struct {
	Category      []SelectOption
	MediumSel     []SelectOption
	CodecSel      []SelectOption
	StandardSel   []SelectOption
	AudioCodec    []SelectOption
	TeamSel       []SelectOption
	ProcessingSel []SelectOption
	SourceSel     []SelectOption
	Tags          []SelectOption
}

type SiteSeedData struct {
	Domain             string
	Name               string
	BaseURL            string
	Framework          string
	AuthType           string
	IsSource           bool
	IsTarget           bool
	CookieCloudDomain  string
	AlternativeDomains string
	Form               SiteFormConfig
}

var seedSites = []SiteSeedData{
	{
		Domain: "longpt.org", Name: "龙PT", BaseURL: "https://longpt.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category: []SelectOption{
				{"401", "电影"}, {"402", "剧集"}, {"403", "综艺"}, {"404", "纪录片"},
				{"405", "动画"}, {"406", "音乐视频"}, {"407", "体育"}, {"408", "音频"},
				{"409", "其他"}, {"410", "有声书"}, {"411", "其他"},
			},
			MediumSel: []SelectOption{
				{"2", "UHD Blu-ray"}, {"1", "Blu-ray"}, {"11", "UHD Blu-ray Remux"},
				{"3", "Blu-ray Remux"}, {"7", "Encode"}, {"4", "WEB-DL"},
				{"5", "HDTV"}, {"6", "DVD"}, {"8", "CD"}, {"9", "Track"}, {"10", "Other"},
			},
			CodecSel: []SelectOption{
				{"1", "H.264/AVC"}, {"2", "H.265/HEVC"}, {"3", "VC-1"},
				{"4", "MPEG-2"}, {"5", "AV1"}, {"6", "Other"},
			},
			StandardSel: []SelectOption{
				{"5", "4K/2160p"}, {"6", "8K/4320p"}, {"2", "1080p/1080i"},
				{"3", "720p/720i"}, {"1", "2K/1440p"}, {"4", "480p/480i"}, {"7", "Other"},
			},
			AudioCodec: []SelectOption{
				{"1", "FLAC"}, {"2", "APE"}, {"3", "DTS-HD MA"}, {"4", "MP3"},
				{"5", "OGG"}, {"6", "AAC"}, {"8", "M4A"}, {"9", "TrueHD Atmos"},
				{"10", "E-AC3(DDP)"}, {"11", "Other"}, {"12", "DTS:X"}, {"13", "DTS"},
				{"14", "LPCM"}, {"15", "AC3"}, {"16", "ALAC"}, {"17", "WAV"},
				{"18", "AV3A"}, {"19", "TrueHD"},
			},
			TeamSel: []SelectOption{
				{"1", "LongA"}, {"2", "LongWeb"}, {"3", "LongPT"}, {"4", "WiKi"},
				{"5", "Other"}, {"6", "RL"}, {"7", "CMCT"}, {"8", "HHWEB"},
			},
			Tags: []SelectOption{
				{"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"5", "国语"},
				{"6", "中字"}, {"7", "HDR"}, {"8", "完结"}, {"9", "英字"},
				{"10", "杜比"}, {"11", "特效"}, {"12", "分集"}, {"13", "高分"},
				{"14", "臻彩MAX"}, {"15", "高码"}, {"16", "高帧"}, {"17", "去广告纯享版"},
			},
		},
	},
	{
		Domain: "pt.hdclone.top", Name: "克隆", BaseURL: "https://pt.hdclone.top",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category: []SelectOption{
				{"401", "Movies/电影"}, {"402", "TV Series/电视剧"},
				{"403", "TV Shows/综艺"}, {"404", "Documentaries/纪录片"},
				{"405", "Animations/动漫"}, {"407", "Othes/其他"},
				{"408", "Music/音乐"}, {"409", "Playlet/短剧"}, {"410", "MV/演唱会"},
			},
			MediumSel: []SelectOption{
				{"9", "UHD Blu-ray"}, {"1", "Blu-ray"}, {"3", "Remux"},
				{"7", "Encode"}, {"6", "WEB-DL"}, {"4", "MiniBD"},
				{"10", "DVD"}, {"8", "CD"}, {"2", "HDTV"}, {"5", "Track"},
			},
			CodecSel: []SelectOption{
				{"1", "H.264/x264/AVC"}, {"6", "H265/HEVC/x265"}, {"2", "VC-1"},
				{"3", "AV1"}, {"4", "MPEG-2"}, {"5", "Other"},
			},
			StandardSel: []SelectOption{
				{"6", "4320p/8K"}, {"1", "2160p/4K"}, {"2", "1080p/1080i"},
				{"3", "720p"}, {"4", "SD/720p以下"}, {"5", "Other"},
			},
			TeamSel: []SelectOption{
				{"1", "M-Team"}, {"2", "CHD"}, {"3", "BeiTai"}, {"4", "WiKi"},
				{"5", "AGSV"}, {"6", "FRDS"}, {"7", "TLF"}, {"8", "CMCT"},
				{"9", "beAst"}, {"10", "HDSky"}, {"11", "HDHome"}, {"12", "TTG"},
				{"13", "UBits"}, {"14", "PTer"},
			},
			Tags: []SelectOption{
				{"1", "禁转"}, {"4", "DIY"}, {"5", "国语"}, {"6", "中字"}, {"7", "HDR"},
			},
		},
	},
	{
		Domain: "pt.lajidui.top", Name: "垃圾堆", BaseURL: "https://pt.lajidui.top",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category: []SelectOption{
				{"401", "Movies/电影"}, {"402", "TV Series/电视剧"},
				{"403", "TV Shows/综艺"}, {"404", "Documentaries/纪录片"},
				{"405", "Animations/动漫"}, {"406", "Music Videos/音乐视频"},
				{"407", "Sports/体育"}, {"408", "Audio/音频"}, {"409", "Misc/其他"},
				{"410", "Cartoon/少儿动画"}, {"411", "Ebook/电子书"},
				{"412", "ShortDrama/短剧"}, {"413", "Game/游戏"},
				{"414", "APP/软件"}, {"415", "Education/教育视频"},
				{"416", "Audiobook/有声书"},
			},
			MediumSel: []SelectOption{
				{"1", "Blu-ray"}, {"2", "HD DVD"}, {"3", "Remux"}, {"4", "MiniBD"},
				{"5", "HDTV"}, {"6", "DVDR"}, {"7", "Encode"}, {"8", "CD"},
				{"9", "Track"}, {"10", "WEB-DL"}, {"11", "Other"},
			},
			CodecSel: []SelectOption{
				{"1", "H.264"}, {"2", "VC-1"}, {"3", "Xvid"}, {"4", "MPEG-2"},
				{"5", "Other"}, {"6", "AV1"}, {"7", "H.265"},
			},
			StandardSel: []SelectOption{
				{"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"},
				{"5", "2k"}, {"6", "4k"}, {"7", "8k"}, {"8", "Other"},
			},
			AudioCodec: []SelectOption{
				{"1", "FLAC"}, {"2", "APE"}, {"3", "DTS"}, {"4", "MP3"},
				{"5", "OGG"}, {"6", "AAC"}, {"7", "Other"}, {"8", "WAV"},
				{"9", "DTS-HD"}, {"10", "TrueHD"}, {"11", "LPCM"},
				{"12", "E-AC-3"}, {"13", "AC-3"},
			},
			TeamSel: []SelectOption{
				{"1", "HDSky"}, {"2", "CHD"}, {"3", "原创"}, {"4", "WiKi"},
				{"5", "Other"}, {"6", "HHWEB"}, {"7", "ADE"}, {"8", "CMCT"},
				{"9", "FRDS"}, {"10", "TJUPT"}, {"11", "UBits"}, {"12", "Ourbits"},
				{"13", "QHstudIo"}, {"14", "HDHome"}, {"15", "AGSVWEB"},
				{"16", "Pter"}, {"17", "CatEDU"}, {"18", "beAst"}, {"19", "LHD"},
				{"20", "BMDru"}, {"21", "BeiTai"}, {"22", "GodDramas"},
			},
			ProcessingSel: []SelectOption{
				{"1", "EPUB"}, {"2", "PDF"}, {"3", "TXT"}, {"4", "DOCX"},
				{"5", "PPTX"}, {"6", "XLSX"}, {"7", "WPS"}, {"8", "AZW3"},
				{"9", "MOBI"}, {"10", "MKV"}, {"11", "MP4"}, {"12", "RAR"},
				{"13", "ZIP"}, {"14", "7z"}, {"16", "ISO"}, {"17", "Other"},
			},
			SourceSel: []SelectOption{
				{"1", "欧美"}, {"2", "台湾"}, {"3", "印度"}, {"6", "Other"},
				{"7", "大陆"}, {"8", "香港"}, {"10", "日本"}, {"11", "韩国"},
			},
			Tags: []SelectOption{
				{"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"5", "国语"},
				{"6", "中字"}, {"7", "HDR"}, {"8", "已刮削"}, {"9", "完结"},
				{"10", "杜比"}, {"11", "粤语"}, {"12", "单集"}, {"13", "三级"},
				{"14", "英语"}, {"15", "国英双语"}, {"16", "简英双语字幕"}, {"17", "多音轨"},
			},
		},
	},
	{
		Domain: "kufei.org", Name: "库非", BaseURL: "https://kufei.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category: []SelectOption{
				{"401", "Movies/电影"}, {"402", "TV Series/电视剧"},
				{"403", "TV Shows/综艺"}, {"404", "Documentaries/纪录片"},
				{"405", "Animations/动漫"}, {"406", "Music Videos/音乐视频"},
				{"407", "Sports/体育"}, {"408", "Music/音乐"},
				{"409", "Others/其他"}, {"410", "Games/游戏"},
				{"411", "E-Books/电子书"}, {"412", "Software/软件"},
				{"413", "Education/教育"}, {"414", "Concern/演唱会"},
				{"415", "Drama/戏剧"}, {"416", "Audio Books/有声读物"},
			},
			MediumSel: []SelectOption{
				{"1", "UHD原盘"}, {"2", "UHD DIY"}, {"3", "UHD Remux"},
				{"4", "BD 原盘"}, {"5", "BD DIY"}, {"6", "BD Remux"},
				{"7", "UHD 压制"}, {"8", "1080P/i 压制"}, {"9", "720P 压制"},
				{"10", "MiniSD"}, {"11", "WEB-DL"}, {"12", "HDTV"},
				{"13", "DVD"}, {"14", "CD"}, {"15", "SACD"}, {"16", "Others"},
				{"17", "Encode"},
			},
			CodecSel: []SelectOption{
				{"1", "H.264/AVC"}, {"2", "X264"}, {"3", "H.265/HEVC"},
				{"4", "X265"}, {"5", "VC-1"}, {"6", "MPEG-2"}, {"7", "MPEG-4"},
				{"8", "Xvid"}, {"9", "AV1"}, {"10", "Other"},
			},
			StandardSel: []SelectOption{
				{"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"},
				{"5", "8K/4320P"}, {"6", "4K/UHD/2160P"}, {"7", "Other"},
			},
			AudioCodec: []SelectOption{
				{"1", "FLAC"}, {"2", "APE"}, {"4", "MP3"}, {"5", "OGG"},
				{"7", "Other"}, {"8", "TrueHD Atmos"}, {"9", "DTS"},
				{"10", "DTS X"}, {"11", "DTS-HDMA"}, {"12", "DTS-HD HR"},
				{"13", "True-HD"}, {"14", "LPCM"}, {"15", "DDP/DD+"},
				{"16", "Dolby Digital/DD"}, {"17", "AC3"}, {"18", "AAC"},
				{"19", "WAV"}, {"20", "DSD"}, {"21", "OGG"}, {"22", "TTA"},
				{"23", "MPEG"}, {"24", "DDP Atmos"},
			},
			TeamSel: []SelectOption{
				{"1", "HDS"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"},
			},
			Tags: []SelectOption{
				{"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"5", "国语"},
				{"6", "中字"}, {"7", "HDR"}, {"8", "英语"}, {"9", "动漫"},
				{"10", "音乐"}, {"11", "书籍"}, {"12", "完结"}, {"13", "应求"}, {"14", "游戏"},
			},
		},
	},
	{
		Domain: "yhpp.cc", Name: "昆仑", BaseURL: "https://yhpp.cc",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category: []SelectOption{
				{"401", "Movies/电影"}, {"402", "TV Series/电视剧"},
				{"403", "TV Shows/综艺"}, {"404", "Documentaries/纪录片"},
				{"405", "Animations/动漫"}, {"406", "Music Videos/音乐视频"},
				{"407", "Sports/体育"}, {"408", "Music/音乐"}, {"409", "Others/其他"},
			},
			MediumSel: []SelectOption{
				{"19", "UHD原盘"}, {"18", "UHD DIY"}, {"17", "UHD Remux"},
				{"16", "UHD压制"}, {"15", "BD原盘"}, {"14", "BD DIY"},
				{"13", "BD Remux"}, {"12", "1080P/i压制"}, {"11", "720P压制"},
				{"10", "MiniSD"}, {"9", "WEB-DL"}, {"8", "HDTV"}, {"7", "CD+VCD"},
				{"6", "DVD"}, {"5", "CD"}, {"4", "SACD"}, {"3", "CD+DVD"},
				{"2", "黑胶"}, {"1", "Other/其他"},
			},
			CodecSel: []SelectOption{
				{"1", "MPEG-2"}, {"2", "MPEG-4"}, {"3", "Xvid"}, {"4", "AV1"},
				{"5", "Other/其他"}, {"6", "VC-1"}, {"7", "x265"},
				{"8", "H.265/HEVC"}, {"9", "x264"}, {"10", "H.264/AVC"},
			},
			StandardSel: []SelectOption{
				{"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"},
				{"5", "Other/其他"}, {"6", "4K/UHD/2160P"}, {"7", "8K/4320P"},
			},
			AudioCodec: []SelectOption{
				{"1", "AAC"}, {"2", "AC3"}, {"3", "TTA"}, {"4", "MP3"},
				{"5", "ALAC"}, {"6", "m4a"}, {"7", "Other/其他"}, {"8", "OGG"},
				{"9", "MPEG"}, {"10", "Dolby Digital/DD"}, {"11", "DDP/DD+/EAC3"},
				{"12", "DDP Atmos"}, {"13", "DSD"}, {"14", "FLAC"}, {"15", "APE"},
				{"16", "WAV"}, {"17", "LPCM"}, {"18", "DTS"}, {"19", "DTS-HD HR"},
				{"20", "DTS-HDMA"}, {"21", "True-HD"}, {"22", "DTS:X"},
				{"23", "TrueHD Atmos"},
			},
			TeamSel: []SelectOption{
				{"1", "DIC"}, {"2", "Red"}, {"3", "GGN"}, {"4", "LemonHD"},
				{"5", "Other/其他"}, {"6", "OpenCD"}, {"7", "FraMeSToR"},
				{"8", "EPSiLON"}, {"9", "BTN/NTb"}, {"10", "PTP"}, {"11", "TLF"},
				{"12", "Hares"}, {"13", "QHstudIo"}, {"14", "PTHome"},
				{"15", "HDHome"}, {"16", "HHClub"}, {"17", "Ubits"},
				{"18", "Audies"}, {"19", "PTer"}, {"20", "OurBits"},
				{"21", "HDS"}, {"22", "FRDS"}, {"23", "CMCT"}, {"24", "beAst"},
				{"25", "WiKi"}, {"26", "TTG"}, {"27", "HDC"}, {"28", "CHDBits"},
				{"29", "HDFans"},
			},
			ProcessingSel: []SelectOption{
				{"1", "MY/马来西亚"}, {"2", "Other/其他"}, {"3", "SG/新加坡"},
				{"4", "IN/印度"}, {"5", "KR/韩国"}, {"6", "JP/日本"},
				{"7", "UK/英国"}, {"8", "EU/欧洲"}, {"9", "US/美国"},
				{"10", "TW/台湾"}, {"11", "HK/香港"}, {"12", "CN/中国大陆"},
			},
			Tags: []SelectOption{
				{"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"5", "国语"},
				{"6", "中字"}, {"7", "HDR"}, {"8", "微光星辰"}, {"9", "原创"},
				{"10", "源站转发"}, {"11", "中英双语"}, {"12", "特效"},
				{"13", "Dolby Vision"}, {"14", "Atmos"}, {"15", "4K"},
				{"16", "8K"}, {"17", "Hi-Res"}, {"18", "完结"}, {"19", "刮削"},
				{"20", "AI修复"}, {"21", "保种"},
			},
		},
	},
	{
		Domain: "13city.org", Name: "13城", BaseURL: "https://13city.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"401", "电影/Movies"}, {"402", "剧集/TV Series"}, {"403", "综艺/TV Shows"}, {"405", "动漫/Animations"}, {"406", "演唱会、MV/Concert、Music Videos"}, {"408", "音乐/Music"}, {"413", "纪录片/Docmentaries"}, {"409", "有声读物/Audiobook"}},
			MediumSel:     []SelectOption{{"9", "Track"}, {"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "HD DVD"}, {"1", "Blu-ray"}, {"10", "WEB-DL"}, {"11", "BluRay"}, {"12", "WEBRip"}, {"13", "Other"}},
			CodecSel:      []SelectOption{{"1", "AVC/H.264/x264"}, {"2", "HEVC/H.265/x265"}, {"3", "MPEG-2"}, {"4", "VC-1"}, {"5", "VPB/VP9"}, {"6", "Xvid"}, {"7", "Other"}},
			StandardSel:   []SelectOption{{"1", "8K"}, {"2", "4K"}, {"3", "1080p"}, {"4", "1080i"}, {"5", "Other"}},
			AudioCodec:    []SelectOption{{"1", "FLAC"}, {"2", "APE"}, {"3", "DTS-HD/DTS"}, {"4", "MP3"}, {"5", "OGG"}, {"6", "AAC"}, {"7", "DDP/E-AC3"}, {"8", "TrueHD"}, {"9", "TrueHD Atmos"}, {"10", "LPCM"}, {"11", "Other"}, {"12", "DD/AC3"}},
			TeamSel:       []SelectOption{{"10", "rainweb"}, {"19", "QHstudIo"}, {"18", "HDSWEB"}, {"17", "WiKi"}, {"16", "UBits"}, {"15", "ADWeb"}, {"14", "ZmWeb"}, {"13", "UBWEB"}, {"12", "SewageWeb"}, {"11", "Other"}, {"1", "13City"}, {"9", "PTerWEB"}, {"8", "QHstudIo"}, {"7", "MTeam"}, {"6", "FRDS"}, {"5", "CHDWEB"}, {"4", "HHWEB"}, {"3", "52pt"}, {"2", "AilMWeb"}},
			ProcessingSel: []SelectOption{{"1", "中国（含港澳台）"}, {"2", "日本"}, {"3", "泰国"}, {"4", "印度"}, {"5", "韩国"}, {"6", "欧美"}, {"7", "Other（其他）​"}},
			Tags:          []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"9", "完结"}, {"8", "分集"}, {"13", "粤语"}, {"10", "多语"}, {"5", "国语"}, {"6", "中字"}, {"14", "中英双字"}, {"12", "4K"}, {"11", "1080p"}, {"7", "HDR"}, {"4", "DIY"}, {"19", "红叶转载"}, {"15", "有声图书"}},
		},
	},
	{
		Domain: "1ptba.com", Name: "壹吧", BaseURL: "https://1ptba.com",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"401", "Movie(電影)"}, {"402", "TV Series(電視影劇)"}, {"403", "TV Shows(電視綜藝)"}, {"404", "Discovery(紀錄教育)"}, {"405", "Cartoon(卡通動漫)"}, {"406", "Music Videos(音樂短片/演唱會)"}, {"407", "Sports(體育賽事)"}, {"408", "HQ Audio(高品质音频)"}, {"410", "Software(軟體)"}, {"411", "Games(電子遊戲)"}, {"412", "eBook(電子書)"}, {"409", "Misc(其他)"}},
			MediumSel:     []SelectOption{{"9", "Track"}, {"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "HD DVD"}, {"1", "Blu-ray(原盘)"}, {"16", "UHD Blu-ray"}, {"17", "UHD Blu-ray/DIY"}, {"19", "Blu-ray/DIY"}},
			CodecSel:      []SelectOption{{"1", "H.264(AVC)"}, {"2", "VC-1"}, {"3", "Xvid"}, {"4", "MPEG-2"}, {"5", "Other"}, {"18", "H.265(HEVC)"}, {"19", "FLAC"}, {"20", "APE"}, {"21", "DTS"}, {"22", "AC-3"}, {"23", "WAV"}, {"24", "MP3"}, {"25", "ALAC"}, {"26", "AAC"}},
			StandardSel:   []SelectOption{{"1", "1080p-HD(逐行)/1920×1080"}, {"2", "1080i-HD(隔行)/1920×1080"}, {"3", "720p-HD(逐行)/1280×720"}, {"4", "SD(标清)/720p×576p"}, {"16", "4K-UHD(超高清)/3840×2160"}, {"17", "8K-UHD(超高清)/7680×4320"}},
			AudioCodec:    []SelectOption{{"1", "FLAC"}, {"2", "APE"}, {"3", "DTS"}, {"4", "MP3"}, {"5", "OGG"}, {"6", "AAC"}, {"7", "Other"}, {"31", "TrueHD"}},
			TeamSel:       []SelectOption{{"1", "HDS"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}, {"20", "1PTBA"}},
			ProcessingSel: []SelectOption{{"1", "Raw"}, {"2", "Encode"}},
			SourceSel:     []SelectOption{{"1", "Blu-ray(原盘)"}, {"2", "DVD(原盘)"}, {"4", "HDTV"}, {"6", "Other"}, {"16", "UHD Blu-ray"}, {"17", "UHD Blu-ray/DIY"}, {"19", "Blu-ray/DIY"}, {"20", "REMUX"}, {"22", "encode"}, {"23", "WEB-DL"}, {"25", "CD"}, {"26", "Track"}},
			Tags:          []SelectOption{{"1", "禁转"}, {"17", "限转"}, {"18", "原创"}, {"2", "首发"}, {"5", "国配"}, {"19", "粤配"}, {"6", "中字"}, {"20", "官字组"}, {"4", "DIY"}, {"21", "Dolby Vision"}, {"7", "HDR10"}, {"22", "HDR10+"}, {"23", "应求"}, {"24", "特效"}, {"25", "音乐专辑"}, {"26", "Music Video"}, {"27", "卡拉OK"}, {"28", "LIVE现场"}, {"29", "演唱会"}, {"31", "AI修复"}, {"30", "最佳影片"}},
		},
	},
	{
		Domain: "audiences.me", Name: "人人", BaseURL: "https://audiences.me",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影"}, {"402", "剧集"}, {"403", "综艺"}, {"406", "纪录片"}, {"408", "音乐"}, {"404", "有声书"}, {"405", "电子书"}, {"407", "体育"}, {"410", "游戏"}, {"412", "学习"}, {"409", "其他"}},
			MediumSel:   []SelectOption{{"12", "UHD Blu-ray 原盘"}, {"13", "UHD Blu-ray DIY"}, {"1", "Blu-ray 原盘"}, {"14", "Blu-ray DIY"}, {"3", "REMUX"}, {"15", "Encode"}, {"5", "HDTV"}, {"10", "WEB-DL"}, {"2", "DVD 原盘"}, {"8", "CD"}, {"9", "Track"}, {"11", "Other"}},
			CodecSel:    []SelectOption{{"6", "H.265(HEVC)"}, {"1", "H.264(AVC)"}, {"2", "VC-1"}, {"4", "MPEG-2"}, {"7", "AV1"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"10", "8K"}, {"5", "4K"}, {"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}, {"11", "None"}},
			AudioCodec:  []SelectOption{{"25", "DTS:X"}, {"26", "TrueHD Atmos"}, {"19", "DTS-HD MA"}, {"20", "TrueHD"}, {"21", "LPCM"}, {"3", "DTS"}, {"18", "DD/AC3"}, {"27", "OPUS"}, {"6", "AAC"}, {"1", "FLAC"}, {"2", "APE"}, {"22", "WAV"}, {"23", "MP3"}, {"24", "M4A"}, {"7", "Other"}},
		},
	},
	{
		Domain: "cangbao.ge", Name: "藏宝阁", BaseURL: "https://cangbao.ge",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影"}, {"402", "剧集"}, {"403", "短剧"}, {"404", "动画"}, {"405", "动漫"}, {"406", "儿童"}, {"407", "综艺"}, {"408", "纪录"}, {"409", "音乐"}, {"410", "书籍"}},
			MediumSel:   []SelectOption{{"9", "UHD"}, {"8", "Remux"}, {"7", "Blu-ray"}, {"6", "WEB-DL"}, {"5", "UHDTV"}, {"4", "HDTV"}, {"3", "Encode"}, {"2", "DVD"}, {"1", "VCD"}},
			CodecSel:    []SelectOption{{"9", "AV1"}, {"8", "HEVC"}, {"7", "VP9"}, {"6", "AVC"}, {"5", "VC-1"}, {"4", "MPEG-4"}, {"3", "MPEG-2"}, {"2", "MPEG-1"}, {"1", "H.261"}},
			StandardSel: []SelectOption{{"7", "4320p"}, {"6", "2160p"}, {"5", "1440p"}, {"4", "1080p"}, {"3", "720p"}, {"2", "540p"}, {"1", "480P"}},
			AudioCodec:  []SelectOption{{"9", "PCM"}, {"8", "TrueHD"}, {"7", "FLAC"}, {"6", "Opus"}, {"5", "DDP"}, {"4", "AAC"}, {"3", "DTS"}, {"2", "AC3"}, {"1", "MP3"}},
			TeamSel:     []SelectOption{{"1", "NONE"}, {"2", "CBGWT"}, {"3", "CBGER"}},
			Tags:        []SelectOption{{"20", "合集"}, {"19", "官方"}, {"18", "原声"}, {"17", "DIY"}, {"16", "特效"}, {"15", "原盘"}, {"14", "DV"}, {"13", "HDR"}, {"12", "高码"}, {"11", "零魔"}, {"10", "禁转"}, {"9", "英字"}, {"8", "国语"}, {"7", "中字"}, {"5", "驻站"}, {"4", "分集"}, {"3", "完结"}, {"2", "自购"}, {"1", "首发"}},
		},
	},
	{
		Domain: "carpt.net", Name: "车站", BaseURL: "https://carpt.net",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影/Movies"}, {"402", "连续剧/TV-Series"}, {"403", "动漫/Animation"}, {"404", "纪录片/Documentary"}, {"405", "综艺/TV-Show"}, {"406", "音乐/Music"}, {"407", "其他/Other"}},
			MediumSel:   []SelectOption{{"1", "Encode"}, {"2", "WEB"}, {"3", "HDTV"}, {"4", "DVDRip"}, {"5", "CD"}, {"6", "Other"}, {"7", "Blu-ray"}, {"8", "Blu-rayUHD"}, {"9", "Remux"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC/x264"}, {"2", "H.265/HEVC/x265"}, {"3", "MPEG-2"}, {"4", "VC-1"}, {"5", "Xvid"}, {"6", "Other"}},
			StandardSel: []SelectOption{{"1", "4K_UHD"}, {"2", "1080p/i"}, {"3", "720p/i"}, {"4", "SD"}, {"5", "Other"}},
			AudioCodec:  []SelectOption{{"1", "TrueHD"}, {"2", "DTS-HD/DTS"}, {"3", "AC3"}, {"4", "LPCM"}, {"5", "FLAC"}, {"6", "mp3"}, {"7", "AAC"}, {"8", "APE"}, {"10", "wav"}, {"9", "Other"}},
			TeamSel:     []SelectOption{{"1", "CarPT"}, {"2", "WiKi"}, {"3", "CMCT"}, {"4", "M-team"}, {"5", "Other"}},
			Tags:        []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"5", "国语"}, {"6", "中字"}, {"7", "HDR"}},
		},
	},
	{
		Domain: "cyanbug.net", Name: "大青虫", BaseURL: "https://cyanbug.net",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"409", "其他"}, {"408", "音轨"}, {"406", "MV"}, {"407", "体育"}, {"403", "综艺"}, {"404", "纪录片"}, {"405", "动漫"}, {"402", "电视剧"}, {"401", "电影"}},
			MediumSel:   []SelectOption{{"1", "Blu-ray"}, {"11", "UHD Blu-ray"}, {"3", "Remux"}, {"7", "Encode"}, {"10", "WEB-DL"}, {"2", "HD DVD"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"6", "DVDR"}, {"8", "CD"}, {"9", "Track"}, {"12", "Other"}},
			CodecSel:    []SelectOption{{"6", "H.265"}, {"1", "H.264"}, {"2", "VC-1"}, {"3", "Xvid"}, {"7", "MPEG-4"}, {"4", "MPEG-2"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"5", "2160p"}, {"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}},
			AudioCodec:  []SelectOption{{"15", "DTS-HD"}, {"11", "DTS-HDMA:X 7.1"}, {"12", "DTS-HDMA"}, {"10", "TrueHD"}, {"13", "Atmos"}, {"3", "DTS"}, {"8", "DD/AC3"}, {"6", "AAC"}, {"1", "FLAC"}, {"2", "APE"}, {"14", "LPCM"}, {"9", "WAV"}, {"7", "Other"}},
			TeamSel:     []SelectOption{{"6", "MTean"}, {"7", "PTer"}, {"1", "HDS"}, {"2", "CHD"}, {"12", "HDC"}, {"4", "WiKi"}, {"10", "BeiTai"}, {"13", "OurBits"}, {"3", "CMCT"}, {"8", "FRDS"}, {"14", "HDH"}, {"15", "Pter"}, {"16", "LeagueWEB"}, {"9", "Audies"}, {"18", "ADWeb"}, {"11", "HHWEB"}, {"17", "SharkWEB"}, {"5", "Other"}},
			Tags:        []SelectOption{{"2", "首发"}, {"1", "禁转"}, {"5", "国语"}, {"6", "中字"}, {"4", "DIY"}, {"7", "HDR 10"}, {"8", "Dolby Vision"}},
		},
	},
	{
		Domain: "discfan.net", Name: "碟粉", BaseURL: "https://discfan.net",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:  []SelectOption{{"401", "电影 - 中国大陆"}, {"404", "电影 - 中国香港"}, {"405", "电影 - 中国台湾"}, {"402", "电影 - 泰国"}, {"403", "电影 - 日本"}, {"406", "电影 - 韩国"}, {"410", "电影 - 世界"}, {"411", "剧集"}, {"414", "音乐"}, {"413", "纪录"}, {"416", "综艺"}, {"417", "体育"}, {"419", "动漫"}},
			SourceSel: []SelectOption{{"1", "HDTV"}, {"2", "4K UltraHD"}, {"3", "Blu-ray Disc"}, {"4", "DVD"}, {"5", "SDTV"}, {"6", "VCD"}, {"7", "LD"}, {"8", "VHS"}, {"9", "Web-DL"}, {"10", "Rip"}, {"11", "Book"}, {"131", "Remux"}},
			Tags:      []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"9", "自购"}, {"4", "DIY"}, {"8", "粤语"}, {"5", "国语"}, {"6", "中字"}, {"10", "DoVi"}, {"7", "HDR"}},
		},
	},
	{
		Domain: "dubhe.site", Name: "天枢", BaseURL: "https://dubhe.site",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "Movies"}, {"402", "TV Series"}, {"403", "TV Shows"}, {"404", "Documentaries"}, {"405", "Animations"}, {"406", "Music Videos"}, {"407", "Sports"}, {"408", "HQ Audio"}, {"409", "Misc"}, {"410", "Books"}, {"411", "photo"}},
			MediumSel:   []SelectOption{{"9", "Track"}, {"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "HD DVD"}, {"1", "Blu-ray"}, {"10", "WEB-DL"}},
			CodecSel:    []SelectOption{{"11", "H.265/HEVC"}, {"10", "x.265"}, {"13", "H.264/AVC"}, {"12", "x264"}, {"7", "MPEG-4"}, {"6", "MPEG-2"}, {"2", "VC-1"}, {"9", "AV1"}, {"8", "Xvid"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"5", "2160p/2160i"}, {"1", "1080p/1080i"}, {"3", "720p"}, {"4", "SD"}, {"7", "Other/其他"}},
			TeamSel:     []SelectOption{{"1", "HDS"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}, {"6", "DubheWeb"}},
			Tags:        []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"5", "国语"}, {"6", "中字"}, {"7", "HDR"}},
		},
	},
	{
		Domain: "duckboobee.org", Name: "三月", BaseURL: "https://duckboobee.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "Movies"}, {"402", "TV Series"}, {"403", "TV Shows"}, {"404", "Documentaries"}, {"405", "Cartoon"}, {"406", "MTV"}, {"407", "Sports"}, {"408", "Music"}, {"409", "Misc"}},
			MediumSel:   []SelectOption{{"13", "WEB-DL"}, {"8", "DVD"}, {"5", "HDTV"}, {"4", "Other"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "DVDR"}, {"1", "Blu-Ray"}},
			CodecSel:    []SelectOption{{"1", "H.264"}, {"2", "HEVC"}, {"3", "H.265"}, {"4", "MPEG-2"}, {"5", "MPEG-4"}, {"6", "X.264"}, {"7", "X.265"}, {"9", "XVID"}, {"10", "AV1"}, {"11", "Other"}},
			StandardSel: []SelectOption{{"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "576P"}, {"6", "2160P"}, {"7", "480P"}, {"8", "4320P"}},
			TeamSel:     []SelectOption{{"1", "3MWEB"}, {"2", "CHD"}, {"3", "TTG"}, {"4", "CMCT"}, {"5", "HDSKY"}, {"6", "OurBits"}, {"7", "M-Team"}, {"8", "WIKI"}, {"9", "Beast"}, {"10", "HHWEB"}, {"11", "Other"}},
			Tags:        []SelectOption{{"2", "分集"}, {"4", "DIY"}, {"5", "国语"}, {"9", "首发"}, {"6", "中字"}, {"7", "HDR"}, {"22", "音乐专辑"}, {"21", "演唱会"}, {"20", "LVE现场"}, {"19", "卡拉OK"}, {"18", "MV"}, {"17", "应求"}, {"16", "限转"}, {"15", "HDR10+"}, {"14", "HDR10"}, {"13", "Dolby Vision"}, {"12", "动画"}, {"11", "官字组"}, {"10", "粤语"}, {"8", "完结"}, {"1", "禁转"}},
		},
	},
	{
		Domain: "et8.org", Name: "他吹吹风", BaseURL: "https://et8.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"624", "Documentaries.纪录片"}, {"628", "Elearning - 杂项学习"}, {"629", "Elearning - 电子书/小说"}, {"630", "Elearning - 电子书/非小说"}, {"631", "Elearning - 杂志"}, {"632", "Elearning - 漫画"}, {"633", "Elearning - 有声书"}, {"634", "Elearning - 公开课"}, {"635", "Elearning - 视频教程"}},
			MediumSel:   []SelectOption{{"10", "UHD Bluray"}, {"1", "BluRay"}, {"5", "Remux"}, {"11", "Encode"}, {"9", "WEB-DL"}, {"6", "HDTV"}, {"3", "HDRip"}, {"7", "DVDR"}, {"4", "DVDRip"}, {"8", "Other"}, {"12", "PDF"}, {"13", "EPUB"}, {"14", "AZW3"}, {"15", "MOBI"}, {"16", "TXT"}, {"17", "Pictures"}},
			CodecSel:    []SelectOption{{"1", "H.264"}, {"6", "x265"}, {"8", "H265/HEVC"}, {"7", "x264"}, {"2", "VC-1"}, {"3", "Xvid"}, {"4", "MPEG-2"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"5", "2160/4K"}, {"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}},
			AudioCodec:  []SelectOption{{"1", "FLAC"}, {"2", "APE"}, {"3", "DTS"}, {"4", "AC3"}, {"6", "AAC"}, {"8", "DTS-HD"}, {"9", "TrueHD"}, {"10", "LPCM"}, {"11", "WAV"}, {"5", "MP3"}, {"7", "Other"}},
			TeamSel:     []SelectOption{{"1", "TorrentCCF/TCCF"}, {"2", "TLF"}, {"3", "BMDru"}, {"4", "CatEDU"}, {"5", "MADFOX"}, {"6", "个人原创资源(Original)"}, {"7", "其他(other)"}},
			SourceSel:   []SelectOption{{"1", "信息技术"}, {"2", "自然科学"}, {"3", "社会科学"}, {"4", "哲学"}, {"5", "法律"}, {"6", "军事政治"}, {"7", "经济"}, {"8", "文体教育/少儿教育"}, {"9", "文体教育/非少儿教育"}, {"10", "语言文字"}, {"11", "文学艺术"}, {"12", "历史地理"}, {"13", "医学卫生"}, {"14", "其他"}},
		},
	},
	{
		Domain: "haidan.cc", Name: "海胆", BaseURL: "https://haidan.cc",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		AlternativeDomains: "www.haidan.cc",
		Form: SiteFormConfig{
			Category:    []SelectOption{{"404", "Documentaries(纪录片)"}, {"401", "Movies(电影)"}, {"405", "Animations(动画片)"}, {"402", "TV Series(电视剧)"}, {"403", "TV Shows(综艺)"}, {"406", "Music Videos(MV)"}, {"407", "Sports(体育)"}, {"409", "Misc(其他)"}, {"408", "HQ Audio(音乐)"}},
			MediumSel:   []SelectOption{{"9", "UHD Blu-ray"}, {"1", "Blu-ray"}, {"3", "Remux"}, {"7", "Encode"}, {"5", "HDTV"}, {"11", "WEB-DL"}, {"6", "DVD"}, {"8", "CD"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC/X264"}, {"2", "VC-1"}, {"4", "MPEG-2"}, {"5", "Other"}, {"11", "H.265/HEVC/X265"}, {"13", "MPEG-4/XviD/DivX"}},
			StandardSel: []SelectOption{{"1", "2160p/4K"}, {"2", "1080p"}, {"3", "1080i"}, {"4", "720p"}, {"5", "540P"}},
			AudioCodec:  []SelectOption{{"1", "FLAC"}, {"2", "APE"}, {"3", "DTS"}, {"4", "MP3"}, {"6", "AAC"}, {"7", "Other"}, {"10", "AC3"}, {"11", "LPCM"}, {"12", "DTS-HDMA"}, {"13", "True-HD"}},
		},
	},
	{
		Domain: "hdarea.club", Name: "好大", BaseURL: "https://hdarea.club",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"300", "Movie UHD-4K"}, {"401", "Movies Blu-ray"}, {"415", "Movies REMUX"}, {"416", "Movies 3D"}, {"410", "Movies 1080p"}, {"411", "Movies 720p"}, {"414", "Movies DVD"}, {"412", "Movies WEB-DL"}, {"413", "Movies HDTV"}, {"417", "Movies iPad"}, {"404", "Documentaries"}, {"405", "Animations"}, {"402", "TV Series"}, {"403", "TV Shows"}, {"406", "Music Videos"}, {"407", "Sports"}, {"409", "Misc"}, {"408", "HQ Audio"}},
			MediumSel:   []SelectOption{{"1", "Blu-ray"}, {"3", "REMUX"}, {"7", "Encode"}, {"9", "WEB-DL"}, {"4", "MiniBD"}, {"5", "HDTV"}, {"2", "HD DVD"}, {"6", "DVDR"}, {"8", "CD"}},
			CodecSel:    []SelectOption{{"7", "H.264(x264/AVC)"}, {"1", "MPEG-4"}, {"6", "H.265(x265/HEVC)"}, {"4", "MPEG-2"}, {"3", "Xvid"}, {"2", "VC-1"}, {"5", "Other"}, {"8", "AV1"}, {"9", "VP8/9"}, {"10", "AVS"}},
			StandardSel: []SelectOption{{"3", "720p"}, {"1", "1080p"}, {"4", "SD"}, {"2", "1080i"}, {"5", "4K"}},
			AudioCodec:  []SelectOption{{"6", "AAC"}, {"5", "DD5.1/AC-3"}, {"7", "TrueHD"}, {"3", "DTS"}, {"4", "DTS-HD MA/DTS XLL"}, {"8", "LPCM"}, {"9", "WAV"}, {"2", "APE"}, {"1", "FLAC"}, {"10", "TrueHD Atmos"}, {"11", "DD2.0/AC-3"}, {"12", "DTS:X"}, {"13", "DTS-HD HR/HRA"}, {"14", "DSD"}, {"15", "DDP Atmos"}, {"16", "DDP/E-AC-3"}, {"17", "MPEG"}, {"18", "Vorbis"}, {"19", "TTA"}, {"20", "AV3A"}, {"21", "MP3"}, {"22", "ALAC"}, {"25", "Opus"}, {"26", "WMA"}, {"27", "AC-4"}, {"28", "MPEG-H"}, {"29", "MQA"}, {"24", "Other"}},
			TeamSel:     []SelectOption{{"1", "EPiC"}, {"2", "HDArea"}, {"3", "HDWING"}, {"4", "WiKi"}, {"5", "TTG"}, {"6", "other"}, {"7", "MTeam"}, {"8", "HDApad"}, {"9", "CHD"}, {"10", "HDAccess"}, {"11", "HDATV"}, {"12", "cXcY"}, {"13", "CMCT"}},
		},
	},
	{
		Domain: "hdhome.org", Name: "家园", BaseURL: "https://hdhome.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"506", "Movies 8K UHD BD"}, {"499", "Movies UHD Blu-ray"}, {"518", "Movies UHD REMUX"}, {"450", "Movies Bluray"}, {"415", "Movies REMUX"}, {"505", "Movies 8K/4320p"}, {"416", "Movies 2160p"}, {"414", "Movies 1080p"}, {"413", "Movies 720p"}, {"411", "Movies SD"}, {"412", "Movies IPad"}, {"523", "TVSeries 8KUHD"}, {"502", "TVSeries 4K Bluray"}, {"451", "Doc Bluray"}, {"421", "Doc REMUX"}, {"526", "TVSeries 4320p"}, {"431", "TVShow 2160p"}, {"433", "TVSeries IPad"}, {"434", "TVSeries 720p"}, {"435", "TVSeries 1080i"}, {"436", "TVSeries 1080p"}, {"437", "TVSeries REMUX"}, {"453", "TVSereis Bluray"}, {"438", "TVSeries 2160p"}, {"439", "Musics APE"}, {"432", "TVSeries SD"}, {"440", "Musics FLAC"}, {"441", "Musics MV"}, {"503", "Musics Bluray"}, {"442", "Sports 720p"}, {"510", "Anime 8K UHD BD"}, {"443", "Sports 1080i"}, {"444", "Anime SD"}, {"445", "Anime IPad"}, {"446", "Anime 720p"}, {"447", "Anime 1080p"}, {"448", "Anime REMUX"}, {"454", "Anime Bluray"}, {"531", "Anime UHD REMUX"}, {"409", "Misc"}, {"449", "Anime 2160p"}, {"509", "Anime 8K/4320p"}, {"501", "Anime UHD Blu-ray"}, {"504", "Sports 2160p"}, {"511", "Sport 8K/4320p"}, {"508", "Doc 8K UHD BD"}, {"529", "Doc 8K UHD BD REMUX"}, {"500", "Doc UHD Blu-ray"}, {"507", "Doc 8K/4320p"}, {"422", "Doc 2160p"}, {"420", "Doc 1080p"}, {"419", "Doc 720p"}, {"417", "Doc SD"}, {"418", "Doc IPad"}, {"424", "TVMusic 1080i"}, {"423", "TVMusic 720p"}, {"452", "TVShows Bluray"}, {"430", "TVShow REMUX"}, {"429", "TVShow 1080p"}, {"428", "TVShow 1080i"}, {"427", "TVShow 720p"}, {"425", "TVShow SD"}, {"426", "TVShow IPad"}},
			MediumSel:     []SelectOption{{"10", "UHD Blu-ray"}, {"1", "Blu-ray"}, {"3", "Remux"}, {"7", "Encode"}, {"5", "HDTV"}, {"8", "CD"}, {"4", "MiniBD"}, {"11", "WEB-DL"}},
			CodecSel:      []SelectOption{{"1", "AVC/H264/x264"}, {"2", "HEVC/H265/x265"}, {"3", "VC-1"}, {"4", "MPEG-2"}, {"5", "Other"}},
			StandardSel:   []SelectOption{{"1", "2160p/4K"}, {"2", "1080p"}, {"3", "1080i"}, {"4", "720p"}, {"5", "SD"}, {"10", "4320p/8K"}},
			AudioCodec:    []SelectOption{{"6", "AAC"}, {"15", "AC3/DD"}, {"2", "APE"}, {"16", "WAV"}, {"1", "FLAC"}, {"3", "DTS"}, {"13", "TrueHD"}, {"14", "LPCM"}, {"11", "DTS-HDMA"}, {"18", "DTS-HDHRA"}, {"12", "TrueHD Atmos"}, {"17", "DTS-HDMA:X 7.1"}, {"7", "Other"}},
			TeamSel:       []SelectOption{{"1", "HDHome"}, {"2", "HDH"}, {"3", "HDHTV"}, {"4", "HDHPad"}, {"12", "HDHWEB"}, {"20", "3201"}, {"17", "SHMA"}, {"21", "TVman"}, {"19", "ARiN"}, {"6", "TTG"}, {"7", "M-Team"}, {"11", "Other"}, {"22", "969154968"}, {"23", "BMDru"}},
			ProcessingSel: []SelectOption{{"1", "Raw"}, {"2", "Encode"}},
			SourceSel:     []SelectOption{{"9", "UHD Blu-ray"}, {"1", "Blu-ray"}, {"4", "HDTV"}, {"3", "DVD"}, {"7", "WEB-DL"}, {"8", "Other"}},
		},
	},
	{
		Domain: "hdsky.me", Name: "天空", BaseURL: "https://hdsky.me",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "Movies/电影"}, {"404", "Documentaries/纪录片"}, {"410", "iPad/iPad影视"}, {"405", "Animations/动漫"}, {"402", "TV Series/剧集(分集）"}, {"411", "TV Series/剧集(合集）"}, {"403", "TV Shows/综艺"}, {"406", "Music Videos/音乐MV"}, {"407", "Sports/体育"}, {"408", "HQ Audio/无损音乐"}, {"409", "Misc/其他"}, {"412", "TV Series/海外剧集(分集）"}, {"413", "TV Series/海外剧集(合集）"}, {"414", "TV Shows/海外综艺(分集）"}, {"415", "TV Shows/海外综艺(合集）"}, {"416", "Shortplay/短剧"}},
			MediumSel:   []SelectOption{{"13", "UHD Blu-ray"}, {"14", "UHD Blu-ray/DIY"}, {"1", "Blu-ray"}, {"12", "Blu-ray/DIY"}, {"3", "Remux"}, {"7", "Encode"}, {"5", "HDTV"}, {"6", "DVDR"}, {"8", "CD"}, {"4", "MiniBD"}, {"9", "Track"}, {"11", "WEB-DL"}, {"15", "SACD"}, {"2", "HD DVD"}, {"16", "3D Blu-ray"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC"}, {"13", "x265"}, {"10", "x264"}, {"12", "HEVC"}, {"2", "VC-1"}, {"4", "MPEG-2"}, {"3", "Xvid"}, {"11", "Other"}, {"14", "MVC"}, {"15", "ProRes"}, {"17", "VP9"}, {"16", "AV1"}},
			StandardSel: []SelectOption{{"5", "4K/2160p"}, {"1", "2K/1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}, {"6", "8K/4320P"}},
			AudioCodec:  []SelectOption{{"10", "DTS-HDMA"}, {"16", "DTS-HDMA:X 7.1"}, {"17", "TrueHD Atmos"}, {"19", "PCM"}, {"11", "TrueHD"}, {"3", "DTS"}, {"13", "LPCM"}, {"1", "FLAC"}, {"2", "APE"}, {"4", "MP3"}, {"5", "OGG"}, {"6", "AAC"}, {"12", "AC3/DD"}, {"7", "Other"}, {"14", "DTS-HD HR"}, {"15", "WAV"}, {"18", "DSD"}, {"22", "Opus"}, {"20", "E-AC3"}, {"21", "DDP with Dolby Atmos"}, {"23", "ALAC"}},
			TeamSel:     []SelectOption{{"6", "HDSky/原盘DIY小组"}, {"1", "HDS/重编码及remux小组"}, {"28", "HDS3D/3D重编码小组"}, {"9", "HDSTV/电视录制小组"}, {"31", "HDSWEB/网络视频小组"}, {"18", "HDSPad/移动视频小组"}, {"22", "HDSCD/无损音乐小组"}, {"34", "HDSpecial|稀缺资源"}, {"24", "Original/自制原创资源"}, {"27", "Other/其他制作组或转发资源"}, {"26", "Autoseed/自动发布机器人"}, {"30", "BMDru小组"}, {"25", "AREA11/韩剧合作小组"}, {"33", "Request/应求发布资源"}, {"35", "HDSWEB/(网络视频小组合集专用)"}, {"36", "HDSAB/有声书小组"}, {"37", "HDSWEB/(补档专用)"}},
		},
	},
	{
		Domain: "hdtime.org", Name: "时光", BaseURL: "https://hdtime.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:   []SelectOption{{"409", "其他"}, {"411", "文档"}, {"410", "游戏"}, {"406", "MV"}, {"408", "音乐"}, {"404", "纪录片"}, {"407", "体育"}, {"414", "软件"}, {"405", "动漫"}, {"403", "综艺"}, {"402", "剧集"}, {"401", "电影"}, {"424", "Blu-Ray原盘"}},
			MediumSel:  []SelectOption{{"9", "Track"}, {"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "HD DVD"}, {"1", "Blu-ray"}, {"10", "WEB-DL"}},
			CodecSel:   []SelectOption{{"1", "AVC/H.264/x264"}, {"12", "HEVC/H.265/x265"}, {"11", "x264-10bit"}, {"15", "AV1"}, {"14", "VP8/VP9"}, {"17", "AVS3"}, {"3", "xvid"}, {"4", "MPEG-2"}, {"2", "VC-1"}, {"16", "VVC/H.266/x266"}, {"5", "Other"}},
			AudioCodec: []SelectOption{{"8", "TrueHD Atmos"}, {"9", "TrueHD"}, {"10", "DTS-HD"}, {"3", "DTS"}, {"11", "DD/AC3"}, {"12", "DDP/EAC3"}, {"13", "LPCM/PCM"}, {"6", "AAC"}, {"14", "Audio Vivid/AV3A"}, {"15", "OPUS"}, {"1", "FLAC"}, {"2", "APE"}, {"4", "MP3"}, {"5", "OGG"}, {"7", "Other"}},
			TeamSel:    []SelectOption{{"9", "个人原创"}, {"5", "Other"}, {"2", "CHD"}, {"3", "beAst"}, {"4", "WiKi"}, {"8", "CMCT"}, {"7", "PADTime"}, {"15", "VTime"}, {"12", "HDT"}, {"6", "HDTime"}, {"16", "QHstudIo"}, {"17", "AilMWeb"}, {"18", "HHWEB"}},
			Tags:       []SelectOption{{"8", "杜比视界"}, {"1", "禁转"}, {"2", "首发"}, {"3", "官方"}, {"4", "DIY"}, {"5", "国语"}, {"6", "中字"}, {"7", "HDR"}},
		},
	},
	{
		Domain: "hdvideo.top", Name: "HDVideo", BaseURL: "https://hdvideo.top",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"406", "音轨"}, {"408", "音乐MV"}, {"407", "体育"}, {"404", "纪录片"}, {"403", "综艺"}, {"405", "动漫"}, {"402", "电视剧"}, {"401", "电影"}},
			MediumSel:   []SelectOption{{"14", "WEB-DL"}, {"16", "UHDTV/HDTV"}, {"10", "UHD Blu-ray"}, {"11", "FHD Blu-ray"}, {"13", "Encode"}, {"12", "Remux"}, {"18", "DVD"}, {"20", "CD"}, {"19", "Other"}},
			CodecSel:    []SelectOption{{"14", "ProRes"}, {"12", "AV1/S"}, {"10", "VP8/9"}, {"8", "VC-1"}, {"9", "MPEG-2"}, {"7", "AVC/H.264/x264"}, {"6", "HEVC/H.265/x265"}, {"11", "Other"}},
			StandardSel: []SelectOption{{"5", "4320p/8K"}, {"6", "2160p/4K"}, {"8", "1080p"}, {"9", "1080i"}, {"10", "720p"}, {"11", "Other"}},
			AudioCodec:  []SelectOption{{"23", "DDP Atmos/EAC3"}, {"14", "TrueHD_Atmos"}, {"8", "LPCM/PCM"}, {"10", "DDP/EAC3"}, {"3", "DTS_HD|MA"}, {"15", "DTS_HD|HR"}, {"9", "DD/AC3"}, {"11", "TrueHD"}, {"16", "DTS_X"}, {"18", "MP2/3"}, {"22", "MPEG"}, {"5", "Opus"}, {"1", "FLAC"}, {"20", "OGG"}, {"12", "WAV"}, {"17", "DTS"}, {"19", "TAA"}, {"21", "AAC"}, {"2", "APE"}, {"7", "Other"}},
			TeamSel:     []SelectOption{{"2", "HDVMV"}, {"1", "HDVWEB"}, {"5", "HDV"}, {"4", "Other"}},
			Tags:        []SelectOption{{"1", "禁转"}, {"9", "限转"}, {"3", "官方"}, {"28", "连载"}, {"17", "完结"}, {"18", "原创"}, {"29", "首发"}, {"31", "微短剧"}, {"30", "漫剧"}, {"5", "国语"}, {"10", "粤语"}, {"6", "中字"}, {"19", "特效"}, {"22", "DIY"}, {"12", "Dolby Vision"}, {"11", "HDR10"}, {"20", "HDR10+"}, {"27", "HDR Vivid"}, {"13", "HLG"}, {"8", "源码"}, {"16", "金种"}, {"21", "应求"}, {"15", "零魔"}, {"23", "MV"}, {"24", "LIVE现场"}, {"26", "演唱会"}, {"25", "音乐专辑"}},
		},
	},
	{
		Domain: "lemonhd.net", Name: "柠檬不甜", BaseURL: "https://lemonhd.net",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "Movies(电影)"}, {"402", "Muisc(音乐)"}, {"403", "Animations(动漫/动画)"}, {"404", "Music Videos(音乐视频)"}, {"405", "Documentaries(纪录片)"}, {"406", "TV Series(电视剧)"}, {"407", "TV Shows(综艺)"}, {"408", "3D(3D视频)"}, {"409", "Other(其它)"}},
			MediumSel:   []SelectOption{{"1", "Blu-ray"}, {"2", "Encode"}, {"3", "Remux"}, {"4", "WEB-DL"}, {"5", "HDTV"}, {"6", "4K-UltraHD"}, {"7", "8K-UltraHD"}, {"8", "DVD"}, {"9", "CD"}, {"10", "Other"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC"}, {"2", "H.265/HEVC"}, {"3", "VC-1"}, {"4", "MPEG-2"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"1", "4K_UHD"}, {"2", "1080p/i"}, {"3", "720p/i"}, {"4", "SD"}, {"5", "Other"}},
			AudioCodec:  []SelectOption{{"1", "Atmos TrueHD"}, {"2", "TrueHD"}, {"3", "DTS-HD MA"}, {"4", "DTS X"}, {"5", "DTS"}, {"6", "AC3/DD"}, {"7", "EAC3/DDP"}, {"8", "AAC"}, {"9", "LPCM"}, {"10", "FLAC"}, {"11", "WAV"}, {"12", "APE"}, {"13", "Other"}},
			TeamSel:     []SelectOption{{"6", "LemonHD"}, {"1", "HDS"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}},
			Tags:        []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"5", "国语"}, {"6", "中字"}, {"8", "4K"}, {"7", "HDR"}, {"9", "原盘"}},
		},
	},
	{
		Domain: "mua.xloli.cc", Name: "萝莉", BaseURL: "https://mua.xloli.cc",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影 (Movie)"}, {"402", "电视剧(TV Series)"}, {"430", "综艺(TV Show)"}, {"405", "动画 (Animation)"}, {"408", "音乐 (Music)"}, {"410", "舞台演出 (Stage Performance)"}, {"404", "纪录片 (Documentary)"}, {"412", "游戏 (Game)"}, {"413", "软件 (Software)"}, {"411", "漫画/CG杂图/动漫杂志"}},
			MediumSel:   []SelectOption{{"14", "UHD Blu-ray"}, {"1", "Blu-ray"}, {"3", "Remux"}, {"7", "Encode"}, {"5", "HDTV"}, {"12", "WEB-DL"}, {"2", "DVD"}, {"8", "CD"}, {"11", "Other"}},
			CodecSel:    []SelectOption{{"9", "AVC/x264"}, {"8", "HEVC/x265"}, {"15", "AV1"}, {"12", "VC-1"}, {"13", "MPEG-2"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"5", "2160p/4K"}, {"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}, {"9", "Other"}},
			AudioCodec:  []SelectOption{{"6", "AAC"}, {"13", "AC3/DD"}, {"20", "DDP/E-AC3"}, {"21", "MP3"}, {"1", "FLAC"}, {"3", "DTS"}, {"9", "TrueHD"}, {"15", "LPCM"}, {"10", "DTS-HD MA"}, {"23", "DTS:X"}, {"24", "OPUS"}, {"14", "Other"}},
			TeamSel:     []SelectOption{{"11", "VCB-Studio"}, {"15", "7³ACG"}, {"42", "AI-Raws"}, {"48", "ANK-Raws"}, {"49", "LittleBakas!"}, {"50", "mawen1250"}, {"40", "jsum@U2"}, {"53", "Moozzi2"}, {"39", "Snow-Raws"}, {"46", "CMCT"}, {"41", "GodDramas"}, {"47", "beAst"}, {"5", "Other"}},
			SourceSel:   []SelectOption{{"6", "日本 (JPN)"}, {"1", "大陆 (CHN)"}, {"4", "欧美 (West)"}, {"3", "台湾 (TWN)"}, {"5", "韩国 (KOR)"}, {"2", "香港 (HKG)"}, {"7", "印度 (IND)"}, {"8", "俄国 (RUS)"}, {"11", "泰国 (THA)"}, {"13", "其它 (Other)"}},
			Tags:        []SelectOption{{"8", "R18"}, {"1", "禁转"}, {"20", "原创"}, {"15", "自购"}, {"2", "首发"}, {"5", "国语"}, {"23", "粤语"}, {"6", "中字"}, {"18", "ENSub"}, {"22", "特效"}, {"4", "DIY"}, {"13", "DoVi"}, {"7", "HDR10"}, {"14", "HDR10+"}, {"16", "Atmos"}, {"9", "原生原盘"}, {"11", "LOLI"}, {"24", "无对白"}, {"19", "GalGame"}, {"21", "分集"}, {"10", "其他"}},
		},
	},
	{
		Domain: "nanyangpt.com", Name: "南洋", BaseURL: "https://nanyangpt.com",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category: []SelectOption{{"401", "电影"}, {"402", "剧集"}, {"403", "动漫"}, {"404", "综艺"}, {"405", "体育"}, {"406", "纪录"}, {"407", "音乐"}, {"408", "学习"}, {"409", "软件"}, {"410", "游戏"}, {"411", "其它"}},
		},
	},
	{
		Domain: "njtupt.top", Name: "浦园", BaseURL: "https://njtupt.top",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影"}, {"402", "剧集"}, {"403", "综艺"}, {"405", "动漫"}, {"406", "演出"}, {"422", "音乐"}, {"404", "纪录片"}, {"407", "体育"}, {"415", "资料"}, {"425", "游戏"}, {"428", "软件"}, {"409", "其他"}},
			MediumSel:   []SelectOption{{"9", "Blu-ray"}, {"8", "Remux"}, {"5", "WEB-DL"}, {"7", "Encode"}, {"6", "HDTV"}, {"2", "DVD"}, {"10", "Other"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC"}, {"3", "H.265/HEVC"}, {"6", "AV1"}, {"2", "VC-1"}, {"4", "MPEG-2"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"5", "2160p"}, {"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}},
			Tags:        []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"3", "官方"}, {"8", "自购"}, {"5", "国语"}, {"6", "中字"}, {"11", "应求"}, {"10", "保种"}},
		},
	},
	{
		Domain: "p.t-baozi.cc", Name: "包子", BaseURL: "https://p.t-baozi.cc",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影"}, {"402", "剧集"}, {"403", "综艺"}, {"404", "纪录片"}, {"405", "动漫"}, {"406", "音乐视频"}, {"407", "体育运动"}, {"408", "高品质音频"}, {"410", "短剧"}, {"409", "其他"}},
			MediumSel:   []SelectOption{{"13", "Blu-ray DIY"}, {"1", "Blu-ray 原盘"}, {"12", "UHD Blu-ray DIY"}, {"11", "UHD Blu-ray 原盘"}, {"10", "WEB-DL"}, {"9", "Track"}, {"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "HD DVD"}},
			CodecSel:    []SelectOption{{"6", "H.265(HEVC)"}, {"1", "H.264(AVC)"}, {"2", "VC-1"}, {"4", "MPEG-2"}, {"3", "AV1"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"6", "8K"}, {"5", "4K"}, {"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}, {"7", "None"}},
			AudioCodec:  []SelectOption{{"8", "DTS:X"}, {"9", "TrueHD Atmos"}, {"10", "DTS-HD MA"}, {"11", "TrueHD"}, {"12", "LPCM"}, {"3", "DTS"}, {"1", "DD/AC3"}, {"2", "OPUS"}, {"6", "AAC"}, {"13", "FLAC"}, {"14", "APE"}, {"15", "WAV"}, {"4", "MP3"}, {"5", "M4A"}, {"7", "Other"}},
			TeamSel:     []SelectOption{{"6", "BAOZIWEB"}, {"7", "Baozi"}, {"1", "HDS"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}},
			Tags:        []SelectOption{{"9", "原创"}, {"5", "国语"}, {"8", "粤语"}, {"2", "首发"}, {"6", "中字"}, {"12", "完结"}, {"4", "DIY"}, {"11", "动画"}, {"13", "Dolby Vision"}, {"14", "HDR10"}, {"15", "HDR10+"}, {"1", "禁转"}, {"16", "限转"}, {"17", "应求"}, {"18", "MV"}, {"19", "卡拉OK"}, {"20", "LIVE现场"}, {"21", "演唱会"}, {"23", "成人"}, {"7", "HDR"}, {"22", "音乐专辑"}},
		},
	},
	{
		Domain: "pandapt.net", Name: "熊猫", BaseURL: "https://pandapt.net",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影"}, {"402", "电视剧"}, {"415", "短剧"}, {"405", "动漫"}, {"403", "综艺"}, {"404", "纪录片"}, {"407", "体育"}, {"412", "软件"}, {"411", "游戏"}, {"413", "演唱会/音乐会"}, {"409", "其他"}},
			MediumSel:   []SelectOption{{"11", "UHD Blu-ray"}, {"1", "Blu-ray"}, {"3", "Remux"}, {"7", "Encode"}, {"9", "Track"}, {"10", "WEB-DL"}, {"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"12", "UHDTV"}, {"4", "MiniBD"}, {"2", "HD DVD"}},
			CodecSel:    []SelectOption{{"6", "HEVC/H.265/x265"}, {"1", "AVC/H.264/x264"}, {"7", "VP8/VP9"}, {"2", "VC-1"}, {"3", "Xvid"}, {"4", "MPEG-2"}, {"5", "Other"}, {"8", "AV1"}},
			StandardSel: []SelectOption{{"6", "4320k/8K"}, {"5", "2160p/4K"}, {"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"7", "SD"}},
			AudioCodec:  []SelectOption{{"19", "AV3A"}, {"8", "TrueHD Atmos"}, {"1", "TrueHD"}, {"2", "DTS-X"}, {"3", "DTS-HD"}, {"18", "DTS-HR"}, {"4", "DTS"}, {"5", "DD/AC3"}, {"6", "DDP/EAC3"}, {"7", "AAC"}, {"11", "FLAC"}, {"9", "LPCM/PCM"}, {"16", "Other"}},
			TeamSel:     []SelectOption{{"6", "Panda(原盘diy组)"}, {"10", "Panda(原盘Remux组)"}, {"1", "Panda(压制组)"}, {"7", "AilMWeb(流媒体组)"}, {"8", "AilMTV(电视录制组)"}, {"14", "AilMUpscale(超分视频组)"}, {"15", "CatEDU(部分禁转)"}, {"22", "Red Leaves (红叶)"}, {"5", "Other"}},
			SourceSel:   []SelectOption{{"7", "CHN(中国大陆)"}, {"1", "EU/US(欧美)"}, {"2", "HK/MAC/TW(港澳台地区)"}, {"3", "JPN(日本)"}, {"4", "KOR(韩国)"}, {"9", "IND(印度)"}, {"8", "SEA(东南亚)"}, {"6", "Other(其他)"}},
			Tags:        []SelectOption{{"1", "禁转"}, {"20", "驻站"}, {"16", "纯净版"}, {"4", "DIY"}, {"10", "完结"}, {"17", "分集"}, {"5", "国语"}, {"13", "粤语"}, {"6", "中字"}, {"12", "特效"}, {"8", "杜比视界"}, {"9", "HDR10+"}, {"7", "HDR10"}, {"15", "菁彩HDR"}, {"22", "HLG"}, {"2", "横屏HOR"}, {"23", "竖屏VER"}, {"11", "应求"}, {"14", "自购"}},
		},
	},
	{
		Domain: "piggo.me", Name: "二师兄", BaseURL: "https://piggo.me",
		Framework: "nexusphp", IsSource: true, IsTarget: false,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"907", "纪录片"}, {"910", "读书绘本"}, {"911", "音乐"}, {"905", "有声读物"}, {"909", "儿童电影"}, {"908", "儿童剧集"}},
			MediumSel:     []SelectOption{{"11", "Untouched"}, {"3", "Remux"}, {"7", "Encode"}, {"5", "DIY"}, {"8", "Other"}},
			CodecSel:      []SelectOption{{"1", "H.264/X.264"}, {"6", "H.265/X.265"}, {"5", "Other"}},
			StandardSel:   []SelectOption{{"3", "720p"}, {"1", "1080p/i"}, {"5", "4K"}, {"6", "other"}},
			AudioCodec:    []SelectOption{{"1", "FLAC"}, {"2", "APE"}, {"3", "DTS"}, {"4", "MP3"}, {"6", "AAC"}, {"7", "Other"}, {"8", "AC-3"}, {"9", "DTS-HD MA"}, {"10", "TrueHD"}, {"11", "LPCM"}},
			TeamSel:       []SelectOption{{"8", "PigoWeb"}, {"7", "PigoHD"}, {"9", "PigoNF"}, {"10", "PigoAD"}, {"5", "Other"}},
			ProcessingSel: []SelectOption{{"4", "DV（杜比视界)"}, {"3", "HDR10+"}, {"2", "HDR10"}, {"1", "SDR"}},
			SourceSel:     []SelectOption{{"3", "DVD"}, {"5", "TV"}, {"7", "WEB-DL"}, {"1", "Blu-ray"}, {"6", "Other"}},
			Tags:          []SelectOption{{"20", "自购"}, {"17", "独占"}, {"1", "禁转"}, {"11", "合集"}, {"13", "完结"}, {"5", "国语"}, {"6", "中字"}, {"7", "HDR 10"}, {"21", "中英双语"}, {"16", "应求"}, {"15", "怀旧计划"}, {"14", "追更"}, {"12", "Dolby Atmos"}, {"10", "特效"}, {"9", "Dolby Vision"}, {"8", "3D"}, {"18", "分集"}},
		},
	},
	{
		Domain: "pt.0ff.cc", Name: "农场", BaseURL: "https://pt.0ff.cc",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"401", "Movies|电影"}, {"402", "TV Series|电视剧"}, {"403", "Documentaries|纪录片"}, {"404", "TV Shows|综艺"}, {"405", "Animations|动漫"}, {"406", "Music Videos|音乐视频"}, {"407", "HQ Audio|无损音乐"}, {"408", "Sports|体育"}, {"427", "Education|学习教育"}, {"412", "book|书籍"}, {"411", "Game|游戏"}, {"410", "Resources|网络资源"}, {"409", "待定"}, {"428", "Other|其他"}},
			MediumSel:     []SelectOption{{"1", "UHD Blu-ray"}, {"2", "Blu-ray"}, {"3", "UHDTV"}, {"4", "IPTV"}, {"5", "Remux"}, {"6", "Encode"}, {"7", "HDTV"}, {"8", "DVDR"}, {"9", "CD"}, {"10", "MiniBD"}, {"11", "Track"}, {"12", "WEB-DL"}, {"13", "SACD"}, {"14", "HD DVD"}, {"15", "3D Blu-ray"}, {"21", "其它 (Other)"}},
			CodecSel:      []SelectOption{{"1", "H.265/HEVC"}, {"2", "H.264/AVC"}, {"3", "X265"}, {"4", "X264"}, {"5", "HEVC"}, {"6", "VC-1"}, {"7", "MPEG-4"}, {"8", "MPEG-2"}, {"9", "Xvid"}, {"10", "MVC"}, {"11", "ProRes"}, {"12", "VP8/9"}, {"13", "AV1"}, {"14", "其它 (Other)"}},
			StandardSel:   []SelectOption{{"1", "8K/4320P"}, {"2", "4K/2160P"}, {"3", "1080P"}, {"4", "1440P"}, {"5", "1080I"}, {"6", "720P"}, {"7", "SD"}, {"14", "其它 (Other)"}},
			AudioCodec:    []SelectOption{{"10", "DTS-HDMA:X 7.1"}, {"22", "其它 (Other)"}, {"21", "OGG"}, {"17", "PCM"}, {"16", "DSD"}, {"15", "APE"}, {"14", "DTS"}, {"13", "AAC"}, {"12", "MP3"}, {"11", "WAV"}, {"1", "TrueHD Atmos"}, {"9", "FLAC"}, {"8", "OPUS"}, {"7", "LPCM"}, {"6", "DTS-X"}, {"5", "TrueHD"}, {"4", "AC3|DD"}, {"3", "DTS-HD|HR"}, {"2", "DTS-HD|MA"}},
			TeamSel:       []SelectOption{{"15", "FFWEB"}, {"14", "其它 (Other)"}},
			ProcessingSel: []SelectOption{{"34", "一年级"}, {"35", "二年级"}, {"36", "三年级"}, {"37", "四年级"}, {"38", "五年级"}, {"39", "六年级"}, {"40", "七年级"}, {"41", "八年级"}, {"42", "九年级"}, {"43", "高一"}, {"44", "高二"}, {"45", "高三"}, {"46", "大学"}},
			SourceSel:     []SelectOption{{"35", "内地 (China,CHN)"}, {"36", "香港 (HKG,CHN)"}, {"37", "台湾 (TWN,CHN)"}, {"39", "韩国 (KOR)"}, {"38", "欧美 (Western)"}, {"40", "日本 (JPN)"}, {"41", "印度 (IND)"}, {"34", "泰国 (Th)"}, {"42", "其它 (Other)"}},
			Tags:          []SelectOption{{"12", "连载"}, {"10", "完结"}, {"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"5", "国语"}, {"6", "中字"}, {"7", "HDR"}, {"8", "短剧"}, {"11", "3D"}},
		},
	},
	{
		Domain: "pt.aling.de", Name: "阿玲", BaseURL: "https://pt.aling.de",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"405", "动画"}, {"402", "电视剧"}, {"401", "电影"}, {"404", "纪录片"}},
			MediumSel:   []SelectOption{{"1", "Blu-ray"}, {"2", "Blu-ray DIY"}, {"3", "Remux"}, {"7", "Encode"}, {"4", "TV"}, {"5", "WEB-DL"}, {"6", "DVD 原盘"}, {"8", "Other"}},
			CodecSel:    []SelectOption{{"1", "H.264(AVC)"}, {"2", "H.265(HEVC)"}, {"3", "AV1"}, {"4", "MPEG-2"}, {"5", "VC-1"}, {"6", "Other"}},
			StandardSel: []SelectOption{{"1", "8K"}, {"2", "4K"}, {"3", "1080p"}, {"4", "720p"}, {"5", "SD"}},
			TeamSel:     []SelectOption{{"1", "aling"}, {"2", "alingWEB"}, {"3", "Other"}},
			SourceSel:   []SelectOption{{"1", "内地"}, {"2", "香港"}, {"3", "台湾"}, {"4", "日本"}, {"5", "朝鲜"}, {"6", "印度"}, {"7", "印尼"}, {"8", "泰国"}, {"9", "苏联"}, {"10", "欧米"}, {"11", "其他"}},
			Tags:        []SelectOption{{"12", "国语"}, {"13", "粤语"}, {"10", "其他中国方言"}, {"6", "中字"}, {"1", "禁转"}, {"14", "Dolby Vision"}, {"4", "DIY"}, {"7", "HDR"}},
		},
	},
	{
		Domain: "pt.btschool.club", Name: "学校", BaseURL: "https://pt.btschool.club",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"405", "电影/Movies"}, {"406", "连续剧/TV-Series"}, {"407", "动漫/Animation"}, {"408", "纪录片/Documentary"}, {"412", "综艺/TV-Show"}, {"404", "软件/Software"}, {"402", "资料/Education"}, {"411", "游戏/Game"}, {"409", "音乐/Music"}, {"410", "体育/Sports"}, {"415", "其他/Other"}},
			MediumSel:   []SelectOption{{"1", "Blu-ray"}, {"12", "Blu-rayUHD"}, {"7", "Encode"}, {"10", "WEB-DL"}, {"3", "Remux"}, {"5", "HDTV"}, {"6", "DVDRip"}, {"8", "CD"}, {"11", "Other"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC/x264"}, {"10", "H.265/HEVC/x265"}, {"4", "MPEG-2"}, {"2", "VC-1"}, {"3", "Xvid"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"5", "4K_UHD"}, {"1", "1080p/i"}, {"3", "720p/i"}, {"4", "SD"}, {"6", "Other"}},
			AudioCodec:  []SelectOption{{"11", "TrueHD"}, {"3", "DTS-HD/DTS"}, {"10", "AC3"}, {"5", "LPCM"}, {"1", "FLAC"}, {"4", "MP3"}, {"6", "AAC"}, {"2", "APE"}, {"7", "Other"}},
			TeamSel:     []SelectOption{{"1", "BTSCHOOL"}, {"13", "Zone"}, {"2", "BtsHD"}, {"3", "BtsTV"}, {"4", "BtsPAD"}, {"5", "WiKi"}, {"6", "HDChina"}, {"7", "HDB_iNT"}, {"9", "M-team"}, {"10", "CMCT"}, {"11", "Ourbits"}, {"12", "Other"}},
		},
	},
	{
		Domain: "pt.cdy.skin", Name: "传道院", BaseURL: "http://pt.cdy.skin",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "Movies"}, {"402", "TV Series"}, {"403", "TV Shows"}, {"404", "Documentaries"}, {"405", "Animations"}, {"406", "Music Videos"}, {"407", "Sports"}, {"408", "HQ Audio"}, {"409", "Misc"}, {"410", "Game"}, {"411", "Program"}},
			MediumSel:   []SelectOption{{"1", "Blu-ray"}, {"2", "HD DVD"}, {"3", "Remux"}, {"4", "MiniBD"}, {"5", "HDTV"}, {"6", "DVDR"}, {"7", "Encode"}, {"8", "CD"}, {"9", "Track"}},
			CodecSel:    []SelectOption{{"1", "H.264"}, {"6", "H.265"}, {"2", "VC-1"}, {"3", "Xvid"}, {"4", "MPEG-2"}, {"5", "Other"}, {"7", "AV1"}},
			AudioCodec:  []SelectOption{{"8", "Dolby Atmos"}, {"9", "DTS:X"}, {"10", "DTS-HD MA"}, {"12", "Dolby TrueHD"}, {"13", "LPCM"}, {"14", "DDP\\E-AC-3"}, {"15", "DD/AC-3"}, {"16", "AAC"}, {"17", "Other"}},
			StandardSel: []SelectOption{{"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}, {"5", "2160p"}, {"6", "Other"}},
			TeamSel:     []SelectOption{{"1", "HDS"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}, {"6", "FRDS"}, {"7", "Mteam"}, {"8", "HHCLUB"}},
		},
	},
	{
		Domain: "bilibili.download", Name: "轨道炮", BaseURL: "https://bilibili.download",
		Framework: "nexusphp", IsSource: false, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影"}, {"402", "剧集"}, {"403", "综艺"}, {"404", "纪录片"}, {"405", "动漫"}, {"406", "MV"}, {"407", "体育"}, {"408", "音乐"}, {"409", "misc"}, {"410", "软件"}, {"411", "学习"}, {"412", "游戏"}, {"419", "漫画"}, {"420", "下架视频备份"}},
			MediumSel:   []SelectOption{{"1", "Blu-ray"}, {"2", "UHD"}, {"3", "Remux"}, {"4", "WEB-DL"}, {"5", "HDTV"}, {"6", "DVD"}, {"7", "Encode"}, {"8", "CD"}, {"9", "Track"}},
			CodecSel:    []SelectOption{{"1", "H264"}, {"2", "H265"}, {"3", "VC-1"}, {"4", "MPEG-2"}, {"5", "XVID"}, {"6", "Other"}},
			StandardSel: []SelectOption{{"1", "4K"}, {"2", "1080p/i"}, {"3", "720p"}, {"4", "SD"}, {"5", "Other"}, {"6", "2K"}},
			AudioCodec:  []SelectOption{{"1", "TrueHD/Atmos"}, {"2", "DTS-HD/DTS-HDMA"}, {"3", "AC3"}, {"4", "LPCM"}, {"5", "Flac"}, {"6", "MP3"}, {"7", "AAC"}, {"8", "APE"}, {"9", "Other"}, {"10", "WAV"}},
		},
	},
	{
		Domain: "pt.eastgame.org", Name: "TLF", BaseURL: "https://pt.eastgame.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"438", "电影 (Movie)"}, {"440", "电视剧(TV series)"}, {"441", "综艺 (TV Show)"}, {"442", "动漫 (Anime)"}, {"443", "纪录片 (Documentary)"}, {"444", "体育 (Sport)"}, {"445", "音乐视频 (Music Video)"}, {"446", "音乐(Music)"}, {"447", "游戏 (Game)"}, {"448", "软件 (Software)"}, {"449", "资料（E-Learning）"}, {"450", "其它 (Other)"}},
			MediumSel:     []SelectOption{{"7", "Encode"}, {"10", "UHD Blu-ray"}, {"1", "Blu-ray/ HD DVD"}, {"3", "Remux"}, {"4", "WEB-DL"}, {"5", "HDTV"}, {"6", "DVDR"}, {"8", "CD"}, {"9", "Other"}},
			CodecSel:      []SelectOption{{"1", "H264/x264/AVC"}, {"6", "H265/x265/HEVC"}, {"4", "MPEG-2"}, {"2", "VC-1"}, {"3", "Xvid"}, {"5", "Other"}},
			StandardSel:   []SelectOption{{"4", "SD"}, {"3", "720p"}, {"2", "1080i"}, {"1", "1080p"}, {"7", "2K"}, {"6", "2160p/4K"}, {"5", "Other"}},
			AudioCodec:    []SelectOption{{"12", "LPCM"}, {"13", "Dolby Atmos"}, {"14", "Dolby TrueHD"}, {"9", "Dolby Digital/AC3"}, {"11", "DTS-HD MA"}, {"10", "DTS X"}, {"3", "DTS"}, {"1", "FLAC"}, {"2", "APE"}, {"8", "WAV"}, {"6", "AAC"}, {"15", "Opus"}, {"4", "MP3"}, {"5", "OGG"}, {"7", "Other"}},
			TeamSel:       []SelectOption{{"1", "TLF HALFCD TeaM"}, {"2", "TLF iNT TeaM"}, {"8", "个人原创"}, {"5", "Other"}},
			ProcessingSel: []SelectOption{{"1", "CN(中国大陆)"}, {"3", "HK/TW(港台)"}, {"4", "JP(日)"}, {"5", "KR(韩)"}, {"2", "US/EU(欧美)"}, {"6", "OT(其他)"}},
			SourceSel:     []SelectOption{{"16", "0DAY/Scene"}, {"17", "P2P/Non-Scene"}},
		},
	},
	{
		Domain: "pt.hdupt.com", Name: "好多油", BaseURL: "https://pt.hdupt.com",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		AlternativeDomains: `["pt.upxin.net"]`,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"401", "Movies/电影"}, {"402", "TV Series/电视剧"}, {"403", "TV Shows/综艺"}, {"404", "Documentaries/纪录片"}, {"405", "Animations/动画"}, {"406", "Music Videos/音乐 MV"}, {"407", "Sports/体育"}, {"408", "HQ Audio/无损音乐"}, {"411", "Misc/其他"}, {"410", "Games/游戏"}},
			MediumSel:     []SelectOption{{"1", "Blu-ray"}, {"11", "UHD Blu-ray"}, {"5", "HDTV"}, {"6", "DVD"}, {"3", "Remux"}, {"15", "UHD Remux"}, {"16", "UHD Remux TV"}, {"12", "Remux TV"}, {"7", "Encode"}, {"14", "Encode TV"}, {"10", "WEB-DL/WEBRip"}, {"13", "WEB-DL/WEBRip TV"}, {"4", "MiniBD"}, {"8", "CD"}, {"9", "Track"}},
			CodecSel:      []SelectOption{{"1", "H.264/AVC"}, {"14", "H.265/HEVC"}, {"2", "VC-1"}, {"16", "x264"}, {"3", "Xvid"}, {"18", "MPEG/MPEG-2"}, {"5", "Other"}},
			StandardSel:   []SelectOption{{"1", "1080p"}, {"2", "1080i"}, {"5", "4K/2160p"}, {"3", "720p"}, {"4", "SD"}, {"6", "iPad"}},
			AudioCodec:    []SelectOption{{"16", "DTS:X"}, {"1", "DTS-HDMA"}, {"3", "TrueHD"}, {"11", "LPCM"}, {"4", "DTS"}, {"2", "AC3/EAC3"}, {"6", "AAC"}, {"7", "FLAC"}, {"10", "APE"}, {"17", "WAV"}, {"18", "MPEG"}, {"13", "Other"}},
			TeamSel:       []SelectOption{{"2", "HDU"}, {"5", "Other"}},
			ProcessingSel: []SelectOption{{"1", "CN/中国内地"}, {"3", "HK/TW/港台"}, {"2", "US/EU/欧美"}, {"4", "JP/日本"}, {"5", "KR/韩国"}, {"6", "India/印度"}, {"8", "SEA/东南亚"}, {"7", "Other"}},
		},
	},
	{
		Domain: "pt.luckpt.de", Name: "幸运", BaseURL: "https://pt.luckpt.de",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影"}, {"402", "电视剧"}, {"405", "动画"}, {"406", "MV"}, {"408", "音乐"}, {"409", "其他"}, {"410", "综艺"}, {"411", "纪录片"}, {"412", "体育"}, {"413", "短剧"}},
			MediumSel:   []SelectOption{{"9", "Track"}, {"8", "CD"}, {"6", "DVD"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"1", "Blu-ray"}, {"10", "UHD Blu-ray"}, {"11", "WEB-DL"}, {"13", "Other"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC"}, {"2", "AV1"}, {"3", "VC-1"}, {"4", "MPEG-2"}, {"5", "Other"}, {"6", "H.265/HEVC"}, {"12", "MPEG-4/XviD"}},
			StandardSel: []SelectOption{{"1", "1080p/1080i"}, {"3", "720p/720i"}, {"4", "480p/480i"}, {"5", "2K/1440p/1440i"}, {"6", "4K/2160p/2160i"}, {"7", "8K/4320p/4320i"}, {"8", "Other"}},
			AudioCodec:  []SelectOption{{"11", "TrueHD Atmos"}, {"19", "PCM"}, {"18", "WAV"}, {"17", "M4A"}, {"16", "DTS-HD MA"}, {"15", "DTS:X"}, {"14", "TrueHD"}, {"13", "LPCM"}, {"12", "DDP/E-AC3"}, {"1", "FLAC"}, {"8", "DD/AC3"}, {"7", "Other"}, {"6", "AAC"}, {"5", "OGG"}, {"4", "MP3"}, {"3", "DTS"}, {"2", "APE"}},
			TeamSel:     []SelectOption{{"5", "Other"}, {"7", "LuckWeb"}, {"8", "LuckMusic"}, {"9", "FRDS"}, {"10", "StarfallWeb"}, {"11", "LuckAni"}, {"12", "LuckDIY"}, {"13", "LuckDocu"}},
			Tags:        []SelectOption{{"2", "首发"}, {"23", "原创"}, {"6", "中字"}, {"5", "国语"}, {"10", "完结"}, {"1", "禁转"}, {"4", "DIY"}, {"22", "英语"}, {"21", "菁彩HDR"}, {"20", "Dolby Vision"}, {"19", "HDR10"}, {"18", "HDR10+"}, {"17", "合集"}, {"16", "特效"}, {"14", "粤语"}, {"11", "大包"}, {"9", "连载"}},
		},
	},
	{
		Domain: "pt.muxuege.org", Name: "慕雪阁", BaseURL: "https://pt.muxuege.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影"}, {"402", "电视剧"}, {"403", "综艺"}, {"404", "纪录片"}, {"405", "动漫"}, {"406", "Music Videos"}, {"407", "体育"}, {"408", "音乐"}, {"418", "广播剧"}, {"417", "有声书"}, {"416", "短剧"}, {"415", "软件"}, {"414", "图片"}, {"413", "教育"}, {"412", "电子书"}, {"411", "游戏"}, {"410", "系统镜像"}, {"409", "其他"}},
			MediumSel:   []SelectOption{{"9", "Track"}, {"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "HD DVD"}, {"1", "Blu-ray"}, {"10", "web"}},
			CodecSel:    []SelectOption{{"1", "H.264 & AVC"}, {"2", "VC-1"}, {"3", "Xvid"}, {"4", "MPEG-2"}, {"5", "Other"}, {"6", "H.265 & HEVC"}, {"7", "AV1"}, {"8", "VP9"}, {"9", "HDR 10"}, {"10", "TXT"}, {"11", "PDF"}},
			StandardSel: []SelectOption{{"1", "1080p"}, {"3", "720p"}, {"4", "SD"}, {"5", "2160p"}, {"6", "540p"}},
			TeamSel:     []SelectOption{{"6", "StarfallWeb星陨阁"}, {"7", "MxWeb慕雪阁"}, {"47", "NovaHD"}, {"26", "FraMeSToR"}, {"24", "DISC"}, {"27", "FLUX"}, {"28", "EPSiLON"}, {"29", "BLUTONIUM"}, {"30", "TAoE"}, {"31", "DECADE"}, {"32", "TEPES"}, {"33", "SWTYBLZ"}, {"34", "NTb"}, {"35", "PLAYERS"}, {"36", "AMA"}, {"37", "CM"}, {"38", "BTN"}, {"39", "BBC"}, {"40", "MvGroup"}, {"41", "zmWeb"}, {"42", "AGSVWEB"}, {"43", "TLF HALFCD TeaM"}, {"44", "ADweb"}, {"45", "Audies"}, {"46", "TPAudio躺平"}, {"14", "UBWEB"}, {"2", "CHDbits"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}, {"8", "MTeam"}, {"9", "3Mweb 三月传媒"}, {"10", "AiMWeb 熊猫"}, {"11", "Tmp"}, {"12", "TPTV"}, {"13", "Mweb"}, {"25", "CiNEFiLE"}, {"15", "Ubits"}, {"16", "QHstudIo"}, {"17", "DBTV"}, {"18", "Qurbits"}, {"19", "NatureWeb"}, {"20", "CHD"}, {"21", "BeyondHD"}, {"22", "HDHome"}, {"23", "BLU"}, {"1", "HDSky 高清天空"}},
			Tags:        []SelectOption{{"33", "珍宝楼"}, {"30", "乐府"}, {"41", "高码率"}, {"37", "DIY"}, {"36", "压制"}, {"27", "原盘"}, {"35", "杜比全景声"}, {"34", "杜比视频"}, {"25", "杜比视界"}, {"26", "完结"}, {"12", "分集"}, {"11", "完结日漫"}, {"8", "完结国漫"}, {"57", "NovaHD"}, {"56", "linux"}, {"55", "Windows"}, {"54", "Mac"}, {"53", "H.265"}, {"52", "H.264"}, {"51", "4k"}, {"50", "1080p"}, {"49", "纯净版"}, {"48", "HDR 真彩"}, {"47", "古装"}, {"46", "菜单修改"}, {"45", "自购"}, {"42", "特效字慕"}, {"40", "生肉"}, {"38", "国语"}, {"32", "大包"}, {"10", "国风音乐"}},
		},
	},
	{
		Domain: "pt.novahd.top", Name: "Nova高清", BaseURL: "https://pt.novahd.top",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"414", "Game/游戏"}, {"413", "Anime Series/番剧"}, {"412", "Anime/动漫"}, {"411", "Short Play/短剧"}, {"401", "Movies/电影"}, {"402", "TV Series/电视剧"}, {"403", "TV Shows/综艺"}, {"404", "Documentaries/记录片"}, {"405", "Animations/动画"}, {"406", "MV/演唱会"}, {"407", "Sports/体育"}, {"409", "Music/音乐"}, {"410", "Othes/其他"}},
			MediumSel:   []SelectOption{{"12", "DVD"}, {"11", "WEB-DL"}, {"10", "UHD Blu-ray"}, {"9", "Track"}, {"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "HD DVD"}, {"1", "Blu-ray"}},
			CodecSel:    []SelectOption{{"1", "H264/x264/AVC"}, {"2", "VC-1"}, {"3", "Xvid"}, {"4", "MPEG-2"}, {"5", "Other"}, {"6", "H265/HEVC/x265"}},
			StandardSel: []SelectOption{{"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}, {"5", "2160p/4K"}, {"6", "4320p/8K"}, {"7", "2160p/4K 60Fps"}, {"8", "2160p/4K 120Fps"}},
			AudioCodec:  []SelectOption{{"1", "FLAC"}, {"2", "APE"}, {"3", "DTS"}, {"4", "MP3"}, {"5", "OGG"}, {"6", "AAC"}, {"7", "ALAC"}, {"8", "TrueHD Atmos"}, {"9", "DDP/E-AC3"}, {"10", "DD/AC3"}, {"11", "LPCM"}, {"12", "TrueHD"}, {"13", "DTS-HD MA"}, {"14", "DTS:X"}, {"15", "Other"}},
			TeamSel:     []SelectOption{{"1", "HDSky"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}, {"6", "FRDS"}, {"7", "beAst"}, {"8", "CMCT"}, {"9", "TLF"}, {"10", "M-Team"}, {"11", "BeiTai"}, {"12", "AGSV"}, {"13", "HDHome"}, {"14", "TTG"}, {"15", "NHDWeb"}, {"16", "NDJWEB"}},
			Tags:        []SelectOption{{"2", "首发"}, {"4", "DIY"}, {"5", "国语"}, {"6", "中字"}, {"7", "HDR"}, {"21", "60FPS"}, {"20", "10Bit"}, {"19", "高码"}, {"18", "连载"}, {"17", "番组"}, {"15", "特效"}, {"14", "杜比"}, {"13", "大包"}, {"12", "应求"}, {"11", "英字"}, {"10", "完结"}, {"9", "分集"}, {"8", "驻站"}},
		},
	},
	{
		Domain: "pt.sjtu.edu.cn", Name: "葡萄", BaseURL: "https://pt.sjtu.edu.cn",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "华语电影"}, {"402", "欧美电影"}, {"403", "亚洲电影"}, {"406", "纪录片"}, {"407", "港台电视剧"}, {"408", "亚洲电视剧"}, {"409", "大陆电视剧"}, {"410", "欧美电视剧"}, {"411", "大陆综艺节目"}, {"412", "港台综艺节目"}, {"413", "欧美综艺节目"}, {"414", "日韩综艺节目"}, {"420", "华语音乐"}, {"421", "日韩音乐"}, {"422", "欧美音乐"}, {"423", "原声音乐"}, {"425", "古典音乐"}, {"426", "mp3合辑"}, {"427", "Music Videos"}, {"429", "游戏"}, {"431", "动漫"}, {"432", "体育"}, {"434", "软件"}, {"435", "学习"}, {"440", "mac"}, {"451", "校园原创"}, {"450", "其他"}},
			CodecSel:    []SelectOption{{"1", "H.264"}, {"2", "VC-1"}, {"3", "Xvid"}, {"4", "MPEG-2"}, {"5", "FLAC"}, {"6", "APE"}, {"7", "DTS"}, {"8", "AC-3"}, {"9", "其他"}, {"10", "h.265"}},
			StandardSel: []SelectOption{{"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}, {"5", "其他"}, {"6", "4k"}},
		},
	},
	{
		Domain: "pt.tey.cc", Name: "太乙", BaseURL: "https://pt.tey.cc",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "Movies | 电影"}, {"402", "TV Series | 剧集"}, {"404", "Documentaries | 纪录"}, {"403", "TV Shows | 综艺"}, {"405", "Animations | 动漫"}, {"407", "Sports | 体育"}, {"406", "Music Videos | MV"}, {"408", "HQ Audio | 无损"}, {"409", "Other | 其他"}},
			MediumSel:   []SelectOption{{"1", "WEB-DL"}, {"2", "HDTV"}, {"6", "DVDR"}, {"8", "CD"}, {"9", "Track"}},
			CodecSel:    []SelectOption{{"2", "H.264(x264/AVC)"}, {"3", "H.265(x265/HEVC)"}, {"4", "MPEG-2"}, {"5", "VP8/9"}, {"1", "Other"}},
			StandardSel: []SelectOption{{"1", "8K"}, {"2", "4K"}, {"3", "1080p"}, {"4", "1080i"}, {"5", "720p"}},
			AudioCodec:  []SelectOption{{"2", "DTS-HDMA:X 7.1"}, {"3", "DTS-HDMA"}, {"4", "DTS"}, {"5", "TrueHD Atmos"}, {"6", "TrueHD"}, {"7", "E-AC3 Atmos(DDP Atmos)"}, {"8", "E-AC3(DDP)"}, {"9", "AC3(DD)"}, {"10", "AAC"}, {"1", "Other"}},
			TeamSel:     []SelectOption{{"1", "Tey"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}},
			Tags:        []SelectOption{{"2", "完结"}, {"3", "连载"}, {"4", "禁转"}, {"5", "首发"}, {"6", "中字"}, {"11", "杜比"}, {"10", "10bit"}, {"7", "HDR"}, {"8", "短剧"}, {"9", "零魔"}, {"15", "纯享"}, {"14", "粤语"}, {"13", "国配"}, {"12", "DIY​"}},
		},
	},
	{
		Domain: "pt.xingyungept.org", Name: "星陨阁", BaseURL: "https://pt.xingyungept.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"409", "其他"}, {"408", "音频"}, {"406", "MV"}, {"407", "体育"}, {"410", "短剧"}, {"404", "纪录片"}, {"405", "动漫"}, {"403", "综艺"}, {"402", "电视剧"}, {"401", "电影"}},
			MediumSel:   []SelectOption{{"10", "Other"}, {"9", "Track"}, {"8", "CD"}, {"6", "DVD"}, {"5", "HDTV"}, {"4", "WEB-DL"}, {"7", "Encode"}, {"3", "Remux"}, {"1", "Blu-ray"}, {"2", "UHD Blu-ray"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC"}, {"2", "H.265/HEVC"}, {"3", "VC-1"}, {"4", "MPEG-2"}, {"5", "AV1"}, {"6", "Other"}},
			StandardSel: []SelectOption{{"1", "480p/480i"}, {"2", "720p/720i"}, {"3", "1080p/1080i"}, {"4", "4K/2160p/2160i"}, {"5", "8k/4320p/4320i"}, {"6", "Other"}},
			AudioCodec:  []SelectOption{{"15", "ALAC"}, {"14", "AAC"}, {"13", "APE"}, {"12", "TrueHD Atmos"}, {"11", "DDP/E-AC3"}, {"10", "DD/AC3"}, {"9", "LPCM"}, {"8", "TrueHD"}, {"7", "DTS:X"}, {"6", "DTS-HD MA"}, {"5", "DTS"}, {"4", "M4A"}, {"3", "WAV"}, {"2", "MP3"}, {"1", "FLAC"}, {"16", "Other"}, {"17", "OPUS"}, {"18", "AV3V"}},
			TeamSel:     []SelectOption{{"8", "StarfallWeb"}, {"12", "Pure@StarfallWeb"}, {"4", "WiKi"}, {"3", "MySiLU"}, {"1", "HDS"}, {"2", "CHD"}, {"5", "Other"}, {"7", "rainweb"}, {"6", "rain"}, {"9", "AGSVWEB"}, {"10", "Starfall"}, {"11", "NatureWeb"}},
			Tags:        []SelectOption{{"23", "驻站"}, {"5", "国语"}, {"6", "中字"}, {"25", "去头尾广告纯净版"}, {"20", "补帧"}, {"19", "超分"}, {"18", "特效"}, {"17", "大包"}, {"16", "应求"}, {"15", "英字"}, {"14", "韩剧"}, {"13", "美剧"}, {"12", "粤语"}, {"9", "特效字幕"}, {"8", "杜比视界"}, {"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"7", "HDR"}, {"27", "高码率"}, {"26", "高帧率"}, {"24", "原生"}, {"11", "完结"}, {"10", "分集"}},
		},
	},
	{
		Domain: "ptcafe.club", Name: "咖啡", BaseURL: "https://ptcafe.club",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影"}, {"402", "剧集"}, {"403", "综艺"}, {"404", "纪录"}, {"405", "动漫"}, {"406", "演唱"}, {"407", "体育"}, {"408", "音乐"}, {"409", "其他"}},
			MediumSel:   []SelectOption{{"1", "UHD Blu-ray 原盘"}, {"2", "UHD Blu-ray DIY"}, {"3", "UHD Remux"}, {"4", "Blu-ray 原盘"}, {"5", "Blu-ray DIY"}, {"6", "Remux"}, {"7", "Encode"}, {"8", "WEB-DL"}, {"9", "TV"}, {"10", "DVD"}, {"11", "CD"}, {"12", "Track"}, {"13", "Other"}},
			CodecSel:    []SelectOption{{"1", "H.265/HEVC"}, {"2", "H.264/AVC"}, {"3", "X265"}, {"4", "X264"}, {"5", "VC-1"}, {"6", "MPEG-2"}, {"7", "MPEG-4"}, {"8", "XVID"}, {"9", "VP9"}, {"10", "DIVX"}, {"11", "Other"}},
			StandardSel: []SelectOption{{"1", "4320P/8K/FUHD"}, {"2", "2160P/4K/UHD"}, {"3", "1080p/1080i/FHD"}, {"4", "720p/720i/HD"}, {"5", "360p/360i/SD"}, {"6", "Other"}},
			AudioCodec:  []SelectOption{{"1", "DTS-HDMA:X 7.1"}, {"2", "DTS-HDMA"}, {"3", "DTS-HDHR"}, {"4", "DTS-HD"}, {"5", "DTS-X"}, {"6", "LPCM"}, {"7", "AC3"}, {"8", "Atmos"}, {"9", "AAC"}, {"10", "TrueHD"}, {"11", "DTS"}, {"12", "FLAC"}, {"13", "APE"}, {"14", "MP3"}, {"15", "WAV"}, {"16", "OPUS"}, {"17", "OGG"}, {"18", "Other"}},
			TeamSel:     []SelectOption{{"1", "ADE"}, {"2", "ADWeb"}, {"3", "Audies"}, {"4", "beAst"}, {"5", "BeiTai"}, {"6", "BeyondHD"}, {"7", "BtsTV"}, {"8", "CafeTV"}, {"9", "CafeWEB"}, {"10", "CHDBits"}, {"11", "CHDWEB"}, {"12", "CMCT"}, {"13", "DJWEB"}, {"14", "FRDS"}, {"15", "HDCTV"}, {"16", "HDH"}, {"17", "HDHome"}, {"18", "HDSky"}, {"19", "HDSWEB"}, {"20", "HHWEB"}, {"21", "MTeam"}, {"22", "MWeb"}, {"23", "OurBits"}, {"24", "OurTV"}, {"25", "PTCafe"}, {"26", "PTerWEB"}, {"27", "QHstudIo"}, {"28", "TTG"}, {"29", "WiKi"}, {"30", "Other"}},
			SourceSel:   []SelectOption{{"1", "大陆"}, {"2", "港台"}, {"3", "欧美"}, {"4", "日本"}, {"5", "韩国"}, {"6", "印度"}, {"7", "其他"}},
			Tags:        []SelectOption{{"1", "官方"}, {"2", "首发"}, {"3", "完结"}, {"4", "原创"}, {"5", "禁转"}, {"7", "国语"}, {"8", "粤语"}, {"9", "中字"}, {"10", "备胎"}, {"11", "杜比视界"}, {"12", "HDR"}, {"13", "DIY"}, {"14", "应求"}, {"15", "高码高帧"}, {"16", "月月"}},
		},
	},
	{
		Domain: "ptchdbits.co", Name: "彩虹岛", BaseURL: "https://ptchdbits.co",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"401", "Movies"}, {"404", "Documentaries"}, {"405", "Animations"}, {"402", "TV Series"}, {"403", "TV Shows"}, {"406", "Music"}, {"407", "Sports"}, {"409", "Demo"}, {"408", "HQ Audio"}, {"410", "Game"}},
			MediumSel:     []SelectOption{{"1", "Blu-ray"}, {"19", "UHD Blu-ray"}, {"3", "Remux"}, {"4", "Encode"}, {"6", "HDTV"}, {"18", "WEB-DL"}, {"8", "CD"}},
			CodecSel:      []SelectOption{{"1", "H.264/AVC"}, {"5", "H.265"}, {"6", "MPEG-4"}, {"4", "MPEG-2"}, {"2", "VC-1"}, {"3", "AV1"}},
			StandardSel:   []SelectOption{{"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"5", "Other"}, {"7", "8K"}, {"6", "4K"}},
			AudioCodec:    []SelectOption{{"3", "DTS"}, {"7", "AC3"}, {"10", "DTS-HD"}, {"11", "True-HD"}, {"13", "LPCM"}, {"1", "FLAC"}, {"2", "APE"}, {"12", "WAV"}, {"6", "AAC"}, {"14", "ALAC"}},
			TeamSel:       []SelectOption{{"3", "压制组"}, {"14", "CHDBits"}, {"13", "SGNB"}, {"1", "REMUX"}, {"2", "CHDTV"}, {"15", "CHDPAD"}, {"12", "CHDWEB"}, {"11", "CHDHKTV"}, {"8", "OneHD"}, {"16", "blucook"}, {"19", "KAN"}, {"22", "JKCT"}, {"23", "BMDru"}, {"25", "Destiny"}, {"26", "SP"}},
			ProcessingSel: []SelectOption{{"1", "3D"}, {"3", "美剧"}, {"4", "日剧"}, {"5", "港剧"}, {"6", "韩剧"}, {"7", "英剧"}, {"8", "国剧"}, {"9", "台剧"}, {"10", "新剧"}, {"11", "马剧"}, {"13", "合集"}},
			SourceSel:     []SelectOption{{"1", "官方"}, {"7", "转载"}, {"8", "复活区"}, {"9", "原创"}},
		},
	},
	{
		Domain: "ptfans.cc", Name: "PTFans", BaseURL: "https://ptfans.cc",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "Movies(电影)"}, {"404", "TV Series(电视剧)"}, {"405", "TV Shows(综艺)"}, {"406", "Documentaries(纪录片)"}, {"403", "Sport(体育、竞技、武术及相关)"}, {"409", "Games(游戏及相关)"}, {"407", "Music(音乐、专辑、MV、演唱会)"}, {"408", "Art(曲艺、相声、小品、戏曲、舞蹈、歌剧、评书等)"}, {"410", "Science(科学、知识、技能)"}, {"411", "School(应试、考级、职称、初中以上教育)"}, {"412", "Book(书籍、杂志、报刊、有声书)"}, {"413", "Code(IT技术、建模、编程、信息技术、大数据、人工智能）"}, {"414", "Animate(3D动画、2.5次元)"}, {"415", "ACGN(二次元、漫画)"}, {"416", "Baby(婴幼、儿童、早教、小学及相关)"}, {"417", "Resource(素材、数据、图片、文档、模板)"}, {"418", "Software(软件、系统、 程序、APP等)"}},
			MediumSel:   []SelectOption{{"9", "Other"}, {"8", "Encode"}, {"6", "Blu-ray"}, {"5", "Web-DL"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "DVD"}, {"1", "CD"}},
			CodecSel:    []SelectOption{{"1", "H.264(x264/AVC)"}, {"2", "H.265(x265/HEVC)"}, {"3", "Bluray(VC-1)"}, {"4", "Bluray(AVC)"}, {"5", "Bluray(HEVC)"}, {"6", "MPEG-2"}, {"7", "Xvid"}, {"8", "AV1"}, {"9", "Other"}},
			StandardSel: []SelectOption{{"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}, {"5", "4K"}, {"6", "8K"}},
			TeamSel:     []SelectOption{{"1", "HDS"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}},
			Tags:        []SelectOption{{"8", "DoVi"}, {"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"5", "国语"}, {"6", "中字"}, {"7", "HDR"}},
		},
	},
	{
		Domain: "pthome.net", Name: "铂金家", BaseURL: "https://pthome.net",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影"}, {"404", "纪录片"}, {"405", "动漫"}, {"402", "电视剧"}, {"403", "综艺"}, {"407", "体育"}, {"408", "音乐"}, {"410", "游戏"}, {"411", "软件"}, {"412", "学习"}, {"409", "其他"}},
			MediumSel:   []SelectOption{{"12", "UHD Blu-ray"}, {"13", "UHD Blu-ray/DIY"}, {"1", "Blu-ray(原盘)"}, {"14", "Blu-ray/DIY"}, {"3", "REMUX"}, {"5", "HDTV"}, {"15", "encode"}, {"10", "WEB-DL"}, {"2", "DVD(原盘)"}, {"8", "CD"}, {"9", "Track"}, {"11", "Other"}},
			CodecSel:    []SelectOption{{"6", "H.265(HEVC)"}, {"1", "H.264(AVC)"}, {"2", "VC-1"}, {"4", "MPEG-2"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"10", "8K"}, {"5", "4K"}, {"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}, {"11", "None"}},
			AudioCodec:  []SelectOption{{"19", "DTS-HD MA"}, {"20", "TrueHD"}, {"21", "LPCM"}, {"3", "DTS"}, {"18", "DD/AC3"}, {"6", "AAC"}, {"1", "FLAC"}, {"2", "APE"}, {"22", "WAV"}, {"23", "MP3"}, {"24", "M4A"}, {"7", "Other"}},
			TeamSel:     []SelectOption{{"19", "PTHome"}, {"21", "PTH"}, {"20", "PTHweb"}, {"22", "PTHtv"}, {"23", "PTHAudio"}, {"24", "PTHeBook"}, {"25", "PTHmusic"}, {"5", "Other"}},
		},
	},
	{
		Domain: "sbpt.link", Name: "SBPT", BaseURL: "https://sbpt.link",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影(Movie)"}, {"408", "音乐(Music)"}, {"409", "其他"}, {"407", "体育(Sport)"}, {"406", "音乐短片(MV)Music Videos"}, {"403", "综艺(TV Show)"}, {"402", "电视剧(TV Series)"}, {"405", "动画(Animation)"}, {"404", "纪录片(Documentary)"}},
			MediumSel:   []SelectOption{{"9", "Track"}, {"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "HD DVD"}, {"1", "Blu-ray"}, {"10", "WEB-DL"}, {"11", "WEBRip"}, {"12", "BDRip"}},
			CodecSel:    []SelectOption{{"1", "H.264 / AVC"}, {"2", "VC-1"}, {"3", "MPEG-4 Part 2 (如 Xvid/DivX)"}, {"4", "MPEG-2"}, {"5", "Other"}, {"6", "H.265 / HEVC"}, {"7", "VP8"}, {"8", "VP9"}, {"9", "ProRes"}, {"10", "H.266 / VVC"}},
			StandardSel: []SelectOption{{"5", "2160p"}, {"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}},
			TeamSel:     []SelectOption{{"1", "HDS"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}},
			Tags:        []SelectOption{{"16", "原盘ISO"}, {"1", "中英双语字幕"}, {"14", "特效字幕"}, {"2", "原盘BDMV"}, {"4", "DIY"}, {"5", "国语"}, {"6", "粤语"}, {"7", "3D"}, {"15", "合集"}, {"13", "CC"}, {"12", "WEB-DL"}, {"10", "Remux"}, {"9", "UHD Blu-ray"}, {"8", "mUHD"}},
		},
	},
	{
		Domain: "sewerpt.com", Name: "下水道", BaseURL: "https://sewerpt.com",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"408", "音乐 / Music"}, {"409", "其他 / Misc"}, {"403", "综艺 / TV Shows"}, {"402", "电视剧 / TV Series"}, {"405", "动漫 / Animations"}, {"404", "纪录片 / Documentaries"}, {"401", "电影 / Movies"}},
			MediumSel:   []SelectOption{{"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "HD DVD"}, {"1", "Blu-ray"}, {"10", "WEB-DL"}},
			CodecSel:    []SelectOption{{"6", "HEVC"}, {"1", "AVC"}, {"2", "VC-1"}, {"3", "Xvid"}, {"4", "MPEG-2"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"2", "480p"}, {"3", "720p"}, {"1", "1080p/1080i"}, {"4", "2K/1440p"}, {"5", "4K/2160p"}, {"6", "8K/4320p"}},
			AudioCodec:  []SelectOption{{"1", "FLAC"}, {"2", "APE"}, {"3", "DTS"}, {"4", "MP3"}, {"5", "OGG"}, {"6", "AAC"}, {"7", "Other"}, {"8", "AC3"}, {"9", "ALAC"}, {"10", "WAV"}, {"11", "E-AC3"}, {"12", "TrueHD Atmos"}, {"13", "TrueHD"}, {"14", "DTS-HD MA"}, {"15", "DTS:X"}, {"16", "LPCM"}, {"17", "AV3A"}, {"18", "OPUS"}},
			TeamSel:     []SelectOption{{"1", "SewageWeb"}, {"5", "Other"}},
			Tags:        []SelectOption{{"3", "官方"}, {"11", "冷门/低分"}, {"1", "禁转"}, {"5", "国语"}, {"15", "粤语"}, {"6", "中字"}, {"10", "原盘"}, {"4", "DIY"}, {"7", "HDR"}, {"2", "首发"}, {"13", "短剧"}, {"9", "原创"}, {"12", "完结"}, {"16", "高码率"}, {"14", "杜比"}, {"8", "分集"}},
		},
	},
	{
		Domain: "springsunday.net", Name: "不可说", BaseURL: "https://springsunday.net",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			MediumSel:   []SelectOption{{"1", "Blu-ray"}, {"4", "Remux"}, {"2", "MiniBD"}, {"6", "BDRip"}, {"7", "WEB-DL"}, {"8", "WEBRip"}, {"5", "HDTV"}, {"9", "TVRip"}, {"3", "DVD"}, {"10", "DVDRip"}, {"11", "CD"}, {"99", "Other"}},
			CodecSel:    []SelectOption{{"1", "H.265/HEVC"}, {"2", "H.264/AVC"}, {"3", "VC-1"}, {"4", "MPEG-2"}, {"5", "AV1"}, {"99", "Other"}},
			StandardSel: []SelectOption{{"1", "2160p"}, {"2", "1080p"}, {"3", "1080i"}, {"4", "720p"}, {"5", "SD"}, {"99", "Other"}},
			AudioCodec:  []SelectOption{{"1", "DTS-HD"}, {"2", "TrueHD"}, {"6", "LPCM"}, {"3", "DTS"}, {"11", "E-AC-3"}, {"4", "AC-3"}, {"5", "AAC"}, {"7", "FLAC"}, {"8", "APE"}, {"9", "WAV"}, {"10", "MP3"}, {"12", "OPUS"}, {"13", "AV3A"}, {"99", "Other"}},
			SourceSel:   []SelectOption{{"1", "Mainland(大陆)"}, {"2", "Hongkong(香港)"}, {"3", "Taiwan(台湾)"}, {"4", "West(欧美)"}, {"5", "Japan(日本)"}, {"6", "Korea(韩国)"}, {"7", "India(印度)"}, {"8", "Russia(俄国)"}, {"9", "Thailand(泰国)"}, {"99", "Other(其他地区)"}},
		},
	},
	{
		Domain: "tjupt.org", Name: "不可羊", BaseURL: "https://tjupt.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category: []SelectOption{{"401", "电影"}, {"402", "剧集"}, {"403", "综艺"}, {"404", "资料"}, {"405", "动漫"}, {"406", "音乐"}, {"407", "体育"}, {"408", "软件"}, {"409", "游戏"}, {"411", "纪录片"}, {"412", "移动视频"}, {"410", "其他"}},
		},
	},
	{
		Domain: "u2.dmhy.org", Name: "幼儿园", BaseURL: "https://u2.dmhy.org",
		Framework: "nexusphp", IsSource: true, IsTarget: false,
		Form: SiteFormConfig{},
	},
	{
		Domain: "ubits.club", Name: "优堡", BaseURL: "https://ubits.club",
		Framework: "nexusphp", IsSource: true, IsTarget: false,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影(Movie)"}, {"404", "纪录片(Documentaries)"}, {"405", "动漫(Animations)"}, {"402", "电视剧(TV Series)"}, {"403", "综艺（TV Shows）"}, {"409", "演唱会(Misic Videos)"}, {"407", "体育节目(Sports)"}, {"406", "音乐CD(Music CD Tracker)"}, {"408", "Other"}},
			MediumSel:   []SelectOption{{"10", "4K UHD原盘(UltraHD Blu-ray)"}, {"1", "蓝光原盘(Blu-ray)"}, {"4", "流媒体(WEB-DL)"}, {"3", "REMUX"}, {"7", "压制(Encode)"}, {"2", "HD DVD"}, {"5", "HDTV"}, {"6", "DVDR"}, {"8", "Lossless Music"}, {"9", "Track"}},
			CodecSel:    []SelectOption{{"7", "H265(HEVC/x265)"}, {"1", "H264(AVC/x264)"}, {"11", "AV1"}, {"2", "VC-1"}, {"4", "MPEG-2"}, {"10", "AVS"}, {"3", "Xvid"}, {"9", "MPEG-4"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"6", "4320p"}, {"5", "2160p"}, {"7", "1440p"}, {"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}},
			AudioCodec:  []SelectOption{{"8", "Dolby Atmos"}, {"9", "DTS:X"}, {"10", "TrueHD"}, {"11", "DTS-HD MA/HR"}, {"13", "LPCM"}, {"12", "DD+(Dolby Digital Plus)"}, {"14", "DD(AC3)"}, {"3", "DTS"}, {"6", "AAC"}, {"1", "FLAC"}, {"2", "APE"}, {"5", "OGG"}, {"4", "MP3"}, {"7", "Other"}},
			TeamSel:     []SelectOption{{"1", "UBits"}, {"6", "UBWEB"}, {"7", "UBTV"}, {"5", "Other"}},
			SourceSel:   []SelectOption{{"1", "中国大陆(China Mainland)"}, {"2", "中国香港(China HK)"}, {"3", "中国台湾(China Taiwan)"}, {"4", "欧美(Euro/American)"}, {"5", "日本(Japanese)"}, {"6", "韩国(Korea)"}, {"7", "泰国(Thailand)"}, {"8", "印度(India)"}, {"9", "俄罗斯(Russia)"}, {"11", "其它(Other)"}},
			Tags:        []SelectOption{{"1", "禁转"}, {"3", "官方"}, {"17", "原生原盘"}, {"4", "DIY"}, {"5", "国语"}, {"11", "粤语"}, {"6", "中字"}, {"21", "高分国剧"}, {"12", "特效字幕"}, {"8", "杜比视界"}, {"10", "HDR10+"}, {"18", "菁彩HDR"}, {"9", "HDR10"}, {"23", "连载"}, {"20", "合集"}, {"19", "HLG"}, {"22", "自购"}, {"14", "菜单修改"}},
		},
	},
	{
		Domain: "www.dragonhd.xyz", Name: "龙之家", BaseURL: "https://www.dragonhd.xyz",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影"}, {"411", "剧集"}, {"412", "游戏"}, {"404", "纪录片"}, {"405", "动漫"}, {"403", "综艺"}, {"406", "MV"}, {"407", "体育"}, {"408", "音乐"}, {"410", "AV"}, {"409", "其他"}},
			MediumSel:   []SelectOption{{"9", "UHD"}, {"1", "Blu-ray"}, {"10", "Remux"}, {"11", "Encode"}, {"6", "WEB-DL"}, {"3", "DVD"}, {"4", "HDTV"}, {"7", "CD/SACD"}, {"8", "Other"}},
			CodecSel:    []SelectOption{{"1", "H.264(AVC)"}, {"2", "H.265(HEVC)"}, {"3", "VC-1"}, {"4", "MPEG-2"}, {"23", "VP9"}, {"21", "Other"}},
			StandardSel: []SelectOption{{"1", "8K/4320p"}, {"3", "4K/2160p"}, {"4", "2K/1440p"}, {"5", "1080p/1080i"}, {"6", "720p"}, {"7", "SD"}},
			AudioCodec:  []SelectOption{{"15", "TrueHD Atmos"}, {"16", "TrueHD"}, {"3", "DTS-X"}, {"11", "DTS-HD MA"}, {"17", "DTS-HD HR"}, {"18", "DTS"}, {"1", "FLAC"}, {"6", "AAC"}, {"7", "DD/DD+/AC3"}, {"10", "LPCM"}, {"4", "MP3"}, {"5", "OGG"}, {"2", "APE"}, {"12", "WAV"}, {"14", "Other"}},
			TeamSel:     []SelectOption{{"9", "DragonHD"}, {"1", "HDS"}, {"2", "CHD"}, {"4", "WiKi"}, {"5", "Beitai"}, {"6", "FRDS"}, {"7", "CMCT"}, {"10", "LeagueHD"}, {"8", "Other"}},
		},
	},
	{
		Domain: "www.hitpt.com", Name: "百川", BaseURL: "https://www.hitpt.com",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "高清电影"}, {"402", "高清剧集"}, {"403", "抢鲜或标清"}, {"405", "动漫"}, {"407", "体育"}, {"413", "纪录片"}, {"416", "综艺"}, {"415", "Music Video"}},
			CodecSel:    []SelectOption{{"10", "H.265"}, {"1", "H.264"}, {"14", "X265"}, {"13", "X264"}, {"2", "VC-1"}, {"4", "MPEG-2"}, {"12", "MPEG-4"}, {"3", "Xvid"}, {"11", "VP9"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"7", "8K"}, {"5", "4K"}, {"6", "1440p"}, {"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}, {"8", "Other"}},
			AudioCodec:  []SelectOption{{"19", "DTS-HDMA:X 7.1"}, {"18", "DTS-HDMA"}, {"17", "DTS-HDMR"}, {"16", "DTS-HD"}, {"15", "DTS-X"}, {"14", "LPCM"}, {"8", "AC3"}, {"13", "Atmos"}, {"6", "AAC"}, {"12", "TrueHD"}, {"3", "DTS"}, {"1", "FLAC"}, {"2", "APE"}, {"4", "MP3"}, {"11", "WAV"}, {"5", "OGG"}, {"7", "Other"}},
			TeamSel:     []SelectOption{{"22", "PTer"}, {"18", "HDDolby"}, {"21", "HSPT"}, {"19", "OurBits"}, {"16", "MTeam"}, {"20", "FRDS"}, {"14", "beAst"}, {"6", "HDWinG"}, {"8", "CHDBits"}, {"5", "CMCT"}, {"13", "WiKi"}, {"17", "百川自制"}, {"3", "HIT内部资料"}, {"4", "其他"}},
			SourceSel:   []SelectOption{{"10", "保种资源"}, {"11", "UHD"}, {"1", "Blu-ray"}, {"12", "Remux"}, {"4", "HDTV"}, {"2", "BDrip"}, {"9", "Web"}, {"3", "DVD"}, {"7", "CD"}, {"5", "TV"}, {"8", "Other"}},
			Tags:        []SelectOption{{"9", "杜比"}, {"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"8", "粤语"}, {"5", "国语"}, {"6", "中字"}, {"7", "HDR"}},
		},
	},
	{
		Domain: "www.hxpt.org", Name: "好学", BaseURL: "https://www.hxpt.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"401", "学前教育"}, {"402", "小学部"}, {"403", "初中部"}, {"404", "高职部"}, {"405", "高中部"}, {"406", "教育"}, {"410", "纪录片"}},
			MediumSel:     []SelectOption{{"1", "视频"}, {"2", "音频"}, {"3", "书籍"}, {"4", "文档"}, {"5", "笔记"}, {"6", "课件"}, {"7", "软件"}, {"8", "会议"}, {"9", "图片"}, {"10", "其他"}},
			CodecSel:      []SelectOption{{"1", "RMVB"}, {"2", "MP4"}, {"3", "JPG"}, {"4", "PDF"}, {"5", "TXT"}, {"6", "DOC"}, {"7", "XLS"}, {"8", "PPT"}, {"9", "WMA"}, {"10", "MP3"}, {"11", "RAR"}, {"12", "EXE"}, {"13", "MKV"}, {"14", "EPUB"}, {"15", "MA4"}, {"16", "AZW3"}, {"17", "DJVU"}, {"18", "CAJ"}, {"19", "MOBI"}, {"20", "TIFF"}, {"21", "PSD"}, {"22", "APE"}, {"23", "FLAC"}, {"24", "ZIP"}, {"25", "UVZ"}, {"26", "其他"}},
			StandardSel:   []SelectOption{{"2", "720p/720i"}, {"3", "1080p/1080i"}, {"4", "4K/2160p/2160i"}, {"5", "8K/4320p/4320i"}, {"6", "Other"}},
			AudioCodec:    []SelectOption{{"1", "幼儿园"}, {"2", "一年级"}, {"3", "二年级"}, {"4", "三年级"}, {"5", "四年级"}, {"6", "五年级"}, {"7", "六年级"}, {"8", "七年级"}, {"9", "八年级"}, {"10", "九年级"}, {"11", "高一"}, {"12", "高二"}, {"13", "高三"}, {"14", "其他"}},
			TeamSel:       []SelectOption{{"10", "Other"}, {"11", "HX"}, {"9", "HXPT"}, {"8", "HXWEB"}},
			ProcessingSel: []SelectOption{{"1", "人教版"}, {"26", "新人教"}, {"19", "人民版"}, {"2", "部编版"}, {"28", "统编版"}, {"3", "苏教版"}, {"4", "鄂教版"}, {"5", "鲁教版"}, {"6", "北师大版"}, {"7", "沪教版"}, {"8", "冀教版"}, {"9", "浙教版"}, {"10", "河大版"}, {"11", "湘教版"}, {"12", "京教版"}, {"13", "外研版"}, {"14", "外研版"}, {"15", "牛津版"}, {"25", "中图版"}, {"16", "粤沪版"}, {"17", "北师版"}, {"18", "陕教版"}, {"20", "北师版"}, {"21", "川教版"}, {"22", "上海教育版"}, {"23", "济南版"}, {"24", "北京版"}, {"27", "其他"}},
			SourceSel:     []SelectOption{{"18", "2026"}, {"17", "2025"}, {"16", "2024"}, {"15", "2023"}, {"14", "2022"}, {"13", "2021"}, {"12", "2020"}, {"11", "2019"}, {"10", "2018"}, {"9", "2017"}, {"8", "2016"}, {"7", "2015"}, {"6", "2014"}, {"5", "2013"}, {"4", "2012"}, {"3", "2011"}, {"2", "2010年前"}, {"1", "Other"}},
			Tags:          []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"5", "国语"}, {"6", "中字"}, {"7", "HDR"}, {"8", "语文"}, {"9", "数学"}, {"10", "外语"}, {"11", "历史"}, {"12", "地理"}, {"22", "物理"}, {"13", "化学"}, {"14", "生物"}, {"15", "道德与法治(道法)"}, {"16", "音乐"}, {"17", "美术"}, {"18", "体育"}, {"19", "自然科学"}, {"20", "信息技术"}, {"21", "兴趣爱好"}, {"23", "分集"}, {"24", "完结"}},
		},
	},
	{
		Domain: "www.okpt.net", Name: "OKPT", BaseURL: "https://www.okpt.net",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"402", "电视剧"}, {"401", "电影"}, {"440", "短剧"}, {"405", "动漫/动画"}, {"404", "纪录片"}, {"403", "综艺/真人秀"}, {"434", "电子书"}, {"432", "有声书"}, {"436", "漫画书"}, {"413", "游戏"}, {"407", "体育"}, {"431", "软件"}, {"409", "其它"}},
			MediumSel:     []SelectOption{{"16", "其他（Other）"}, {"11", "UHD Blu-ray"}, {"1", "Blu-ray"}, {"3", "Remux"}, {"7", "Encode"}, {"10", "WEB-DL"}, {"5", "HDTV"}, {"2", "DVD"}, {"15", "SACD"}, {"8", "CD"}, {"17", "Vinyl"}, {"18", "HDCD"}, {"19", "HI-RES"}, {"20", "Web"}},
			CodecSel:      []SelectOption{{"21", "EPUB/AZW3/MOBI"}, {"19", "AZW3/MOBI"}, {"16", "PDF"}, {"17", "EPUB"}, {"20", "ZIP"}, {"15", "TXT"}, {"11", "HEVC/H.265/x265"}, {"2", "AVC/H.264/x264"}, {"10", "H.266/VVC"}, {"7", "AV1"}, {"12", "VP9"}, {"14", "Other"}},
			StandardSel:   []SelectOption{{"1", "8K"}, {"2", "4K/2160p"}, {"3", "1080p/1080i"}, {"4", "720p"}, {"10", "Other"}},
			AudioCodec:    []SelectOption{{"22", "DTS:X"}, {"14", "MPEG"}, {"4", "MP3"}, {"5", "APE"}, {"20", "WAV"}, {"1", "FLAC 分轨"}, {"3", "DTS"}, {"16", "LPCM"}, {"19", "TrueHD"}, {"7", "DTS-HD"}, {"6", "AAC"}, {"15", "DD/DD+"}, {"23", "镜像(Mirror) 整轨"}, {"21", "Other"}, {"24", "WAV 整轨"}, {"25", "DSF 分轨"}},
			TeamSel:       []SelectOption{{"29", "ZmPT"}, {"28", "Ying"}, {"32", "U2"}, {"27", "UBits"}, {"26", "TTG"}, {"24", "Rousi"}, {"22", "PTHome"}, {"23", "Red Leaves"}, {"21", "PterClub"}, {"5", "Panda"}, {"2", "OurBits"}, {"3", "OKPT"}, {"12", "M-Team"}, {"11", "LemonHD"}, {"1", "HD4FANS"}, {"10", "HDSky"}, {"6", "HDHome"}, {"9", "HDDollby"}, {"8", "HDChina"}, {"4", "HHClub"}, {"20", "FRDS"}, {"19", "DaJiao"}, {"16", "CMCT"}, {"17", "CHDBits"}, {"18", "BTSchool"}, {"31", "BeiTai"}, {"13", "Audiences"}, {"14", "Other"}},
			ProcessingSel: []SelectOption{{"8", "中国大陆（CN）"}, {"7", "港澳台（HK/MAC/TW）"}, {"6", "欧美（EU/US）"}, {"5", "日本（JP）"}, {"4", "韩国（KR）"}, {"17", "印度（India）"}, {"18", "东南亚（SEA）"}, {"3", "其他（Other）"}},
			Tags:          []SelectOption{{"1", "禁转"}, {"56", "驻站"}, {"11", "自购"}, {"51", "完結"}, {"50", "分集"}, {"5", "国语"}, {"45", "粤语"}, {"57", "英语"}, {"28", "日语"}, {"25", "韩语"}, {"6", "中字"}, {"58", "英字"}, {"4", "DIY"}, {"12", "特效"}, {"7", "HDR"}, {"8", "DoVi"}, {"53", "Atmos"}, {"31", "资料教程"}},
		},
	},
	{
		Domain: "www.oshen.win", Name: "奥申", BaseURL: "https://www.oshen.win",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"410", "游戏/Game"}, {"408", "高清音乐"}, {"409", "音乐"}, {"407", "体育"}, {"406", "MV(音乐视频)"}, {"403", "综艺节目"}, {"402", "连续剧"}, {"405", "动漫"}, {"404", "纪录片"}, {"401", "电影"}},
			MediumSel:   []SelectOption{{"9", "Track"}, {"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "HD DVD"}, {"1", "Blu-ray"}},
			CodecSel:    []SelectOption{{"1", "H.264"}, {"2", "VC-1"}, {"3", "Xvid"}, {"4", "MPEG-2"}, {"5", "Other"}, {"10", "H.265/HEVC"}},
			StandardSel: []SelectOption{{"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}, {"5", "4K/UHD"}},
			TeamSel:     []SelectOption{{"7", "SCSG"}, {"6", "CMCT"}, {"9", "PTHOME"}, {"8", "HDTIME"}, {"10", "OshenPT"}, {"1", "HDS"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}, {"11", "52pt"}},
			Tags:        []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"3", "官方"}, {"4", "DIY"}, {"5", "国语"}, {"6", "中字"}, {"7", "HDR"}},
		},
	},
	{
		Domain: "www.tangpt.top", Name: "趟平", BaseURL: "https://www.tangpt.top",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影"}, {"402", "电视剧"}, {"403", "综艺"}, {"404", "纪录片"}, {"405", "动漫"}, {"406", "MV"}, {"407", "体育"}, {"414", "其他"}, {"409", "音乐"}},
			MediumSel:   []SelectOption{{"9", "Other"}, {"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "HD DVD"}, {"1", "Blu-ray"}, {"10", "UHD Blu-ray"}, {"11", "WEB-DL"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC"}, {"6", "H.265/HEVC"}, {"7", "AV1"}, {"10", "VP8/9"}, {"2", "VC-1"}, {"3", "Xvid"}, {"4", "MPEG-2"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"1", "1080i/1080P"}, {"4", "2K/1440i/1440P"}, {"5", "4K/2160i/2160P"}, {"6", "8K/4320i/4320P"}, {"7", "Other"}, {"2", "720i/720P"}, {"3", "480i/480P"}},
			AudioCodec:  []SelectOption{{"9", "TrueHD Atmos"}, {"16", "DTS-HD MA"}, {"15", "DTS:X"}, {"14", "DTS"}, {"12", "TrueHD"}, {"19", "LPCM"}, {"13", "WAV"}, {"1", "FLAC"}, {"11", "DD/AC3"}, {"10", "DDP/E-AC3"}, {"6", "AAC"}, {"5", "OGG"}, {"4", "MP3"}, {"17", "M4A"}, {"18", "OPUS"}, {"2", "APE"}, {"8", "AV3V"}, {"7", "Other"}},
			TeamSel:     []SelectOption{{"39", "TPWEB"}, {"43", "LUCKMUSIC"}, {"42", "LUCKWEB"}, {"41", "LUCKDIY"}, {"40", "AilMWeb"}, {"37", "QHstudIo"}, {"2", "CHD"}, {"1", "HDS"}, {"26", "U2"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}, {"6", "StarfallWeb"}, {"38", "CSWEB"}, {"7", "AGSVWEB"}, {"36", "UBWEB"}, {"35", "ZmWeb"}, {"34", "HHWEB"}, {"31", "ADWeb"}, {"8", "FRDS"}, {"24", "OpenCD"}, {"23", "beAst"}, {"21", "HDArea"}, {"19", "FFansDIY"}, {"17", "CMCT"}, {"16", "OurTV"}, {"15", "BtsHD"}, {"14", "HDHome"}, {"12", "PTerWEB"}, {"11", "HDSWEB"}, {"10", "MWeb"}},
			Tags:        []SelectOption{{"45", "首发"}, {"36", "自由"}, {"35", "玄学"}, {"28", "自制"}, {"1", "演唱会"}, {"40", "cosplay"}, {"39", "禁转"}, {"38", "写真"}, {"41", "NSFW"}, {"23", "讲座"}, {"4", "DIY"}, {"10", "粤语"}, {"5", "国语"}, {"6", "中字"}, {"19", "完结"}, {"7", "HDR"}, {"42", "秀人网"}, {"34", "娱乐"}, {"32", "恐怖"}, {"31", "书籍"}, {"30", "游戏"}, {"29", "软件"}, {"27", "情色"}, {"24", "短视频"}, {"22", "纯享"}, {"21", "分集"}, {"20", "大包"}, {"15", "动画"}, {"9", "DV"}, {"8", "特效"}},
		},
	},
	{
		Domain: "zmpt.cc", Name: "织梦", BaseURL: "https://zmpt.cc",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"409", "其他 / Misc"}, {"422", "纪录片 / documentary"}, {"417", "动漫 / Anime"}, {"427", "短剧 / Short Play"}, {"401", "电影 / Movies"}, {"402", "电视剧 / TV Series"}, {"403", "综艺 / TV Shows"}, {"423", "音乐 / Music"}, {"424", "有声书 / Audiobook"}, {"425", "软件 / Software"}, {"426", "游戏 / Game"}},
			MediumSel:   []SelectOption{{"12", "其他资料"}, {"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "HD DVD"}, {"1", "Blu-ray"}, {"10", "WEB-DL"}},
			StandardSel: []SelectOption{{"7", "480p"}, {"8", "720p"}, {"1", "1080p/1080i"}, {"6", "2K/1440p"}, {"5", "4K/2160p"}, {"9", "8K/4320p"}},
			AudioCodec:  []SelectOption{{"7", "Other"}, {"1", "FLAC"}, {"2", "APE"}, {"3", "DTS"}, {"5", "OGG"}, {"8", "AC3"}, {"6", "AAC"}, {"4", "MP3"}, {"9", "ALAC"}, {"10", "WAV"}},
			TeamSel:     []SelectOption{{"5", "Other"}, {"7", "ZmWeb"}, {"6", "ZmPT"}, {"8", "ZmMusic"}, {"11", "ZmAudio"}, {"9", "DYZ-Movie"}, {"10", "GodDramas"}, {"12", "RL/RL4B"}},
			Tags:        []SelectOption{{"17", "驻站"}, {"1", "禁转"}, {"5", "国语"}, {"15", "粤语"}, {"6", "中字"}, {"4", "DIY"}, {"14", "杜比"}, {"7", "HDR"}, {"16", "无损"}, {"2", "首发"}, {"13", "短剧"}, {"11", "汉化"}, {"9", "原创"}, {"12", "完结"}, {"8", "分集"}, {"18", "纯享版"}},
		},
	},
	{
		Domain: "zrpt.cc", Name: "自然", BaseURL: "https://zrpt.cc",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"412", "Nature"}, {"413", "NatureWeb"}, {"401", "军事纪实"}, {"402", "生物自然"}, {"403", "餐饮美食"}, {"404", "人文记录"}, {"405", "宇宙星球"}, {"406", "音乐视频"}, {"407", "体育运动"}, {"409", "其它"}, {"415", "旅游风景"}, {"416", "纪录片"}},
			MediumSel:   []SelectOption{{"9", "Remux"}, {"8", "Blu-ray"}, {"6", "Other"}, {"5", "Track"}, {"4", "HDTV"}, {"7", "WEB-DL"}, {"3", "CD"}, {"2", "DVD"}, {"1", "UHD Blu-ray"}, {"10", "Encode"}, {"11", "MiniBD"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC"}, {"2", "VC-1"}, {"3", "MPEG-4"}, {"4", "MPEG-2"}, {"5", "Other"}, {"6", "H.265/HEVC"}, {"7", "AV1"}, {"8", "VP9"}},
			StandardSel: []SelectOption{{"1", "8K"}, {"2", "4K"}, {"3", "1080p"}, {"4", "1080i"}, {"5", "720p"}, {"6", "SD"}, {"7", "Other"}},
			AudioCodec:  []SelectOption{{"1", "DTS:X"}, {"2", "DTS"}, {"3", "DTS-HD MA"}, {"4", "DTS-HD HRA"}, {"5", "TrueHD Atmos"}, {"6", "TrueHD"}, {"7", "LPCM"}, {"8", "DD/AC3"}, {"9", "DDP/E-AC3"}, {"10", "FLAC"}, {"11", "AAC"}, {"12", "APE"}, {"13", "WAV"}, {"14", "MP3"}, {"15", "M4A"}, {"16", "OPUS"}, {"17", "Other"}, {"18", "AV3A"}, {"19", "USAC"}},
			TeamSel:     []SelectOption{{"1", "HDS"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}, {"6", "ZRPT"}},
			Tags:        []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"5", "国语"}, {"6", "中字"}, {"7", "HDR"}, {"41", "HDR10+"}, {"40", "Remux"}, {"22", "原创"}, {"21", "特效"}, {"20", "应求"}, {"19", "杜比"}, {"18", "驻站"}, {"17", "连载"}, {"12", "原盘"}, {"11", "分集"}, {"10", "完结"}, {"9", "粤语"}, {"8", "英语"}},
		},
	},
	{
		Domain: "hdfans.org", Name: "红豆饭", BaseURL: "https://hdfans.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"401", "Movies/电影"}, {"402", "TV Series/电视剧"}, {"403", "Documentaries/纪录片"}, {"406", "Music/音乐"}, {"416", "TV Shows/综艺"}, {"417", "Animations/动漫"}, {"407", "Music Videos/音乐视频"}, {"408", "Concert/演唱会"}, {"404", "Education/教育"}, {"405", "Audio Books/有声读物"}, {"409", "Drama/戏剧"}, {"418", "Sports/体育"}, {"419", "Software/软件"}, {"421", "Games/游戏"}, {"423", "E-Books/电子书"}, {"410", "Others/其他"}},
			MediumSel:     []SelectOption{{"17", "UHD原盘"}, {"18", "UHD DIY"}, {"19", "UHD Remux"}, {"20", "UHD压制"}, {"21", "BD原盘"}, {"22", "BD DIY"}, {"23", "BD Remux"}, {"24", "1080P/i压制"}, {"25", "720P压制"}, {"26", "MiniSD"}, {"5", "WEB-DL"}, {"6", "HDTV"}, {"7", "DVD"}, {"9", "CD"}, {"16", "SACD"}, {"30", "CD+VCD"}, {"27", "CD+DVD"}, {"28", "黑胶"}, {"10", "Other"}},
			CodecSel:      []SelectOption{{"1", "H.264/AVC"}, {"2", "x264"}, {"3", "H.265/HEVC"}, {"4", "x265"}, {"5", "VC-1"}, {"10", "MPEG-2"}, {"11", "MPEG-4"}, {"12", "Xvid"}, {"14", "AV1"}, {"13", "Other"}},
			StandardSel:   []SelectOption{{"1", "8K/4320P"}, {"2", "4K/UHD/2160P"}, {"3", "1080P"}, {"4", "1080i"}, {"5", "720P"}, {"6", "SD"}, {"7", "Other"}},
			AudioCodec:    []SelectOption{{"1", "TrueHD Atmos"}, {"3", "DTS:X"}, {"6", "True-HD"}, {"4", "DTS-HDMA"}, {"5", "DTS-HD HR"}, {"2", "DTS"}, {"7", "LPCM"}, {"14", "WAV"}, {"13", "APE"}, {"12", "FLAC"}, {"20", "DSD"}, {"23", "DDP Atmos"}, {"21", "DDP/DD+/EAC3"}, {"19", "Dolby Digital/DD"}, {"22", "MPEG"}, {"15", "OGG"}, {"11", "AAC"}, {"10", "AC3"}, {"16", "TTA"}, {"17", "MP3"}, {"25", "ALAC"}, {"24", "m4a"}, {"18", "Other"}},
			TeamSel:       []SelectOption{{"9", "HDFans"}, {"1", "CHDBits"}, {"2", "HDC"}, {"19", "TTG"}, {"3", "WiKi"}, {"4", "beAst"}, {"5", "CMCT"}, {"6", "FRDS"}, {"7", "HDS"}, {"17", "OurBits"}, {"20", "PTer"}, {"29", "Audies"}, {"41", "Ubits"}, {"34", "HHClub"}, {"18", "HDHome"}, {"16", "PTHome"}, {"40", "QHstudIo"}, {"28", "Hares"}, {"8", "TLF"}, {"37", "PTP"}, {"32", "BTN/NTb"}, {"30", "EPSiLON"}, {"31", "FraMeSToR"}, {"33", "OpenCD"}, {"35", "DIC"}, {"36", "Red"}, {"39", "GGN"}, {"26", "LemonHD"}, {"27", "Others"}},
			ProcessingSel: []SelectOption{{"1", "CN/中国大陆"}, {"4", "HK/香港"}, {"5", "TW/台湾"}, {"2", "US/美国"}, {"8", "EU/欧洲"}, {"3", "UK/英国"}, {"6", "JP/日本"}, {"7", "KR/韩国"}, {"10", "IN/印度"}, {"11", "SG/新加坡"}, {"12", "MY/马来西亚"}, {"9", "Other/其他"}},
			Tags:          []SelectOption{{"2", "微光星辰"}, {"14", "官字组"}, {"16", "甄选"}, {"24", "驻站"}, {"15", "首发"}, {"13", "原创"}, {"3", "禁转"}, {"8", "限转"}, {"29", "源站转发"}, {"4", "DIY"}, {"21", "原生"}, {"5", "国语"}, {"10", "粤语"}, {"6", "中字"}, {"17", "中英双语"}, {"18", "特效"}, {"7", "HDR"}, {"9", "Dolby Vision"}, {"12", "Atmos"}, {"19", "4K"}, {"20", "8K"}, {"22", "CC收藏"}, {"30", "Hi-Res"}, {"23", "完结"}, {"28", "刮削"}, {"27", "AI修复"}, {"26", "保种"}},
		},
	},
	{
		Domain: "pt.ying.us.kg", Name: "樱花", BaseURL: "https://pt.ying.us.kg",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影"}, {"402", "电视剧"}, {"403", "综艺"}, {"404", "纪录片"}, {"405", "动漫"}, {"406", "MV"}, {"407", "体育"}, {"408", "音频"}, {"409", "其他"}, {"410", "短剧"}},
			MediumSel:   []SelectOption{{"1", "UHD Blu-ray"}, {"2", "Blu-ray"}, {"3", "Remux"}, {"4", "WEB-DL"}, {"5", "HDTV"}, {"6", "DVD"}, {"7", "Encode"}, {"8", "CD"}, {"9", "Track"}, {"10", "Other"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC"}, {"2", "VC-1"}, {"4", "MPEG-2"}, {"5", "Other"}, {"6", "AV1"}, {"7", "H.265/HEVC"}},
			StandardSel: []SelectOption{{"1", "4K/2160p"}, {"2", "1080p/1080i"}, {"3", "720p/720i"}, {"4", "480p/480i"}, {"5", "8K/4320p"}, {"6", "Other"}},
			TeamSel:     []SelectOption{{"1", "HDS"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}, {"6", "YHWeb"}},
			Tags:        []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"5", "国语"}, {"6", "中字"}, {"7", "HDR"}, {"8", "粤语"}, {"9", "杜比"}, {"10", "合并"}, {"11", "零魔"}, {"12", "超分"}, {"13", "大包"}, {"14", "应求"}, {"15", "完结"}, {"16", "分集"}, {"17", "英字"}, {"18", "韩剧"}},
		},
	},
	{
		Domain: "open.cd", Name: "皇后", BaseURL: "https://open.cd",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"408", "音乐(Music)"}, {"402", "演唱会(Vocal Concert)"}, {"405", "戏剧(Drama)"}, {"409", "其他(Other)"}},
			SourceSel:   []SelectOption{{"2", "流行(Pop)"}, {"3", "古典(Classical)"}, {"11", "器乐(Instrumental)"}, {"4", "原声(OST)"}, {"5", "摇滚(Rock)"}, {"8", "爵士(Jazz)"}, {"12", "新世纪(NewAge)"}, {"13", "舞曲(Dance)"}, {"14", "电子(Electronic)"}, {"15", "民谣(Folk)"}, {"16", "独立(Indie)"}, {"17", "嘻哈(Hip Hop)"}, {"18", "音乐剧(Musical)"}, {"19", "乡村(Country)"}, {"20", "另类(Alternative)"}, {"21", "世界音乐(World)"}, {"9", "其他(Others)"}},
			StandardSel: []SelectOption{{"1", "镜像(Mirror)"}, {"2", "WAV"}, {"4", "FLAC"}, {"15", "DTS"}, {"17", "DFF"}, {"18", "DSF"}, {"19", "DST"}, {"10", "其它(Other)"}},
			MediumSel:   []SelectOption{{"1", "CD"}, {"2", "24KCD"}, {"3", "DSD"}, {"4", "LPCD"}, {"5", "HDCD"}, {"6", "SACD"}, {"7", "SRCD"}, {"8", "K2CD"}, {"9", "DTS"}, {"10", "DAT"}, {"11", "Blu-ray"}, {"12", "HD DVD"}, {"13", "HDTV"}, {"14", "DVD"}, {"16", "HQCD"}, {"17", "XRCD"}, {"18", "SHM-CD"}, {"19", "Blu-spec"}, {"20", "Vinyl"}, {"21", "Web"}, {"22", "HI-RES"}, {"15", "Other"}},
			TeamSel:     []SelectOption{{"7", "OpenCD"}, {"8", "LLM"}, {"9", "TSxD"}, {"6", "KHQ"}, {"5", "其他(Other)"}},
		},
	},
	{
		Domain: "cspt.top", Name: "财神", BaseURL: "https://cspt.top",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "Movies"}, {"402", "TV Series"}, {"403", "TV Shows"}, {"405", "Animations"}, {"407", "Sports"}, {"408", "Music"}, {"404", "Documentaries"}, {"406", "Music Videos"}, {"409", "Others"}},
			CodecSel:    []SelectOption{{"1", "H264/AVC"}, {"2", "H265/HEVC"}, {"3", "VC-1"}, {"4", "MPEG-2"}, {"5", "AV1"}, {"6", "Other"}},
			StandardSel: []SelectOption{{"1", "4K"}, {"2", "1080p/i"}, {"3", "1080p"}, {"4", "720p"}, {"5", "SD"}, {"6", "Other"}, {"7", "8K"}},
			AudioCodec:  []SelectOption{{"1", "DTS"}, {"2", "DTS-HD"}, {"3", "DTS-HD MA"}, {"4", "DTS:X"}, {"5", "TrueHD"}, {"6", "TrueHD Atmos"}, {"7", "DD/AC3"}, {"8", "DDP/E-AC3"}, {"9", "FLAC"}, {"10", "AAC"}, {"11", "Other"}, {"12", "LPCM"}, {"13", "MP3"}, {"14", "OGG"}, {"15", "APE"}, {"16", "WAV"}},
			TeamSel:     []SelectOption{{"1", "HDSky"}, {"2", "CHD"}, {"3", "HDHome"}, {"4", "TTG"}, {"5", "Other"}},
			SourceSel:   []SelectOption{{"1", "CN"}, {"2", "HK"}, {"3", "TW"}, {"4", "US"}, {"5", "JP"}, {"6", "KR"}, {"7", "UK"}, {"8", "EU"}, {"9", "IN"}, {"10", "Other"}},
			Tags:        []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"3", "DIY"}, {"4", "国语"}, {"5", "中字"}, {"6", "HDR"}, {"7", "Dolby Vision"}, {"8", "Atmos"}, {"9", "完结"}, {"10", "分集"}, {"11", "特效"}, {"12", "粤语"}, {"13", "HDR10+"}, {"14", "原生"}, {"15", "甄选"}, {"16", "驻站"}, {"17", "原创"}, {"18", "微光星辰"}, {"19", "中英双语"}, {"20", "应求"}, {"21", "短剧"}, {"22", "源站转发"}, {"23", "大包"}, {"24", "保种"}},
		},
	},
	{
		Domain: "www.hdkyl.in", Name: "麒麟", BaseURL: "https://www.hdkyl.in",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"401", "Movies/电影"}, {"402", "TV Series/电视剧"}, {"403", "TV Shows/综艺"}, {"404", "Documentaries/纪录片"}, {"405", "Animations/动漫"}, {"406", "Music Videos/音乐视频"}, {"407", "Sports/体育"}, {"408", "Audio/音频"}, {"409", "Others/其他"}, {"410", "Cartoon/少儿动画"}, {"411", "ShortDrama/短剧"}, {"412", "Game/游戏"}, {"413", "Ebook/电子书"}, {"414", "Education/教育视频"}},
			MediumSel:     []SelectOption{{"1", "Blu-ray"}, {"2", "HD DVD"}, {"3", "Remux"}, {"4", "MiniBD"}, {"5", "HDTV"}, {"6", "DVDR"}, {"7", "Encode"}, {"8", "WEB-DL"}, {"9", "UHD Blu-ray"}, {"10", "Other"}},
			CodecSel:      []SelectOption{{"1", "H.264/AVC"}, {"2", "H.265/HEVC"}, {"3", "VC-1"}, {"4", "MPEG-2"}, {"5", "AV1"}, {"6", "Xvid"}, {"7", "Other"}},
			StandardSel:   []SelectOption{{"1", "8K/4320p"}, {"2", "4K/2160p"}, {"3", "1080p/1080i"}, {"4", "720p"}, {"5", "SD"}, {"6", "Other"}, {"7", "2K/1440p"}},
			AudioCodec:    []SelectOption{{"1", "FLAC"}, {"2", "APE"}, {"3", "DTS"}, {"4", "DTS-HD"}, {"5", "DTS-HD MA"}, {"6", "DTS:X"}, {"7", "TrueHD"}, {"8", "TrueHD Atmos"}, {"9", "LPCM"}, {"10", "DD/AC3"}, {"11", "DDP/E-AC3"}, {"12", "AAC"}, {"13", "MP3"}, {"14", "OGG"}, {"15", "WAV"}, {"16", "Other"}, {"17", "ALAC"}, {"18", "AV3A"}},
			TeamSel:       []SelectOption{{"1", "HDSky"}, {"2", "CHD"}, {"3", "HDHome"}, {"4", "TTG"}, {"5", "beAst"}, {"6", "CMCT"}, {"7", "FRDS"}, {"8", "HDS"}, {"9", "Other"}},
			ProcessingSel: []SelectOption{{"1", "2018"}, {"2", "2019"}, {"3", "2020"}, {"4", "2021"}, {"5", "2022"}, {"6", "2023"}, {"7", "2024"}, {"8", "2025"}, {"9", "2026"}, {"10", "2015"}, {"11", "2016"}},
			SourceSel:     []SelectOption{{"1", "CN/中国大陆"}, {"2", "HK/香港"}, {"3", "TW/台湾"}, {"4", "US/美国"}, {"5", "JP/日本"}, {"6", "KR/韩国"}, {"7", "UK/英国"}, {"8", "EU/欧洲"}, {"9", "IN/印度"}, {"10", "SG/新加坡"}, {"11", "MY/马来西亚"}, {"12", "TH/泰国"}, {"13", "RU/俄罗斯"}, {"14", "CA/加拿大"}, {"15", "AU/澳大利亚"}, {"16", "Other"}, {"17", "DE/德国"}, {"18", "FR/法国"}},
			Tags:          []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"3", "DIY"}, {"4", "国语"}, {"5", "中字"}, {"6", "HDR"}, {"7", "完结"}, {"8", "Dolby Vision"}, {"9", "Atmos"}, {"10", "HDR10+"}, {"11", "短剧"}, {"12", "粤语"}, {"13", "驻站"}, {"14", "原创"}, {"15", "甄选"}, {"16", "特效"}},
		},
	},
	{
		Domain: "www.agsvpt.com", Name: "末日", BaseURL: "https://www.agsvpt.com",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "Movies/电影"}, {"402", "TV Series/电视剧"}, {"403", "TV Shows/综艺"}, {"404", "Documentaries/纪录片"}, {"405", "Animations/动漫"}, {"406", "Music Videos/音乐视频"}, {"407", "Sports/体育"}, {"408", "Audio/音频"}, {"409", "Misc/其他"}, {"410", "Cartoon/少儿动画"}, {"411", "Ebook/电子书"}, {"412", "Study/学习"}},
			MediumSel:   []SelectOption{{"1", "Blu-ray"}, {"2", "HD DVD"}, {"3", "Remux"}, {"4", "MiniBD"}, {"5", "HDTV"}, {"6", "DVDR"}, {"7", "Encode"}, {"8", "WEB-DL"}, {"9", "Track"}, {"10", "Other"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC"}, {"2", "H.265/HEVC"}, {"3", "VC-1"}, {"4", "MPEG-2"}, {"5", "AV1"}, {"6", "Other"}},
			StandardSel: []SelectOption{{"1", "8K/4320p"}, {"2", "4K/2160p"}, {"3", "1080p/1080i"}, {"4", "720p"}, {"5", "SD"}, {"6", "Other"}},
			AudioCodec:  []SelectOption{{"1", "FLAC"}, {"2", "APE"}, {"3", "DTS"}, {"4", "DTS-HD"}, {"5", "DTS-HD MA"}, {"6", "DTS:X"}, {"7", "TrueHD"}, {"8", "TrueHD Atmos"}, {"9", "LPCM"}, {"10", "DD/AC3"}, {"11", "DDP/E-AC3"}, {"12", "AAC"}, {"13", "MP3"}, {"14", "OGG"}, {"15", "WAV"}, {"16", "Other"}},
			TeamSel:     []SelectOption{{"1", "AGSVPT"}, {"2", "AGSVMUSIC"}, {"3", "Hares"}, {"4", "RL"}, {"5", "BeiTai"}, {"6", "PTer"}, {"7", "CHD"}, {"8", "HDHome"}, {"9", "CMCT"}, {"10", "Ourbits"}, {"11", "Other"}},
			Tags:        []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"3", "DIY"}, {"4", "国语"}, {"5", "中字"}, {"6", "HDR"}, {"7", "完结"}, {"8", "Dolby Vision"}, {"9", "Atmos"}, {"10", "HDR10+"}, {"11", "短剧"}, {"12", "粤语"}, {"13", "驻站"}, {"14", "原创"}, {"15", "甄选"}, {"16", "特效"}, {"17", "方舟计划"}, {"18", "冰种"}, {"19", "大包"}, {"20", "保种"}},
		},
	},
	{
		Domain: "www.hddolby.com", Name: "不可杜", BaseURL: "https://www.hddolby.com",
		Framework: "nexusphp", IsSource: true, IsTarget: false,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "Movies/电影"}, {"402", "TV Series/电视剧"}, {"403", "TV Shows/综艺"}, {"404", "Documentaries/纪录片"}, {"405", "Animations/动漫"}, {"406", "Music Videos/音乐视频"}, {"407", "Sports/体育"}, {"408", "HQ Audio/高品质音频"}, {"409", "Misc/其他"}, {"410", "Concert/演唱会"}, {"411", "Other/其他"}},
			MediumSel:   []SelectOption{{"1", "Blu-ray"}, {"2", "HD DVD"}, {"3", "Remux"}, {"4", "MiniBD"}, {"5", "HDTV"}, {"6", "DVDR"}, {"7", "Encode"}, {"8", "WEB-DL"}, {"9", "Track"}, {"10", "CD"}, {"11", "Other"}, {"12", "UHD Blu-ray"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC"}, {"2", "H.265/HEVC"}, {"3", "VC-1"}, {"4", "MPEG-2"}, {"5", "AV1"}, {"6", "Xvid"}, {"7", "MPEG-4"}, {"8", "VP9"}, {"9", "VVC/H.266"}, {"10", "Other"}, {"11", "x264"}},
			StandardSel: []SelectOption{{"1", "4K/2160p"}, {"2", "1080p/1080i"}, {"3", "720p"}, {"4", "SD"}, {"5", "8K/4320p"}, {"6", "Other"}},
			AudioCodec:  []SelectOption{{"1", "FLAC"}, {"2", "APE"}, {"3", "DTS"}, {"4", "DTS-HD"}, {"5", "DTS-HD MA"}, {"6", "DTS:X"}, {"7", "TrueHD"}, {"8", "TrueHD Atmos"}, {"9", "LPCM"}, {"10", "DD/AC3"}, {"11", "DDP/E-AC3"}, {"12", "AAC"}, {"13", "MP3"}, {"14", "OGG"}, {"15", "WAV"}, {"16", "Other"}, {"17", "ALAC"}, {"18", "AV3A"}},
			TeamSel:     []SelectOption{{"1", "Dream"}, {"2", "QHstudIo"}, {"3", "CornerMV"}, {"4", "DBTV"}, {"5", "Telesto"}, {"6", "Other"}, {"7", "FRDS"}, {"8", "beAst"}, {"9", "CMCT"}, {"10", "WiKi"}, {"11", "HDS"}, {"12", "OurBits"}, {"13", "PTer"}, {"14", "HDHome"}},
		},
	},
	{
		Domain: "ptlgs.org", Name: "劳改所", BaseURL: "https://ptlgs.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "Movies/电影"}, {"402", "TV Series/电视剧"}, {"403", "TV Shows/综艺"}, {"404", "Documentaries/纪录片"}, {"405", "Animations/动漫"}, {"406", "Music Videos/音乐视频"}, {"407", "Sports/体育"}, {"408", "Audio/音频"}, {"409", "Misc/其他"}},
			MediumSel:   []SelectOption{{"1", "Blu-ray"}, {"2", "HD DVD"}, {"3", "Remux"}, {"4", "MiniBD"}, {"5", "HDTV"}, {"6", "DVDR"}, {"7", "Encode"}, {"8", "WEB-DL"}, {"9", "Track"}, {"10", "CD"}, {"11", "Other"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC"}, {"2", "H.265/HEVC"}, {"3", "VC-1"}, {"4", "MPEG-2"}, {"5", "Other"}},
			StandardSel: []SelectOption{{"1", "8K/4320p"}, {"2", "4K/2160p"}, {"3", "1080p/1080i"}, {"4", "720p"}, {"5", "SD"}, {"6", "Other"}},
			AudioCodec:  []SelectOption{{"1", "FLAC"}, {"2", "APE"}, {"3", "DTS"}, {"4", "DTS-HD"}, {"5", "DTS-HD MA"}, {"6", "DTS:X"}, {"7", "TrueHD"}, {"8", "TrueHD Atmos"}, {"9", "LPCM"}, {"10", "DD/AC3"}, {"11", "Other"}},
			TeamSel:     []SelectOption{{"1", "DYZ-WEB"}, {"2", "DYZ-Movie"}, {"3", "DYZ-TV"}, {"4", "Eleph"}, {"5", "beAst"}, {"6", "ZmWeb"}, {"7", "Other"}},
			Tags:        []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"3", "DIY"}, {"4", "国语"}, {"5", "中字"}, {"6", "HDR"}, {"7", "完结"}, {"8", "分集"}, {"9", "短剧"}, {"10", "原创"}, {"11", "驻站"}, {"12", "杜比"}, {"13", "粤语"}, {"14", "无损"}, {"15", "汉化"}, {"16", "特效"}, {"17", "甄选"}, {"18", "原生"}, {"19", "大包"}},
		},
	},
	{
		Domain: "hhanclub.net", Name: "憨憨", BaseURL: "https://hhanclub.net",
		Framework: "nexusphp", IsSource: true, IsTarget: false,
		Form: SiteFormConfig{
			Category: []SelectOption{{"401", "电影"}, {"402", "电视剧"}, {"403", "综艺"}, {"405", "动漫"}, {"404", "纪录片"}, {"407", "Sports"}, {"409", "其他"}, {"412", "短剧"}},
		},
	},
	{
		Domain: "www.qingwapt.com", Name: "青蛙", BaseURL: "https://www.qingwapt.com",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		CookieCloudDomain: "qingwapt.com",
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "电影"}, {"402", "剧集"}, {"403", "综艺"}, {"405", "动漫"}, {"404", "纪录片"}, {"406", "MV"}, {"407", "体育"}, {"408", "音乐"}, {"412", "短剧"}, {"409", "其他"}},
			SourceSel:   []SelectOption{{"1", "UHD Blu-ray"}, {"8", "Blu-ray"}, {"9", "Remux"}, {"10", "Encode"}, {"7", "WEB-DL"}, {"4", "HDTV"}, {"2", "DVD"}, {"3", "CD"}, {"11", "MiniBD"}, {"5", "Track"}, {"6", "Other"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC"}, {"6", "H.265/HEVC"}, {"2", "VC-1"}, {"4", "MPEG-2"}, {"7", "AV1"}, {"3", "MPEG-4"}, {"8", "VP9"}, {"5", "Other"}},
			AudioCodec:  []SelectOption{{"9", "DTS:X"}, {"14", "DTS"}, {"10", "DTS-HD MA"}, {"21", "DTS-HD HRA"}, {"11", "TrueHD Atmos"}, {"12", "TrueHD"}, {"13", "LPCM"}, {"15", "DD/AC3"}, {"16", "DDP/E-AC3"}, {"1", "FLAC"}, {"17", "AAC"}, {"18", "APE"}, {"19", "WAV"}, {"4", "MP3"}, {"8", "M4A"}, {"20", "OPUS"}, {"22", "AV3A"}, {"7", "Other"}},
			StandardSel: []SelectOption{{"6", "8K/4320p"}, {"7", "4K/2160p"}, {"8", "2K/1440p"}, {"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}, {"5", "Other"}},
			TeamSel:     []SelectOption{{"6", "FROG"}, {"7", "FROGE"}, {"8", "FROGWeb"}, {"10", "CatEDU"}, {"5", "Other"}},
		},
	},
	{
		Domain: "pterclub.net", Name: "猫", BaseURL: "https://pterclub.net",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:  []SelectOption{{"401", "电影 (Movie)"}, {"404", "电视剧 (TV Series)"}, {"403", "动画 (Animation)"}, {"405", "综艺 (TV Show)"}, {"413", "音乐短片 (MV)"}, {"406", "音乐 (Music)"}, {"418", "舞台演出(Stage Performance)"}, {"402", "纪录片 (Documentary)"}, {"407", "体育 (Sport)"}, {"408", "电子书 (Ebook)"}, {"410", "软件 (Software)"}, {"411", "学习 (Study)"}, {"412", "其它 (Other)"}},
			SourceSel: []SelectOption{{"1", "UHD Discs"}, {"2", "BD Discs"}, {"3", "Remux"}, {"4", "HDTV"}, {"5", "WEB-DL"}, {"6", "Encode"}, {"7", "DVD Discs"}, {"8", "FLAC"}, {"9", "WAV"}, {"10", "ISO"}, {"11", "PDF"}, {"12", "PUB"}, {"13", "AZW"}, {"14", "MOBI"}, {"15", "Other"}},
			TeamSel:   []SelectOption{{"1", "大陆 (Mainland,CHN)"}, {"2", "香港 (HKG,CHN)"}, {"3", "台湾 (TWN,CHN)"}, {"4", "欧美 (Western)"}, {"5", "韩国 (KOR)"}, {"6", "日本 (JPN)"}, {"7", "印度 (IND)"}, {"8", "其它 (Other)"}},
			Tags:      []SelectOption{{"jinzhuan", "禁转"}, {"guanfang", "官方"}, {"guoyu", "国语"}, {"yueyu", "粤语"}, {"zhongzi", "中字"}, {"ensub", "英字"}, {"yingqiu", "应求"}, {"diy", "DIY"}, {"pr", "原创"}, {"bim", "自购"}, {"mp", "MV母盘"}},
		},
	},
	{
		Domain: "pt.gtkpw.xyz", Name: "GTK", BaseURL: "https://pt.gtkpw.xyz",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "Movies"}, {"402", "TV Series"}, {"403", "TV Shows"}, {"404", "Documentaries"}, {"405", "Animations"}, {"406", "Music Videos"}, {"407", "Sports"}, {"408", "HQ Audio"}, {"409", "Misc"}, {"410", "Book"}, {"411", "Music Album"}, {"412", "Education"}},
			MediumSel:   []SelectOption{{"1", "Blu-ray"}, {"2", "HD DVD"}, {"3", "Remux"}, {"4", "MiniBD"}, {"5", "HDTV"}, {"6", "DVDR"}, {"7", "Encode"}, {"8", "CD"}, {"9", "Track"}, {"10", "UHD"}, {"11", "WEB-DL"}},
			CodecSel:    []SelectOption{{"1", "H.264"}, {"2", "VC-1"}, {"3", "Xvid"}, {"4", "MPEG-2"}, {"5", "Other"}, {"6", "H.265/HEVC"}, {"7", "AV1"}, {"8", "VP9"}},
			StandardSel: []SelectOption{{"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}, {"5", "2160p/4K"}, {"6", "4320p/8K"}},
			TeamSel:     []SelectOption{{"1", "HDS"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}, {"6", "CMCT"}, {"7", "MARK"}, {"8", "MTeam"}, {"9", "FRDS"}, {"10", "PTHome"}, {"11", "beAst"}},
			Tags:        []SelectOption{{"1", "禁转"}, {"2", "首发"}, {"4", "DIY"}, {"5", "国语"}, {"6", "中字"}, {"7", "HDR"}},
		},
	},
	{
		Domain: "monikadesign.uk", Name: "莫妮卡", BaseURL: "https://monikadesign.uk",
		Framework: "unit3d", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"8", "Anime TV"}, {"6", "Anime Movie"}, {"2", "TV"}, {"1", "Movie"}, {"9", "Music of TV"}, {"3", "Music of Movie"}, {"7", "Anime Live"}, {"5", "Action Live"}, {"4", "Game"}, {"11", "Airing Anime TV"}},
			SourceSel:   []SelectOption{{"1", "Full Disc"}, {"2", "Remux"}, {"3", "Encode"}, {"4", "WEB-DL"}, {"5", "WEBRip"}, {"6", "HDTV"}, {"7", "ALBUM"}, {"14", "SINGLE"}, {"15", "OST"}, {"16", "DRAMA"}},
			StandardSel: []SelectOption{{"1", "4320p"}, {"2", "2160p"}, {"3", "1080p"}, {"4", "1080i"}, {"5", "720p"}, {"6", "576p"}, {"7", "576i"}, {"8", "480p"}, {"9", "480i"}, {"10", "Other"}, {"11", "Lossless"}, {"12", "Hi-Res"}, {"13", "Lossy"}},
			TeamSel:     []SelectOption{{"1008", "101 Anime"}, {"17", "@Anime"}, {"47", "Anime Limited"}, {"66", "Aniplex"}, {"99", "avex pictures"}, {"105", "Bandai Namco Arts"}, {"219", "Crunchyroll"}, {"253", "Disney / Buena Vista"}, {"458", "KADOKAWA"}, {"469", "King Records"}, {"971", "Kyoto Animation"}, {"586", "Muse 木棉花"}, {"616", "NHK"}, {"682", "Pony Canyon"}, {"758", "Sentai Filmworks"}, {"769", "Shochiku"}, {"771", "Shogakukan"}, {"793", "Sony Music"}, {"794", "Sony Pictures"}, {"863", "Toei"}, {"864", "Toho"}, {"929", "VIZ Media"}, {"935", "Warner Bros."}},
		},
	},
	{
		Domain: "dicmusic.com", Name: "海豚", BaseURL: "https://dicmusic.com",
		Framework: "gazelle", IsSource: false, IsTarget: true,
		Form: SiteFormConfig{
			Category:   []SelectOption{{"1", "Album"}, {"3", "Soundtrack"}, {"5", "EP"}, {"6", "Compilation"}, {"7", "Anthology"}, {"9", "Single"}, {"11", "Live"}, {"13", "Remix"}, {"14", "Bootleg"}, {"15", "Interview"}, {"16", "Mixtape"}, {"17", "Demo"}, {"18", "Concert Recording"}, {"19", "DJ Mix"}, {"21", "Unknown"}},
			MediumSel:  []SelectOption{{"CD", "CD"}, {"DVD", "DVD"}, {"Vinyl", "Vinyl"}, {"Soundboard", "Soundboard"}, {"SACD", "SACD"}, {"Blu-ray", "Blu-ray"}, {"DAT", "DAT"}, {"Cassette", "Cassette"}, {"WEB", "WEB"}, {"Unknown Media", "Unknown Media"}},
			CodecSel:   []SelectOption{{"FLAC", "FLAC"}, {"DSD", "DSD"}, {"MP3", "MP3"}, {"AAC", "AAC"}, {"AC3", "AC3"}, {"DTS", "DTS"}},
			AudioCodec: []SelectOption{{"192", "192"}, {"APS (VBR)", "APS (VBR)"}, {"V2 (VBR)", "V2 (VBR)"}, {"V1 (VBR)", "V1 (VBR)"}, {"256", "256"}, {"APX (VBR)", "APX (VBR)"}, {"V0 (VBR)", "V0 (VBR)"}, {"q8.x (VBR)", "q8.x (VBR)"}, {"320", "320"}, {"Lossless", "Lossless"}, {"24bit Lossless", "24bit Lossless"}, {"Other", "Other"}},
		},
	},
	{
		Domain: "zhuque.in", Name: "朱雀", BaseURL: "https://zhuque.in",
		Framework: "tnode", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"501", "电影"}, {"502", "剧集"}, {"503", "动画"}, {"504", "节目"}, {"599", "其它"}},
			MediumSel:   []SelectOption{{"301", "UHD Blu-ray"}, {"302", "UHD Blu-ray DIY"}, {"303", "Blu-ray"}, {"304", "Blu-ray DIY"}, {"305", "Remux"}, {"306", "Encode"}, {"307", "UHDTV"}, {"308", "HDTV"}, {"309", "WEB-DL"}, {"399", "Other"}},
			CodecSel:    []SelectOption{{"101", "H264"}, {"102", "H265"}, {"103", "x264"}, {"104", "x265"}, {"199", "Other"}},
			StandardSel: []SelectOption{{"401", "720p"}, {"402", "1080i"}, {"403", "1080p"}, {"404", "2160p"}, {"499", "Other"}},
			Tags:        []SelectOption{{"601", "官方"}, {"602", "禁转"}, {"603", "国语"}, {"604", "中字"}, {"611", "杜比视界"}, {"613", "HDR10"}, {"614", "特效字幕"}, {"621", "完结"}, {"622", "分集"}},
		},
	},
	{
		Domain: "rousi.pro", Name: "肉丝", BaseURL: "https://rousi.pro",
		Framework: "rousi", AuthType: "passkey", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category: []SelectOption{
				{"movie", "电影"}, {"tv", "电视剧"}, {"documentary", "纪录片"},
				{"animation", "动漫"}, {"music", "音乐"}, {"variety", "综艺"},
				{"sports", "体育"}, {"software", "软件"}, {"ebook", "电子书"},
				{"other", "其它"},
			},
			MediumSel: []SelectOption{
				{"UHD Blu-ray", "UHD Blu-ray"}, {"Blu-ray", "Blu-ray"}, {"WEB-DL", "WEB-DL"},
				{"HDTV", "HDTV"}, {"DVDRip", "DVDRip"}, {"CAM", "CAM"}, {"其它", "其它"},
			},
			StandardSel: []SelectOption{
				{"4K / 2160p", "4K / 2160p"}, {"1080p", "1080p"}, {"1080i", "1080i"},
				{"720p", "720p"}, {"SD", "SD"}, {"其它", "其它"},
			},
			TeamSel: []SelectOption{},
		},
	},
	{
		Domain: "totheglory.im", Name: "套套哥", BaseURL: "https://totheglory.im",
		Framework: "generic", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category: []SelectOption{{"51", "电影DVDRip"}, {"52", "电影720p"}, {"53", "电影1080i/p"}, {"54", "BluRay原盘"}, {"108", "影视2160p"}, {"109", "UHD原盘"}, {"62", "纪录片720p"}, {"63", "纪录片1080i/p"}, {"67", "纪录片BluRay原盘"}, {"69", "欧美剧720p(单集)"}, {"70", "欧美剧1080i/p(单集)"}, {"73", "高清日剧"}, {"76", "大陆港台剧720p(单集)"}, {"75", "大陆港台剧1080i/p(单集)"}, {"74", "高清韩剧"}, {"87", "欧美剧包(全集)"}, {"88", "日剧包"}, {"99", "韩剧包"}, {"90", "华语剧包(全集)"}, {"58", "高清动漫"}, {"111", "动漫原盘"}, {"60", "高清综艺"}, {"101", "日本综艺"}, {"103", "韩国综艺"}, {"59", "MV&演唱会"}, {"82", "OST"}, {"83", "无损音乐FLAC&APE"}, {"84", "补充音轨"}, {"57", "高清体育节目"}, {"56", "Ebook"}, {"28", "PC"}, {"47", "MAC"}, {"77", "APPZ"}, {"110", "SWITCH"}, {"104", "PS4"}, {"45", "XBOX to XBOX360"}, {"46", "PS3"}, {"107", "PSV"}, {"44", "NDS"}, {"106", "WIIU"}, {"27", "WII"}, {"43", "NGC"}, {"91", "MiniVideo"}, {"92", "iPhone/iPad视频"}, {"93", "iPhone/iPad游戏"}, {"94", "iPad书籍"}, {"95", "iPhone/iPad软件"}},
			Tags:     []SelectOption{{"nodistr_yes", "禁转"}},
		},
	},
	{
		Domain: "hdroute.org", Name: "在脚下", BaseURL: "http://hdroute.org",
		Framework: "generic", IsSource: true, IsTarget: false,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"1", "电影"}, {"2", "纪录片"}, {"3", "连续剧"}, {"4", "动画片"}, {"5", "演唱会"}, {"6", "体育节目"}, {"7", "电影音轨"}, {"8", "无损音乐"}, {"9", "其他"}},
			MediumSel:   []SelectOption{{"1", "Blu-ray原盘"}, {"2", "Remux"}, {"3", "HDTV/Rip"}, {"4", "BDRe(重编码)"}, {"5", "CD"}, {"6", "其他"}, {"7", "MP4"}},
			CodecSel:    []SelectOption{{"1", "H(X).264"}, {"2", "VC-1"}, {"3", "MPEG-2"}, {"4", "Xvid"}, {"5", "其他"}, {"6", "MVC"}, {"7", "H(X).265"}},
			StandardSel: []SelectOption{{"1", "1080P"}, {"2", "1080i"}, {"4", "720P"}, {"6", "其他"}, {"7", "4K2K"}},
			AudioCodec:  []SelectOption{{"1", "LPCM"}, {"2", "DTSHDMA"}, {"3", "TrueHD"}, {"4", "DTS/(Core)"}, {"5", "AC-3(DD)"}, {"6", "APE"}, {"7", "FLAC"}, {"8", "其他"}, {"9", "AAC"}},
		},
	},
	{
		Domain: "star-space.net", Name: "影", BaseURL: "https://star-space.net",
		Framework: "generic", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"mo", "电影"}, {"tv", "剧集"}, {"an", "动画"}, {"do", "纪录片"}, {"mv", "MV"}, {"sp", "体育"}, {"ot", "综艺"}},
			MediumSel:   []SelectOption{{"s11", "Other"}, {"s13", "Web-DL"}, {"s21", "HDTV Encode"}, {"s22", "HDTV"}, {"s31", "DVD Encode"}, {"s32", "DVD Remux"}, {"s33", "DVD ISO"}, {"s41", "BD Encode"}, {"s42", "BD Remux"}, {"s43", "BD DIY"}, {"s44", "BD ISO"}, {"s51", "UHD Encode"}, {"s52", "UHD Remux"}, {"s53", "UHD DIY"}, {"s54", "UHD ISO"}},
			CodecSel:    []SelectOption{{"1", "H264 (AVC)"}, {"2", "H265 (HEVC)"}, {"3", "MPEG"}, {"4", "Other"}, {"5", "VC-1"}, {"6", "x264"}, {"7", "x265"}, {"8", "Xvid"}},
			AudioCodec:  []SelectOption{{"1", "AAC"}, {"2", "APE"}, {"3", "DD/DD+/AC3"}, {"4", "DTS"}, {"5", "DTS-HD HR"}, {"6", "DTS-HD MA"}, {"7", "DTS-X"}, {"8", "FLAC"}, {"9", "LPCM"}, {"10", "M4A"}, {"11", "MP3"}, {"12", "OGG"}, {"13", "Other"}, {"14", "TrueHD"}, {"15", "TrueHD Atmos"}, {"16", "WAV"}},
			StandardSel: []SelectOption{{"r1", "SD"}, {"r2", "720"}, {"r3", "1080"}, {"r4", "2160"}, {"r5", "4320"}},
			TeamSel:     []SelectOption{{"2", "YingWEB"}, {"1", "Ying"}, {"3", "YingDIY"}, {"7", "YingMUSIC"}, {"4", "YingMV"}, {"6", "YingHDTV"}, {"8", "CatEDU"}, {"9", "Telesto"}, {"5", "Other"}},
			Tags:        []SelectOption{{"tag_gf", "官方"}, {"tag_xiaozu", "驻站组"}, {"tag_jz", "禁转"}, {"tag_3d", "3D"}, {"tag_chs_sub", "中字"}, {"tag_chs_lang", "国语"}, {"tag_yueyu", "粤语"}, {"tag_eng_sub", "英字"}, {"tag_eng_lang", "英语"}, {"tag_ep", "分集"}, {"tag_complete", "完结"}},
		},
	},
	{
		Domain: "52pt.site", Name: "五二", BaseURL: "https://52pt.site",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "Movies/电影"}, {"404", "Documentaries/纪录片"}, {"405", "Animations/动漫(画)"}, {"402", "TV Series/剧集"}, {"403", "TV Shows/综艺"}, {"406", "Music Videos/音乐MV"}, {"407", "Sports/体育"}, {"408", "HQ Audio/无损音乐"}, {"409", "Misc/其他"}, {"410", "Ebook/电子书"}, {"411", "Edu/学习教育"}},
			MediumSel:   []SelectOption{{"2", "Blu-ray DIY"}, {"4", "Blu-ray Remux"}, {"11", "Blu-ray原盘无中文"}, {"1", "4K UHD无中文"}, {"14", "2K原盘中字"}, {"15", "4K原盘中字"}, {"3", "Encode"}, {"5", "HDTV"}, {"6", "WEB-DL"}, {"7", "DVD"}, {"8", "CD"}, {"9", "Track"}, {"10", "Other"}, {"12", "Remux"}, {"13", "UHD Remux"}, {"16", "UHD Encode"}},
			CodecSel:    []SelectOption{{"13", "H.264/AVC"}, {"1", "H.265(HEVC)"}, {"4", "MPEG-2"}, {"5", "Other"}, {"11", "X264"}, {"14", "H.265"}, {"15", "X265"}, {"3", "VC-1"}, {"12", "AV1"}},
			AudioCodec:  []SelectOption{{"4", "DTS.HDMA"}, {"12", "True.HD"}, {"10", "TRUE.HD Atmos"}, {"3", "DTS:X"}, {"6", "AC3/DD"}, {"14", "LPCM"}, {"1", "FLAC"}, {"2", "APE"}, {"5", "DTSES"}, {"7", "AAC"}, {"8", "DTS"}, {"9", "DTS.HD"}, {"11", "MP3"}, {"13", "Other"}},
			StandardSel: []SelectOption{{"1", "2K/1080p"}, {"5", "4K/2160P"}, {"2", "1080i"}, {"4", "1080P-3D"}, {"3", "720p"}, {"6", "others"}, {"7", "8K/4320p"}},
			TeamSel:     []SelectOption{{"6", "52PT DIY"}, {"7", "52PT REMUX"}, {"1", "BeyondHD"}, {"2", "HDSKY"}, {"3", "TTG"}, {"8", "MTeam"}, {"9", "FRDS"}, {"4", "EbP"}, {"5", "Other"}, {"10", "CMCT"}, {"11", "HDS"}, {"12", "MySiLU"}, {"13", "WiKi"}, {"14", "CHD"}, {"15", "PTHome"}},
		},
	},
	{
		Domain: "crabpt.vip", Name: "蟹黄堡", BaseURL: "https://crabpt.vip",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category: []SelectOption{
				{"401", "电影/Movies"}, {"402", "电视剧/TVSeries"}, {"403", "综艺/TV Shows"},
				{"404", "纪录片/Documentary"}, {"405", "动漫/Anime"}, {"406", "MV"},
				{"407", "体育竞技/Sports"}, {"408", "音乐/Music"},
				{"409", "其他/Other"}, {"413", "短剧/Playlet"},
			},
			SourceSel: []SelectOption{
				{"2", "BluRay"}, {"3", "UHD Blu-ray"}, {"4", "Remux"}, {"5", "Encode"},
				{"6", "WEB-DL"}, {"7", "HDTV"}, {"8", "CD"}, {"9", "MVC"},
				{"10", "ProRes"}, {"11", "Xvid"}, {"1", "Other"},
			},
			StandardSel: []SelectOption{
				{"5", "8K"}, {"4", "4K/2160p"}, {"3", "1080p/1080i"},
				{"2", "720p"}, {"1", "Other"},
			},
			CodecSel: []SelectOption{
				{"2", "AVC/H.264/x264"}, {"3", "HEVC/H.265/x265"}, {"4", "H.266/VVC"},
				{"5", "VP9"}, {"6", "AV1"}, {"14", "VC-1"}, {"15", "MPEG"}, {"1", "Other"},
			},
			AudioCodec: []SelectOption{
				{"2", "AAC"}, {"3", "DD/AC3"}, {"4", "DDP/E-AC3"}, {"5", "DTS"},
				{"6", "TrueHD"}, {"7", "LPCM"}, {"8", "DTS:X"}, {"9", "MPEG"},
				{"10", "FLAC"}, {"11", "WAV"}, {"12", "APE"}, {"15", "DTS-HD"},
				{"16", "ALAC"}, {"17", "DTS-HD MA"}, {"21", "OGG"}, {"22", "DTS-HD"},
				{"23", "DSD"}, {"24", "Opus"}, {"26", "Atmos"}, {"1", "Other"},
			},
			TeamSel: []SelectOption{
				{"2", "CHD"}, {"3", "HDS"}, {"4", "WiKi"}, {"5", "OurBits"}, {"6", "XHB"},
				{"7", "FRDS"}, {"8", "HHWEB"}, {"9", "UBits"}, {"10", "Audiences"},
				{"11", "DYZ-WEB"}, {"12", "DYZ-Movie"}, {"13", "DYZ-TV"}, {"15", "AGSVWEB"},
				{"16", "ZmWeb"}, {"17", "Pter"}, {"18", "FFans"}, {"19", "UBWEB"},
				{"20", "HDFans"}, {"21", "PigoHD"}, {"22", "PigoWeb"}, {"23", "PiGoNF"},
				{"24", "QHstudIo"}, {"25", "ADWeb"}, {"27", "CMCT"}, {"29", "FROG"},
				{"30", "FROGWeb"}, {"32", "tlf"}, {"33", "beAst"}, {"35", "HDHome"},
				{"36", "HDVWEB"}, {"37", "MTeam"}, {"1", "Other"},
			},
			ProcessingSel: []SelectOption{
				{"2", "中国大陆/CN"}, {"3", "港台/HK/TW"}, {"4", "欧美/EU/US"},
				{"5", "日本/JP"}, {"6", "韩国/KR"}, {"7", "印度/India"}, {"1", "其他/Other"},
			},
			Tags: []SelectOption{
				{"1", "禁转"}, {"2", "自购"}, {"4", "原盘DIY"}, {"5", "国语"}, {"6", "粤语"},
				{"7", "中字"}, {"8", "完结"}, {"9", "分集"}, {"10", "特效字幕"},
				{"11", "HDR"}, {"12", "Dolby Vision"}, {"13", "Dolby Atmos"}, {"19", "合集大包"},
				{"20", "伦理"}, {"22", "驻站"}, {"24", "AGSV"}, {"25", "短剧"},
				{"32", "剧情"}, {"33", "喜剧"}, {"34", "动作"}, {"35", "爱情"},
				{"36", "科幻"}, {"37", "动画"}, {"38", "悬疑"}, {"39", "惊悚"},
				{"40", "恐怖"}, {"41", "纪录片"}, {"42", "历史"}, {"43", "战争"},
				{"44", "犯罪"}, {"45", "奇幻"}, {"46", "冒险"}, {"47", "灾难"},
				{"48", "武侠"}, {"50", "ASMR"}, {"59", "未完结"},
			},
		},
	},
	{
		Domain: "pt.itzmx.com", Name: "分享站", BaseURL: "https://pt.itzmx.com",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"405", "动画"}, {"404", "漫画"}, {"408", "音乐"}, {"401", "电影"}, {"402", "电视剧"}, {"414", "蓝光"}, {"403", "综艺"}, {"409", "其他"}},
			StandardSel: []SelectOption{{"1", "1080p"}, {"2", "720p"}, {"3", "2160p"}},
			TeamSel:     []SelectOption{{"1", "Other"}},
		},
	},
	{
		Domain: "ptsbao.club", Name: "烧包", BaseURL: "https://ptsbao.club",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"424", "限制"}, {"409", "其他"}, {"423", "原创"}, {"404", "纪录片"}, {"414", "音乐"}, {"405", "动漫"}, {"403", "综艺"}, {"401", "电影"}, {"402", "剧集"}},
			MediumSel:     []SelectOption{{"11", "AI"}, {"9", "其他"}, {"8", "CD"}, {"3", "DVD"}, {"5", "HDTV"}, {"2", "WEB-DL"}, {"6", "Encode"}, {"4", "Remux"}, {"1", "Blu-ray"}, {"7", "UHD"}},
			CodecSel:      []SelectOption{{"5", "Other"}, {"8", "ProRes"}, {"3", "Xvid/DivX"}, {"7", "VP9"}, {"4", "MPEG-2"}, {"2", "VC-1"}, {"1", "H.264/AVC"}, {"6", "H.265/HEVC"}},
			AudioCodec:    []SelectOption{{"7", "Other"}, {"15", "MP2"}, {"14", "OPUS"}, {"13", "DTS:X"}, {"12", "LPCM/PCM"}, {"5", "OGG"}, {"6", "AAC"}, {"4", "MP3"}, {"3", "AC3"}, {"2", "DTS"}, {"1", "FLAC"}, {"8", "DTS-HD"}, {"9", "DTS-HD MA"}, {"10", "TrueHD"}, {"11", "TrueHD Atmos"}, {"16", "WAV"}},
			StandardSel:   []SelectOption{{"6", "Other"}, {"4", "480"}, {"3", "576"}, {"2", "720"}, {"1", "1080"}, {"7", "2K"}, {"8", "4K"}},
			ProcessingSel: []SelectOption{{"6", "AI"}, {"3", "Other"}, {"5", "原盘"}, {"2", "重编码"}, {"4", "源码"}, {"1", "Remux"}},
			TeamSel:       []SelectOption{{"20", "SGXT"}, {"19", "FFansAIeNcE"}, {"18", "QHstudIo"}, {"17", "19977"}, {"16", "Enichi"}, {"15", "FHDMv"}, {"14", "DSNAS"}, {"13", "EDU"}, {"12", "SBC"}, {"11", "CMCT"}, {"10", "LT"}, {"9", "SMZ"}, {"8", "ZM"}, {"7", "JX"}, {"6", "PTB"}, {"5", "Other"}},
		},
	},
	{
		Domain: "pt.soulvoice.club", Name: "聆音", BaseURL: "https://pt.soulvoice.club",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"500", "电子书"}, {"499", "有声书"}},
			MediumSel:     []SelectOption{{"1", "Blu-ray"}, {"4", "MiniBD"}, {"7", "Encode"}, {"10", "APE/FLAC"}, {"11", "DSD"}, {"12", "Other"}},
			CodecSel:      []SelectOption{{"5", "Other"}, {"2", "H.265"}, {"1", "H.264"}},
			StandardSel:   []SelectOption{{"4", "Other"}, {"3", "2160P"}, {"2", "1080I"}, {"1", "1080P"}},
			TeamSel:       []SelectOption{{"1", "HDS"}, {"2", "CHD"}, {"3", "FRDS"}, {"4", "CMCT"}, {"5", "Other"}},
			SourceSel:     []SelectOption{{"3", "其它"}, {"2", "英文"}, {"1", "中文"}},
			AudioCodec:    []SelectOption{{"19", "Other"}, {"18", "PDF"}, {"3", "MOBI"}, {"2", "EPUB"}, {"1", "AZW/AZW3"}},
			ProcessingSel: []SelectOption{{"9", "其它"}, {"13", "耽美"}, {"12", "漫画"}, {"11", "轻小说"}, {"8", "英文原版"}, {"10", "网络小说"}, {"7", "杂志"}, {"6", "教材"}, {"5", "社科"}, {"4", "文学"}, {"3", "商业"}, {"2", "技术"}, {"1", "Other"}},
		},
	},
	{
		Domain: "ptzone.xyz", Name: "PT地带", BaseURL: "https://ptzone.xyz",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"401", "Movies"}, {"402", "TV Series"}, {"403", "TV Shows"}, {"404", "Documentaries"}, {"405", "Animations"}, {"406", "Music"}, {"407", "Sports"}, {"408", "HQ Audio"}, {"409", "Misc"}, {"410", "Ebook"}, {"411", "Edu"}},
			MediumSel:   []SelectOption{{"9", "Track"}, {"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"4", "WEB-DL"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "Blu-ray"}, {"1", "UHD"}, {"10", "Other"}},
			CodecSel:    []SelectOption{{"1", "H.264"}, {"2", "VC-1"}, {"3", "MPEG-2"}, {"4", "MPEG-4"}, {"5", "Other"}, {"6", "H.265"}},
			AudioCodec:  []SelectOption{{"1", "FLAC"}, {"2", "APE"}, {"3", "DTS"}, {"4", "MP3"}, {"5", "OGG"}, {"6", "AAC"}, {"7", "Other"}, {"8", "DTS-HD MA"}, {"9", "LPCM"}, {"10", "AC3"}, {"11", "ALAC"}, {"12", "WAV"}, {"13", "M4A"}, {"14", "AIFF"}, {"15", "OPUS"}},
			StandardSel: []SelectOption{{"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}, {"5", "8K"}, {"6", "4K"}},
			TeamSel:     []SelectOption{{"6", "PTZWeb"}, {"1", "HDS"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}},
		},
	},
	{
		Domain: "wintersakura.net", Name: "冬樱", BaseURL: "https://wintersakura.net",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"427", "软件/程序/代码"}, {"426", "期刊/论文"}, {"428", "图书"}, {"425", "数据/数据库"}, {"424", "课程"}},
			MediumSel:   []SelectOption{{"25", "Webrip"}, {"24", "3D Blu-ray"}, {"22", "SACD"}, {"18", "CD"}, {"17", "DVDR"}, {"16", "HDTV"}, {"15", "DVD"}, {"14", "WEB-DL"}, {"13", "Encode"}, {"12", "Remux"}, {"11", "Blu-ray"}, {"10", "UHD"}, {"9", "Track"}, {"23", "Other"}},
			CodecSel:    []SelectOption{{"13", "Other"}, {"15", "ProRes"}, {"16", "AV1"}, {"11", "MPEG-2"}, {"10", "VC-1"}, {"8", "x264"}, {"9", "x265"}, {"12", "H.265/HEVC"}, {"7", "H.264/AVC"}},
			AudioCodec:  []SelectOption{{"21", "Other"}, {"27", "Opus"}, {"26", "DD+ Atmos"}, {"25", "DD+"}, {"24", "DSD"}, {"23", "WAV"}, {"22", "M4A"}, {"20", "MP3"}, {"19", "AIFF"}, {"18", "ALAC"}, {"17", "OGG"}, {"16", "AAC"}, {"15", "MP2"}, {"14", "LPCM"}, {"13", "FLAC"}, {"12", "APE"}, {"11", "DD/AC3"}, {"10", "TrueHD Atmos"}, {"9", "TrueHD"}, {"8", "DTS:X"}, {"7", "DTS-HD MA"}, {"6", "DTS-HD HR"}, {"5", "DTS"}, {"4", "DDP"}, {"3", "DST"}, {"2", "DTS ES"}, {"1", "DTS 96/24"}},
			StandardSel: []SelectOption{{"4", "SD"}, {"3", "720p"}, {"2", "1080i"}, {"1", "2K/1080p"}, {"5", "4K/2160p"}, {"6", "8K/4320p"}},
			TeamSel:     []SelectOption{{"5", "Other"}, {"17", "tjupt"}, {"8", "PTer"}, {"7", "FRDS"}, {"6", "CMCT"}, {"15", "ttg"}, {"14", "MySiLU"}, {"13", "HDS"}, {"12", "CHD"}, {"11", "WiKi"}, {"10", "beAst"}, {"9", "PTHome"}, {"16", "LGQ"}},
		},
	},
	{
		Domain: "www.htpt.cc", Name: "海棠", BaseURL: "https://www.htpt.cc",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"1", "相声"}, {"4091", "评书"}, {"4097", "戏曲"}, {"4098", "鼓/琴"}, {"4099", "小曲"}, {"4101", "小品"}, {"4100", "其他"}, {"4102", "视频"}},
			CodecSel:    []SelectOption{{"1", "H.264/AVC/x264"}, {"10", "H.265/HEVC/X.265"}, {"11", "MP3/音频/M4A"}},
			StandardSel: []SelectOption{{"1", "4K"}, {"2", "1080p"}, {"3", "720p"}, {"5", "SD/标清"}},
			TeamSel:     []SelectOption{{"1", "HTPT"}},
		},
	},
	{
		Domain: "www.ptlao.top", Name: "忘年桥", BaseURL: "https://www.ptlao.top",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:    []SelectOption{{"410", "old"}, {"411", "middle"}, {"412", "men and women"}, {"413", "youth"}},
			MediumSel:   []SelectOption{{"9", "Track"}, {"8", "CD"}, {"6", "DVDR"}, {"5", "HDTV"}, {"4", "MiniBD"}, {"7", "Encode"}, {"3", "Remux"}, {"2", "Blu-ray"}, {"1", "UHD"}, {"10", "Other"}, {"11", "WEB-DL"}},
			CodecSel:    []SelectOption{{"1", "H.264"}, {"2", "VC-1"}, {"3", "Xvid"}, {"4", "MPEG-2"}, {"5", "Other"}, {"8", "H.265(HEVC)"}},
			StandardSel: []SelectOption{{"5", "4k/2160p"}, {"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"4", "SD"}},
			TeamSel:     []SelectOption{{"6", "PTL"}, {"1", "HDS"}, {"2", "CHD"}, {"3", "MySiLU"}, {"4", "WiKi"}, {"5", "Other"}},
		},
	},
	{
		Domain: "www.pttime.org", Name: "时间", BaseURL: "https://www.pttime.org",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category: []SelectOption{{"401", "Movies(电影)"}, {"402", "TV Series(连续剧)"}, {"403", "TV Shows(综艺)"}, {"404", "Documentaries(纪录片)"}, {"405", "Sport(体育)"}, {"406", "ACG(动漫)"}, {"407", "Baby(婴幼儿童)"}, {"408", "Music(音乐)"}, {"409", "Art(曲艺)"}, {"411", "Knowledge(知识)"}, {"412", "School(应试)"}, {"420", "Code(编程)"}, {"421", "Games(游戏)"}, {"422", "Software(软件)"}, {"423", "Resource(素材)"}, {"430", "Other(其它)"}},
		},
	},
	{
		Domain: "www.momentpt.top", Name: "瞬间", BaseURL: "https://www.momentpt.top",
		Framework: "nexusphp", IsSource: false, IsTarget: false,
		Form: SiteFormConfig{
			Category: []SelectOption{{"420", "软件"}, {"419", "图书"}, {"418", "预设"}, {"417", "教程"}},
		},
	},
	{
		Domain: "hdcity.city", Name: "城市", BaseURL: "https://hdcity.city",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		AlternativeDomains: "hdcity.leniter.org,hdcity.work",
		Form: SiteFormConfig{
			Category:      []SelectOption{{"401", "Movies/电影"}, {"402", "Series/剧集"}, {"404", "Doc/档案记录"}, {"405", "Anim/动漫"}, {"403", "Shows/节目"}, {"406", "MV/音乐视频"}, {"407", "Sports/体育"}, {"408", "Audio/音频"}, {"728", "Edu/文档/教材"}, {"729", "Soft/软件"}, {"409", "Other/其他"}},
			MediumSel:     []SelectOption{{"1", "BD/蓝光原盘"}, {"2", "HDDVD原盘"}, {"3", "Remux/重混流"}, {"7", "Encode/重编码"}, {"4", "MiniBD/微蓝光"}, {"5", "HDTV/SNG/原始录制"}, {"6", "DVD原盘"}, {"8", "CD/音乐/有声读物"}, {"9", "Track/外挂音轨"}, {"10", "Ebook/文档/图库"}, {"11", "Rec/视频教材"}, {"12", "Joy/游戏"}, {"13", "Prog/程序"}},
			CodecSel:      []SelectOption{{"1", "H.264/AVC"}, {"13", "H.265/HEVC"}, {"4", "MPEG-2"}, {"3", "DivX/XviD"}, {"2", "WMV/VC-1"}, {"16", "AV1"}, {"17", "WebM/VP"}, {"14", "WMA/WMA-LL"}, {"5", "FLAC"}, {"6", "APE"}, {"7", "DTS/DTS-ES"}, {"8", "Dolby AC3"}, {"15", "TrueHD/Atmos"}, {"10", "WAV/Raw"}, {"11", "MP3/MP2"}, {"12", "AAC/M4A"}, {"9", "Other"}},
			StandardSel:   []SelectOption{{"11", "8K-4320p"}, {"10", "8K-4320i"}, {"9", "4K-2160p"}, {"8", "4K-2160i"}, {"1", "1080p"}, {"2", "1080i"}, {"3", "720p"}, {"7", "720i"}, {"6", "540p"}, {"5", "480p"}, {"4", "SD"}},
			ProcessingSel: []SelectOption{{"1", "3D H-OU/上下半宽"}, {"2", "3D H-SBS/左右半宽"}, {"3", "3D Interleaved/交织"}, {"4", "3D Red-blue/红蓝"}, {"5", "3D Alt/其他3D"}},
			TeamSel:       []SelectOption{{"1", "HDCITY-NoVA"}, {"14", "HDCITY-NoPA"}, {"15", "HDCITY-NoTA"}, {"17", "HDCITY-NoXA"}, {"9", "0DAY"}},
		},
	},
	{
		Domain: "pt.keepfrds.com", Name: "朋友", BaseURL: "https://pt.keepfrds.com",
		Framework: "nexusphp", IsSource: true, IsTarget: false,
	},
	{
		Domain: "pt.ptskit.org", Name: "拾刻", BaseURL: "https://pt.ptskit.org",
		Framework: "nexusphp", IsSource: true, IsTarget: false,
	},
	{
		Domain: "ourbits.club", Name: "我堡", BaseURL: "https://ourbits.club",
		Framework: "nexusphp", IsSource: true, IsTarget: false,
		Form: SiteFormConfig{
			Category: []SelectOption{
				{"401", "Movies/电影"}, {"402", "Movies-3D/3D电影"},
				{"419", "Concert/演唱会"}, {"412", "TV-Episode/电视剧单集"},
				{"405", "TV-Pack/电视剧包"}, {"413", "TV-Show/综艺节目"},
				{"410", "Documentary/纪录片"}, {"411", "Animation/动漫"},
				{"415", "Sports/体育"}, {"414", "Music-Video/MV"}, {"416", "Music/音乐"},
			},
			MediumSel: []SelectOption{
				{"12", "UHD Blu-ray"}, {"1", "FHD Blu-ray"}, {"7", "Encode"},
				{"9", "WEB-DL"}, {"5", "HDTV"}, {"13", "UHDTV"},
				{"2", "DVD"}, {"8", "CD"},
			},
			CodecSel: []SelectOption{
				{"12", "H.264"}, {"14", "HEVC"}, {"15", "MPEG-2"},
				{"16", "VC-1"}, {"17", "Xvid"}, {"19", "AV1"}, {"18", "Other"},
			},
			StandardSel: []SelectOption{
				{"1", "1080p"}, {"2", "1080i"}, {"3", "720p"},
				{"4", "SD"}, {"5", "2160p"},
			},
			AudioCodec: []SelectOption{
				{"14", "Atmos"}, {"21", "DTS X"}, {"1", "DTS-HDMA"},
				{"2", "TrueHD"}, {"4", "DTS"}, {"5", "LPCM"}, {"13", "FLAC"},
				{"12", "APE"}, {"7", "AAC"}, {"6", "AC3"}, {"11", "WAV"},
				{"32", "MPEG"}, {"33", "OPUS"},
			},
			ProcessingSel: []SelectOption{
				{"1", "CN/中国大陆"}, {"2", "US/EU/欧美"}, {"3", "HK/TW/港台"},
				{"4", "JP/日"}, {"5", "KR/韩"}, {"6", "OT/其他"},
			},
			TeamSel: []SelectOption{
				{"41", "原创/原抓"},
			},
			Tags: []SelectOption{
				{"sf", "首发"}, {"diy", "DIY"}, {"gy", "国语"}, {"zz", "中字"},
				{"yq", "应求"}, {"jz", "禁转"}, {"db", "杜比视界"},
				{"hdrvivid", "菁彩HDR"}, {"hdr", "HDR10"}, {"hdrp", "HDR10+"},
				{"hlg", "HLG"},
			},
		},
	},
	{
		Domain: "sunnypt.top", Name: "阳光", BaseURL: "https://sunnypt.top",
		Framework: "nexusphp", IsSource: true, IsTarget: false,
	},
	{
		Domain: "xingwan.cc", Name: "星湾", BaseURL: "https://xingwan.cc",
		Framework: "nexusphp", IsSource: true, IsTarget: false,
	},
	{
		Domain: "xingtan.one", Name: "杏坛", BaseURL: "https://xingtan.one",
		Framework: "nexusphp", IsSource: false, IsTarget: true,
		Form: SiteFormConfig{
			Category: []SelectOption{
				{"16", "综合资源"}, {"31", "哲学"}, {"34", "经济学"},
				{"35", "法学"}, {"36", "政治学"}, {"37", "社会学"},
				{"38", "马克思主义"}, {"39", "教育学"}, {"40", "心理学"},
				{"41", "体育学"}, {"42", "语言学"}, {"43", "中国文学"},
				{"44", "外国文学"}, {"45", "新闻传播学"}, {"46", "艺术学"},
				{"47", "历史学"}, {"48", "数学"}, {"49", "物理学"},
				{"50", "化学"}, {"51", "天文学"}, {"52", "地球科学"},
				{"53", "生物学"}, {"54", "机械工程"}, {"55", "计算机科学与技术"},
				{"56", "环境科学与工程"}, {"58", "基础医学"}, {"59", "临床医学"},
				{"61", "口腔医学"}, {"62", "公共卫生与预防医学"}, {"63", "中医学"},
				{"65", "药学"}, {"66", "中药学"}, {"67", "管理学"},
				{"68", "工商管理"}, {"69", "农林经济管理"}, {"70", "公共管理"},
				{"71", "图书情报与档案管理"}, {"72", "理论经济学"},
				{"73", "应用经济学"}, {"74", "电子科学与技术"},
				{"75", "信息与通信工程"}, {"76", "控制科学与工程"},
				{"77", "土木工程"}, {"78", "化学工程与技术"},
				{"79", "材料科学与工程"}, {"80", "力学"}, {"81", "冶金工程"},
				{"82", "电气工程"}, {"83", "动力工程及工程热物理"},
				{"84", "核科学与技术"}, {"85", "水利工程"},
				{"87", "测绘科学与技术"}, {"88", "地质资源与地质工程"},
				{"89", "矿业工程"}, {"91", "船舶与海洋工程"},
				{"93", "航空宇航科学与技术"}, {"94", "兵器科学与技术"},
				{"95", "食品科学与工程"}, {"96", "城乡规划学"},
				{"97", "风景园林学"}, {"98", "建筑学"}, {"99", "安全科学与工程"},
				{"100", "生物医学工程"}, {"101", "系统科学"},
				{"102", "科学技术史"}, {"103", "护理学"}, {"104", "林学"},
				{"105", "水产"}, {"106", "作物学"}, {"107", "畜牧学"},
				{"108", "兽医学"}, {"109", "草学"}, {"110", "生态学"},
				{"111", "民族学"}, {"112", "统计学"}, {"113", "军事思想及军事历史"},
				{"116", "军事战略"}, {"118", "战术指挥"}, {"119", "后勤装备"},
				{"120", "公安学"}, {"121", "军事制度"}, {"122", "军事其他"},
			},
			SourceSel: []SelectOption{
				{"1", "视频"}, {"2", "音频"}, {"3", "书籍"}, {"4", "文档"},
				{"5", "笔记"}, {"7", "其它"}, {"8", "软件"}, {"10", "课件"},
				{"13", "会议"}, {"14", "图片"},
			},
			ProcessingSel: []SelectOption{
				{"2", "RMVB"}, {"3", "MP4"}, {"16", "MKV"}, {"5", "JPG"},
				{"6", "PDF"}, {"7", "TXT"}, {"8", "DOC"}, {"9", "XLS"},
				{"10", "PPT"}, {"11", "WMA"}, {"12", "MP3"}, {"13", "RAR"},
				{"14", "EXE"}, {"15", "其它"}, {"17", "EPUB"}, {"18", "M4A"},
				{"19", "AZW3"}, {"20", "DJVU"}, {"21", "CAJ"}, {"22", "MOBI"},
				{"23", "TIFF"}, {"24", "PSD"}, {"25", "APE"}, {"26", "FLAC"},
				{"27", "ZIP"}, {"28", "UVZ"},
			},
			Tags: []SelectOption{
				{"1", "付费"}, {"2", "首发"}, {"3", "官方"}, {"6", "中字"},
				{"8", "英文"}, {"9", "确认"}, {"10", "英语"}, {"12", "修正"},
				{"13", "原创"}, {"19", "应求"}, {"39", "繁体"},
			},
		},
	},
	{
		Domain: "zeus.hamsters.space", Name: "蝴蝶", BaseURL: "https://zeus.hamsters.space",
		Framework: "nexusphp", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category: []SelectOption{
				{"401", "大陆电影"}, {"413", "港台电影"}, {"414", "亚洲电影"},
				{"415", "欧美电影"}, {"430", "iPad"}, {"433", "抢先视频"},
				{"402", "大陆剧集"}, {"417", "港台剧集"}, {"416", "亚洲剧集"},
				{"418", "欧美剧集"}, {"404", "纪录片"}, {"407", "体育"},
				{"403", "大陆综艺"}, {"419", "港台综艺"}, {"420", "亚洲综艺"},
				{"421", "欧美综艺"}, {"408", "华语音乐"}, {"422", "日韩音乐"},
				{"423", "欧美音乐"}, {"424", "古典音乐"}, {"425", "原声音乐"},
				{"406", "音乐MV"}, {"409", "其他"}, {"432", "电子书"},
				{"405", "完结动漫"}, {"427", "连载动漫"}, {"428", "剧场OVA"},
				{"429", "动漫周边"}, {"410", "游戏"}, {"431", "游戏视频"},
				{"411", "软件"}, {"412", "学习"}, {"426", "MAC"}, {"1037", "HUST"},
			},
			StandardSel: []SelectOption{
				{"1", "1080p"}, {"2", "1080i"}, {"3", "720p"},
				{"4", "SD"}, {"6", "Lossy"}, {"7", "2160p/4K"}, {"5", "Lossless"},
			},
		},
	},
	{
		Domain: "pt.tu88.men", Name: "图88", BaseURL: "https://pt.tu88.men",
		Framework: "nexusphp", IsSource: true, IsTarget: false,
		Form: SiteFormConfig{
			Category: []SelectOption{
				{"412", "漫畫"}, {"413", "繪本"}, {"414", "其他"}, {"417", "圖集"},
			},
			Tags: []SelectOption{
				{"39", "480P"}, {"40", "720P"}, {"41", "1080P"}, {"42", "2K"},
				{"43", "4K"}, {"44", "5K"}, {"45", "8K"},
				{"47", "官方"}, {"48", "禁轉"}, {"49", "首發"}, {"50", "大包"},
				{"51", "全彩"}, {"71", "推薦"},
				{"52", "日系"}, {"53", "欧美系"}, {"54", "港台系"}, {"55", "韓系"},
				{"56", "中文"}, {"57", "日文"}, {"58", "英文"}, {"59", "韓文"},
				{"60", "2D"}, {"61", "3D"}, {"62", "真人"}, {"63", "雜誌"},
				{"64", "連環畫"}, {"65", "同人"}, {"73", "單輯"},
				{"66", "單行本"}, {"67", "連載"}, {"68", "短篇"}, {"69", "完結"},
				{"70", "恐怖"}, {"72", "臨時"},
			},
		},
	},
	{
		Domain: "www.tokyo-manga.top", Name: "东京", BaseURL: "https://www.tokyo-manga.top",
		Framework: "nexusphp", IsSource: true, IsTarget: false,
		AlternativeDomains: `["www.tokyopt.xyz"]`,
	},
	{
		Domain: "www.yemapt.org", Name: "野马", BaseURL: "https://www.yemapt.org",
		Framework: "generic", IsSource: true, IsTarget: true,
		CookieCloudDomain: "yemapt.org",
		Form: SiteFormConfig{
			Category: []SelectOption{
				{"4", "电影"}, {"5", "剧集"}, {"13", "综艺"}, {"14", "动漫"},
				{"15", "纪录片"}, {"17", "体育"}, {"6", "短剧"}, {"16", "MV/演唱会"},
				{"3", "软件"}, {"10", "游戏"}, {"12", "书籍"}, {"22", "其他"},
				{"8", "音乐"}, {"9", "广播剧"},
				{"19", "教育书籍"}, {"20", "教育音频"}, {"21", "教育视频"},
			},
			MediumSel: []SelectOption{
				{"Web-dl", "WEB-DL"}, {"Blu-ray", "Blu-ray"}, {"Blu-rayUHD", "UHD Blu-ray"},
				{"Remux", "Remux"}, {"Encode", "Encode"}, {"HDTV/TV", "HDTV"},
				{"DVDrip", "DVDRip"}, {"CD", "CD"}, {"DVD", "DVD"}, {"Other", "Other"},
			},
			StandardSel: []SelectOption{
				{"720i", "720i"}, {"720p", "720p"}, {"1080i", "1080i"}, {"1080p", "1080p"},
				{"SD", "SD"}, {"1440p/2K", "2K"}, {"2160p/4K", "4K"}, {"8K", "8K"},
				{"Other", "Other"},
			},
			CodecSel: []SelectOption{
				{"H.264(x264/AVC)", "H.264"}, {"H.265(x265/HEVC)", "HEVC"},
				{"Bluray(VC1)", "VC-1"}, {"Bluray(AVC)", "Blu-ray AVC"},
				{"Bluray(HEVC)", "Blu-ray HEVC"}, {"MPEG-2", "MPEG-2"},
				{"Xvid", "Xvid"}, {"AV1", "AV1"}, {"H.266/VVC", "VVC"},
				{"Other", "Other"},
			},
			AudioCodec: []SelectOption{
				{"AAC", "AAC"}, {"AC3", "AC3"}, {"DTS", "DTS"}, {"DTS-HD MA", "DTS-HD MA"},
				{"E-AC3(DDP)", "DDP"}, {"E-AC3 Atoms", "E-AC3 Atmos"},
				{"TrueHD", "TrueHD"}, {"TrueHD Atoms", "TrueHD Atmos"},
				{"LPCM", "LPCM"}, {"FLAC", "FLAC"}, {"APE", "APE"},
				{"MP3", "MP3"}, {"OGG", "OGG"}, {"Opus", "Opus"},
				{"Other", "Other"},
			},
			ProcessingSel: []SelectOption{
				{"CN(中国)", "中国大陆"}, {"HK/CN(香港)", "香港"}, {"TW/CN(台湾)", "台湾"},
				{"US(美国)", "美国"}, {"EU(欧洲)", "欧洲"}, {"JP(日本)", "日本"},
				{"KR(韩国)", "韩国"}, {"Other", "其他"},
			},
			TeamSel: []SelectOption{
				{"OurBits", "OurBits"}, {"BtsHD", "BtsHD"}, {"BtsTV", "BtsTV"},
				{"HDChina", "HDChina"}, {"CMCT", "CMCT"}, {"HHWEB", "HHWEB"},
				{"FRDS", "FRDS"}, {"MTeam", "MTeam"}, {"QHstudio", "QHstudio"},
				{"UBits", "UBits"}, {"Other", "Other"},
			},
			Tags: []SelectOption{
				{"禁转", "禁转"}, {"首发", "首发"}, {"官方", "官方"}, {"自制", "自制"},
				{"国语", "国语"}, {"中字", "中字"}, {"粤语", "粤语"}, {"英字", "英字"},
				{"HDR10", "HDR10"}, {"杜比视界", "杜比视界"}, {"分集", "分集"}, {"完结", "完结"},
			},
		},
	},
	{
		Domain: "greatposterwall.com", Name: "海豹", BaseURL: "https://greatposterwall.com",
		Framework: "gazelle", AuthType: "cookie", IsSource: true, IsTarget: true,
		Form: SiteFormConfig{
			Category:      []SelectOption{{"0", "Movies"}},
			SourceSel:     []SelectOption{{"VHS", "VHS"}, {"DVD", "DVD"}, {"HD-DVD", "HD-DVD"}, {"TV", "TV"}, {"HDTV", "HDTV"}, {"WEB", "WEB"}, {"Blu-ray", "Blu-ray"}, {"Other", "Other"}},
			CodecSel:      []SelectOption{{"DivX", "DivX"}, {"XviD", "XviD"}, {"x264", "x264"}, {"H.264", "H.264"}, {"x265", "x265"}, {"H.265", "H.265"}, {"Other", "Other"}},
			StandardSel:   []SelectOption{{"Other", "Other"}, {"NTSC", "NTSC"}, {"PAL", "PAL"}, {"480p", "480p"}, {"576p", "576p"}, {"720p", "720p"}, {"1080i", "1080i"}, {"1080p", "1080p"}, {"2160p", "2160p"}},
			ProcessingSel: []SelectOption{{"---", "---"}, {"Encode", "Encode"}, {"Remux", "Remux"}, {"DIY", "DIY"}, {"Untouched", "Untouched"}},
			Tags:          []SelectOption{{"动作", "动作"}, {"冒险", "冒险"}, {"动画", "动画"}, {"艺术", "艺术"}, {"亚洲", "亚洲"}, {"传记", "传记"}, {"喜剧", "喜剧"}, {"犯罪", "犯罪"}, {"邪典", "邪典"}, {"纪录片", "纪录片"}, {"剧情", "剧情"}, {"实验", "实验"}, {"家庭", "家庭"}, {"奇幻", "奇幻"}, {"黑色电影", "黑色电影"}, {"历史", "历史"}, {"恐怖", "恐怖"}, {"lgbt", "lgbt"}, {"武侠", "武侠"}, {"音乐", "音乐"}, {"音乐剧", "音乐剧"}, {"悬疑", "悬疑"}, {"演出", "演出"}, {"政治", "政治"}, {"爱情", "爱情"}, {"科幻", "科幻"}, {"短片", "短片"}, {"默片", "默片"}, {"体育", "体育"}, {"惊悚", "惊悚"}, {"video.art", "video.art"}, {"战争", "战争"}, {"西部", "西部"}},
		},
	},
}

func SeedSites(db *gorm.DB) error {
	for _, s := range seedSites {
		var existing model.Site
		err := db.Where("domain = ?", s.Domain).First(&existing).Error
		if err == nil {
			continue
		}

		defs, ok := adapter.FrameworkDefaults[s.Framework]
		if !ok {
			defs = adapter.FrameworkDefaults["generic"]
		}

		site := &model.Site{
			Domain:             s.Domain,
			Name:               s.Name,
			BaseURL:            s.BaseURL,
			Framework:          s.Framework,
			AuthType:           defaultAuthType(s.AuthType),
			Enabled:            true,
			IsSource:           s.IsSource,
			IsTarget:           s.IsTarget,
			CookieCloudDomain:  s.CookieCloudDomain,
			AlternativeDomains: s.AlternativeDomains,

			HashStrategy:        defs.HashStrategy,
			SizeStrategy:        defs.SizeStrategy,
			IDStrategy:          defs.IDStrategy,
			IDPattern:           defs.IDPattern,
			DownloadMode:        "template",
			DownloadURLTemplate: defs.DownloadURLTemplate,
			RequiresSideLoading: defs.RequiresSideLoading,
		}

		if err := db.Create(site).Error; err != nil {
			return fmt.Errorf("create site %s: %w", s.Domain, err)
		}
	}

	return nil
}

func defaultAuthType(s string) string {
	if s != "" {
		return s
	}
	return "cookie"
}

func SeedFieldMappings(db *gorm.DB) error {
	for _, s := range seedSites {
		fieldTypes := map[string][]SelectOption{
			"cat":            s.Form.Category,
			"medium_sel":     s.Form.MediumSel,
			"codec_sel":      s.Form.CodecSel,
			"standard_sel":   s.Form.StandardSel,
			"audiocodec_sel": s.Form.AudioCodec,
			"team_sel":       s.Form.TeamSel,
			"processing_sel": s.Form.ProcessingSel,
			"source_sel":     s.Form.SourceSel,
			"tags":           s.Form.Tags,
		}

		for fieldType, options := range fieldTypes {
			for _, opt := range options {
				if opt.Value == "" || opt.Label == "" {
					continue
				}

				var existing model.SiteFieldMapping
				err := db.Where("site_name = ? AND field_type = ? AND source_value = ?",
					s.Name, fieldType, opt.Label).First(&existing).Error
				if err == nil {
					continue
				}

				mapping := &model.SiteFieldMapping{
					SiteName:    s.Name,
					FieldType:   fieldType,
					SourceValue: opt.Label,
					TargetValue: opt.Value,
				}
				if err := db.Create(mapping).Error; err != nil {
					return fmt.Errorf("create mapping %s/%s/%s: %w", s.Name, fieldType, opt.Label, err)
				}
			}
		}
	}

	return nil
}
