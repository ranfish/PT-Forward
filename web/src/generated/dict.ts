// 本文件由 cmd/gen-dict 自动生成（§59.35 P3），禁止手改。
// 数据源：internal/titleparser/dict/*.json（唯一真相源）。
// 重新生成：go run ./cmd/gen-dict（CI drift 检查强制同步）。

// type 域：standard_key → 显示名（categoryLabel 数据源）
export const CATEGORY_LABELS: Record<string, string> = {
  "category.movie": "电影",
  "category.tv_series": "电视剧",
  "category.tv_shows": "综艺",
  "category.animation": "动漫",
  "category.documentary": "纪录片",
  "category.music": "音乐",
  "category.sports": "体育",
  "category.other": "其他",
}

// medium 域：standard_key → 显示名
export const MEDIUM_LABELS: Record<string, string> = {
  "medium.bluray": "Blu-ray",
  "medium.uhd_bluray": "UHD Blu-ray",
  "medium.bluray_3d": "3D Blu-ray",
  "medium.remux": "Remux",
  "medium.uhd_remux": "UHD Blu-ray Remux",
  "medium.encode": "Encode",
  "medium.webdl": "WEB-DL",
  "medium.webrip": "WEBRip",
  "medium.hdtv": "HDTV",
  "medium.uhdtv": "UHDTV",
  "medium.dvdrip": "DVDRip",
  "medium.bdrip": "BDRip",
  "medium.tvrip": "TVRip",
  "medium.dvd": "DVD",
}

// hdr 域：standard_key → 显示名
export const HDR_LABELS: Record<string, string> = {
  "hdr.dv": "DoVi",
  "hdr.dv_hdr": "DoVi HDR",
  "hdr.dv_hdr10plus": "DoVi HDR10+",
  "hdr.hdr10plus": "HDR10+",
  "hdr.hdr10": "HDR10",
  "hdr.hdr_vivid": "HDR Vivid",
  "hdr.hlg": "HLG",
  "hdr.hdr": "HDR",
  "hdr.sdr": "SDR",
}

// video_codec 域：standard_key → 显示名
export const VIDEO_CODEC_LABELS: Record<string, string> = {
  "video.av1": "AV1",
  "video.vp9": "VP9",
  "video.avs2": "AVS2",
  "video.h266": "VVC",
  "video.xvid": "Xvid",
  "video.x265": "x265",
  "video.h265": "HEVC",
  "video.x264": "x264",
  "video.h264": "H.264",
  "video.vc1": "VC-1",
  "video.mpeg2": "MPEG-2",
}

// audio_codec 域：standard_key → 显示名
export const AUDIO_CODEC_LABELS: Record<string, string> = {
  "audio.truehd": "TrueHD",
  "audio.dts_x": "DTS:X",
  "audio.dts_hd_ma": "DTS-HD MA",
  "audio.dts_hd_hr": "DTS-HD HR",
  "audio.dts": "DTS",
  "audio.ddp": "DDP",
  "audio.dd": "DD",
  "audio.flac": "FLAC",
  "audio.aac": "AAC",
  "audio.alac": "ALAC",
  "audio.ape": "APE",
  "audio.wav": "WAV",
  "audio.opus": "Opus",
  "audio.mp3": "MP3",
  "audio.lpcm": "LPCM",
  "audio.dsd": "DSD",
}

// resolution 域：standard_key → 显示名
export const RESOLUTION_LABELS: Record<string, string> = {
  "resolution.r4320p": "4320p",
  "resolution.r2160p": "2160p",
  "resolution.r1440p": "1440p",
  "resolution.r1080p": "1080p",
  "resolution.r1080i": "1080i",
  "resolution.r720p": "720p",
  "resolution.r480p": "480p",
}

// source 域：standard_key → 显示名
export const SOURCE_LABELS: Record<string, string> = {
  "source.china": "中国大陆",
  "source.hongkong": "中国香港",
  "source.taiwan": "中国台湾",
  "source.western": "美国",
  "source.uk": "英国",
  "source.japan": "日本",
  "source.korea": "韩国",
  "source.other": "其他",
}

// platform 域：canonical → 厂商全名（Tab1 分发方 tooltip）
export const PLATFORM_FULLNAMES: Record<string, string> = {
  "9NOW": "9Now",
  "AE": "A&E",
  "AJAZ": "Al Jazeera English",
  "ALL4": "All4 (Channel 4, ex-4oD)",
  "AMBC": "ABC (US)",
  "AMC": "AMC",
  "AMC+": "AMC+",
  "AMZN": "Amazon, Prime Video",
  "ANLB": "AnimeLab",
  "ANPL": "Animal Planet",
  "AOL": "AOL",
  "ARD": "ARD",
  "AS": "Adult Swim",
  "ATK": "America's Test Kitchen",
  "ATVP": "Apple TV+",
  "AUBC": "ABC (AU) iView",
  "Amazon": "Amazon",
  "BFI": "BFI Player",
  "BKPL": "Blackpills",
  "BNGE": "Binge",
  "BOOM": "Boomerang",
  "BRAV": "BravoTV",
  "Baha": "巴哈姆特動畫瘋",
  "CAN": "CAN",
  "CATCHPLAY": "CATCHPLAY",
  "CBC": "CBC",
  "CBS": "CBS",
  "CC": "Comedy Central",
  "CCGC": "Comedians in Cars Getting Coffee",
  "CEE": "CEE",
  "CHGD": "CHRGD",
  "CHN": "CHN",
  "CLBI": "Club illico",
  "CMAX": "Cinemax",
  "CMOR": "C More",
  "CMT": "Country Music Television",
  "CN": "Cartoon Network",
  "CNBC": "CNBC",
  "CNLP": "Canal+",
  "COOK": "Cooking Channel",
  "CR": "Crunchy Roll",
  "CRAV": "Crave",
  "CRKL": "Crackle",
  "CSPN": "CSpan",
  "CTV": "CTV",
  "CUR": "CuriosityStream",
  "CW": "The CW",
  "CWS": "CWSeed",
  "Crunchyroll": "Crunchyroll",
  "DCU": "DC Universe",
  "DDY": "Digiturk Dilediğin Yerde",
  "DEST": "Destination America",
  "DF": "DramaFever",
  "DHF": "Deadhouse Films",
  "DISC": "Discovery Channel",
  "DIY": "DIY Network",
  "DMM": "DMM",
  "DNSP": "DNSP",
  "DOCC": "Doc Club",
  "DPLY": "DPlay (Rebranded as Discovery+)",
  "DRPO": "Dropout",
  "DSCP": "Discovery+",
  "DSKI": "Daisuki",
  "DSNP": "Disney+",
  "DSNY": "Disney",
  "DTV": "DirecTV Now",
  "Disney": "Disney",
  "Disney+": "Disney+",
  "EPIX": "EPIX",
  "ESP": "ESP",
  "ESPN": "ESPN",
  "ESQ": "Esquire",
  "ETTV": "El Trece",
  "ETV": "E!",
  "EUR": "EUR",
  "FAM": "Family",
  "FJR": "Family Jr",
  "FOOD": "Food Network",
  "FOX": "Fox",
  "FPT": "FPT Play",
  "FRA": "FRA",
  "FREE": "Freeform",
  "FTV": "France.tv",
  "FUNI": "Funimation",
  "FUNi": "FUNimation",
  "FXTL": "Foxtel Now",
  "FYI": "FYI Network",
  "GBR": "GBR",
  "GC": "NHL GameCenter",
  "GER": "GER",
  "GLBL": "Global",
  "GLOB": "GloboSat Play",
  "GO90": "go90",
  "GYAO": "GYAO!",
  "HBO": "HBO",
  "HGTV": "HGTV",
  "HIDI": "HIDIVE",
  "HIST": "History Channel",
  "HKG": "HKG",
  "HLMK": "Hallmark",
  "HMAX": "HBO Max",
  "HTSR": "Hotstar",
  "HULU": "Hulu",
  "Hami": "Hami Video",
  "ID": "Investigation Discovery",
  "IFC": "IFC",
  "IQ": "IQ",
  "ITA": "ITA",
  "ITV": "ITV",
  "JPN": "JPN",
  "KAYO": "Kayo Sports",
  "KKTV": "KKTV",
  "KNOW": "Knowledge Network",
  "KNPY": "Kanopy",
  "LIFE": "Lifetime",
  "LINETV": "LINETV",
  "LN": "Loving Nature",
  "LiTV": "LiTV",
  "MA": "MA",
  "MAX": "Max（原 HBO Max 更名）",
  "MBC": "MBC",
  "MNBC": "MSNBC",
  "MTOD": "Motor Trend OnDemand",
  "MTV": "MTV",
  "MUBI": "MUBI",
  "MY5": "Channel 5",
  "MyTVS": "MyTVS",
  "MyTVSuper": "MyTVSuper",
  "MyVideo": "MyVideo",
  "NATG": "National Geographic",
  "NBA": "NBA League Pass",
  "NBC": "NBC",
  "NF": "Netflix",
  "NFL": "NFL Network",
  "NFLN": "NFL Now",
  "NICK": "Nickelodeon",
  "NOW": "Now (Sky)",
  "NRK": "Norsk Rikskringkasting",
  "Netflix": "Netflix",
  "NowE": "NowE",
  "NowPlay": "NowPlay",
  "NowPlayer": "NowPlayer",
  "ODK": "OnDemandKorea",
  "OXGN": "Oxygen",
  "PA": "Project Alpha",
  "PBS": "PBS",
  "PBSK": "PBS Kids",
  "PCOK": "Peacock",
  "PLAY": "Google Play",
  "PLUZ": "Pluzz",
  "PMNT": "Paramount Network",
  "PMTP": "Paramount+",
  "POGO": "PokerGo",
  "PSN": "Playstation Network",
  "PUHU": "puhutv",
  "QIBI": "Quibi",
  "RED": "YouTube Red",
  "RKTN": "Rakuten TV",
  "ROKU": "The Roku Channel",
  "RSTR": "Rooster Teeth",
  "RTE": "RTÉ",
  "SBS": "SBS (AU)",
  "SEEZN": "SEEZN",
  "SESO": "Seeso",
  "SHDR": "Shudder",
  "SHMI": "Shomi",
  "SHO": "Showtime",
  "SNET": "Sportsnet",
  "SPIK": "Spike",
  "SPRT": "Sprout",
  "STAN": "Stan",
  "STAR": "Disney+ Hotstar",
  "STRP": "Star+",
  "STZ": "Starz",
  "SVT": "Sveriges Television",
  "SWER": "SwearNet",
  "SYFY": "SyFy",
  "Sentai": "Sentai Filmworks",
  "TBS": "TBS",
  "TEN": "TenPlay",
  "TFOU": "TFOU",
  "TIMV": "TIMvision",
  "TLC": "TLC",
  "TOU": "Ici TOU.TV",
  "TRVL": "Travel Channel",
  "TUBI": "TubiTV",
  "TV3": "TV3 (IE)",
  "TV4": "TV4 (SE)",
  "TVB": "TVB",
  "TVBAnywhere": "TVBAnywhere",
  "TVING": "TVING",
  "TVL": "TVLand",
  "TWN": "台湾",
  "TX": "TX",
  "U-NEXT": "U-NEXT",
  "UFC": "UFC",
  "UKTV": "UKTV",
  "UNIV": "Univision",
  "USA": "USA",
  "USAN": "USA Network",
  "VH1": "VH1",
  "VIAP": "Viaplay",
  "VICE": "Viceland",
  "VIKI": "Viki",
  "VLCT": "Velocity",
  "VMEO": "Vimeo",
  "VRV": "VRV",
  "Viu": "Viu",
  "ViuTV": "ViuTV",
  "WME": "WatchMe",
  "WNET": "W Network",
  "WWEN": "WWE Network",
  "WeTV": "WeTV",
  "XBOX": "Xbox Video",
  "YHOO": "Yahoo",
  "YOUKU": "YOUKU",
  "YT": "YouTube Movies",
  "ZDF": "ZDF",
  "friDay": "friDay",
  "iP": "BBC iPlayer",
  "iPad": "iPad",
  "iQIYI": "iQIYI",
  "iT": "iTunes",
  "meWATCH": "meWATCH",
}

// tag 域分组结构（TagSelector 数据源，派生自 dict/tag.json group 字段）
export interface TagDef { key: string; label: string; aliases: string }
export interface TagGroup { name: string; tags: TagDef[] }
export const TAG_GROUPS: TagGroup[] = [
    {
      "name": "HDR/色彩",
      "tags": [
        {
          "key": "dolby_vision",
          "label": "Dolby Vision",
          "aliases": "DV/DoVi"
        },
        {
          "key": "hdr10_plus",
          "label": "HDR10+",
          "aliases": ""
        },
        {
          "key": "hdr10",
          "label": "HDR10",
          "aliases": "HDR 10"
        },
        {
          "key": "hdr_vivid",
          "label": "HDR Vivid",
          "aliases": "国标 HDR"
        },
        {
          "key": "hlg",
          "label": "HLG",
          "aliases": ""
        },
        {
          "key": "10_bit",
          "label": "10bit",
          "aliases": "10-bit 色深"
        }
      ]
    },
    {
      "name": "音频编码",
      "tags": [
        {
          "key": "dolby_atmos",
          "label": "Dolby Atmos",
          "aliases": "全景声"
        },
        {
          "key": "auro_3d",
          "label": "Auro3D",
          "aliases": "Auro-3D"
        },
        {
          "key": "dts_x",
          "label": "DTS:X",
          "aliases": ""
        },
        {
          "key": "lossless",
          "label": "无损",
          "aliases": ""
        },
        {
          "key": "lossy",
          "label": "有损",
          "aliases": ""
        }
      ]
    },
    {
      "name": "语言音轨",
      "tags": [
        {
          "key": "chinese_audio",
          "label": "国语",
          "aliases": "普通话/国配"
        },
        {
          "key": "cantonese_audio",
          "label": "粤语",
          "aliases": ""
        },
        {
          "key": "japanese_audio",
          "label": "日语",
          "aliases": "原声"
        },
        {
          "key": "korean_audio",
          "label": "韩语",
          "aliases": ""
        },
        {
          "key": "original_audio",
          "label": "原声",
          "aliases": ""
        },
        {
          "key": "dubbed",
          "label": "配音",
          "aliases": "Dub"
        }
      ]
    },
    {
      "name": "字幕",
      "tags": [
        {
          "key": "chinese_subtitle",
          "label": "中文字幕",
          "aliases": "CHS/简繁"
        },
        {
          "key": "english_subtitle",
          "label": "英文字幕",
          "aliases": "ENG"
        },
        {
          "key": "hardcoded_subs",
          "label": "硬字幕",
          "aliases": "硬字"
        },
        {
          "key": "encoded_subs",
          "label": "内嵌字幕",
          "aliases": ""
        },
        {
          "key": "external_subtitles",
          "label": "外挂字幕",
          "aliases": ""
        },
        {
          "key": "subtitles_include",
          "label": "含字幕",
          "aliases": ""
        }
      ]
    },
    {
      "name": "版本类型",
      "tags": [
        {
          "key": "diy",
          "label": "DIY",
          "aliases": ""
        },
        {
          "key": "scene",
          "label": "Scene",
          "aliases": "Scene Release"
        },
        {
          "key": "remux",
          "label": "Remux",
          "aliases": ""
        },
        {
          "key": "internal",
          "label": "Internal",
          "aliases": "iNTERNAL"
        },
        {
          "key": "exclusive",
          "label": "独占",
          "aliases": "专属"
        },
        {
          "key": "retail",
          "label": "Retail",
          "aliases": ""
        },
        {
          "key": "web_release",
          "label": "WEB Release",
          "aliases": ""
        },
        {
          "key": "promotional",
          "label": "宣传版",
          "aliases": "Promo"
        },
        {
          "key": "hybrid",
          "label": "Hybrid",
          "aliases": ""
        }
      ]
    },
    {
      "name": "特别版",
      "tags": [
        {
          "key": "special_edition",
          "label": "特别版",
          "aliases": "Special Edition"
        },
        {
          "key": "director_s_cut",
          "label": "导演剪辑",
          "aliases": "Director's Cut"
        },
        {
          "key": "anniversary_edition",
          "label": "纪念版",
          "aliases": "Anniversary"
        },
        {
          "key": "criterion",
          "label": "Criterion",
          "aliases": "CC"
        },
        {
          "key": "the_criterion_collection",
          "label": "CC 收藏",
          "aliases": "Criterion Collection"
        },
        {
          "key": "4k_remaster",
          "label": "4K Remaster",
          "aliases": "4K 重制"
        },
        {
          "key": "4k_restoration",
          "label": "4K Restoration",
          "aliases": "4K 修复"
        }
      ]
    },
    {
      "name": "其他",
      "tags": [
        {
          "key": "commentary",
          "label": "评论音轨",
          "aliases": "Commentary"
        },
        {
          "key": "complete",
          "label": "完结",
          "aliases": "全集"
        }
      ]
    },
    {
      "name": "规格",
      "tags": [
        {
          "key": "high_bitrate",
          "label": "高码",
          "aliases": ""
        },
        {
          "key": "high_frame_rate",
          "label": "高帧",
          "aliases": ""
        },
        {
          "key": "high_rating",
          "label": "高分",
          "aliases": ""
        }
      ]
    },
    {
      "name": "剧集状态",
      "tags": [
        {
          "key": "episode_split",
          "label": "分集",
          "aliases": ""
        },
        {
          "key": "collection",
          "label": "合集",
          "aliases": ""
        }
      ]
    }
  ]

