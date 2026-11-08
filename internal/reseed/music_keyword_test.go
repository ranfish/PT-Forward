package reseed

import (
	"testing"
)

// §59.474: 音乐统一管道 v2 锚——三站全量 7417 颗验证·关键形态对照。
// 期望值与 Python 模拟产出逐字对齐（Go vs Python 移植零偏差验证）。
func TestExtractMusicSearchKeyword(t *testing.T) {
	cases := []struct {
		name      string
		stripYear bool
		want      string
	}{
		// 主规则域：站方 canonical 形态（艺人 - 专辑 年份 格式）
		{"Carol Albert - Stronger Now (2020)", false, "Carol Albert Stronger Now 2020"},
		{"Taylor Swift - 1989 (Deluxe) (2014) [FLAC]", false, "Taylor Swift 1989 Deluxe 2014"},
		// 剥年版
		{"Carol Albert - Stronger Now (2020)", true, "Carol Albert Stronger Now"},
		// §59.477 修订: FLAC 入规格词剥除集（皇后标题恒带·非判别词·
		// 剥除=关键词更宽召回更稳）——原锚 "Taylor Swift Deluxe FLAC"
		{"Taylor Swift - 1989 (Deluxe) (2014) [FLAC]", true, "Taylor Swift Deluxe"},
		// §59.477 A类: & 入分隔符集（皇后带 & 必零结果·12 环境用户实测）
		{"Iggy & the Stooges - Jesus Loves the Stooges (1995) {BCD114} [FLAC]", false, "Iggy the Stooges Jesus Loves"},
		{"Pollini - Piano Sonatas Nos.20&21 2021 FLAC", false, "Pollini Piano Sonatas Nos 20"},
		// §59.477 B类: Scene 形态清洗——目录号+规格词剥除（主体=Artist+Track）
		{"Sydro-Synaptic_Overload-(KSR043)-16BIT-WEB-FLAC-2025-TR", false, "Sydro Synaptic Overload"},
		{"Sambassadeur-New_Moon-(LAB070)-CDEP-FLAC-2005-MANDY", false, "Sambassadeur New Moon"},
		// C1 年份-连字符（中间括号不剥——Trim 只剥首尾·Python 同构）
		{"1999-爱人啊[台湾][WAV]", false, "1999 爱人啊[台湾][WAV"},
		{"1999-爱人啊[台湾][WAV]", true, "爱人啊[台湾][WAV"},
		// D1 点分隔（5 词截断）
		{"Hebert.Von.Karajan.-.[Wagner.Preludes.and.Overtures].专辑.(Flac)", false, "Hebert Von Karajan Wagner Preludes"},
		// C5 日文头方括号
		{"[101201] TVアニメ「アマガミSS」オリジナルサウンドトラック／大森俊之", false, "TVアニメ「アマガミSS」オリジナルサウンドトラック／大森俊之"},
		// 头剥空回退（整名方括号——"_"空格化为"2"分词）
		{"[鳳飛飛歌林專輯飛躍復刻全集之二十三_2-1983 出外的人]", false, "鳳飛飛歌林專輯飛躍復刻全集之二十三 2 1983 出外的人"},
		// A1 空格化（5 词截断——"Management"第 6 词截）
		{"t.A.t.U-Waste Management", false, "t A t U Waste"},
		// 剥头【】整段
		{"【MQA352.8】Himmelborgen", false, "Himmelborgen"},
		// A2 年份+名
		{"2003 Call Off the Search", false, "2003 Call Off the Search"},
		{"2003 Call Off the Search", true, "Call Off the Search"},
		// 多段+副标题括号
		{"Sodagreen (蘇打綠) - Imcomparable Beauty (無與倫比的美麗", false, "Sodagreen 蘇打綠 Imcomparable Beauty 無與倫比的美麗"},
		// 中文短名
		{"甄妮-心聲", false, "甄妮 心聲"},
		// 书名号《》内文优先
		// §59.478 修订: Nicki 锚回归 ：整体形态——粘连拆分收紧为仅纯
		// ASCII 字母数字 token（含全角字符不拆·日文标题案）
		{"Nicki Minaj -《Pink Friday：Roman Reloaded》[Explicit Version]", false, "Pink Friday：Roman Reloaded"},
	}
	for _, c := range cases {
		got := ExtractMusicSearchKeyword(c.name, c.stripYear)
		if got != c.want {
			t.Errorf("ExtractMusicSearchKeyword(%q, %v) = %q, want %q", c.name, c.stripYear, got, c.want)
		}
	}
}

// §59.478: 粘连拆分+SACD/ISO 增补锚（RCA 案）。
func TestExtractMusicSearchKeywordGlued(t *testing.T) {
	cases := []struct {
		name      string
		stripYear bool
		want      string
	}{
		// RCA 案（12 环境·皇后 tid=99869 目标在）："60SACD"→[60,SACD]·
		// §59.487 用户实测定案：SACD=判别词（站内标题不恒带）保留——
		// 皇后站方标题 "60 SACD" 空格形态 AND Contains 命中·更精准
		{"古典名厂RCA Living stereo 60SACD 系列收藏 [SACD-ISO- 152G]", false, "古典名厂RCA Living stereo 60 SACD"},
		// 纯字母/纯数字不拆
		{"Carol Albert Stronger Now 2020", false, "Carol Albert Stronger Now 2020"},
	}
	for _, c := range cases {
		if got := ExtractMusicSearchKeyword(c.name, c.stripYear); got != c.want {
			t.Errorf("ExtractMusicSearchKeyword(%q, %v) = %q, want %q", c.name, c.stripYear, got, c.want)
		}
	}
}

// §59.480: VA 缩写剥除锚（Ultimate Box Set 案——用户实测带 VA 零结果·
// 剥除后 "The Ultimate Box Set Series" 命中 tid=123139）。
func TestExtractMusicSearchKeywordAbbrev(t *testing.T) {
	cases := []struct {
		name      string
		stripYear bool
		want      string
	}{
		// VA 剥除后 cap5 保住 Series（原 cap 被 VA 占位截断丢词）
		{"VA - The Ultimate Box Set Series 2008 FLAC", false, "The Ultimate Box Set Series"},
		{"VA - 精选合集 2020", false, "精选合集 2020"}, // VA 通用剥除（皇后全称形态·非歧视词）
	}
	for _, c := range cases {
		if got := ExtractMusicSearchKeyword(c.name, c.stripYear); got != c.want {
			t.Errorf("ExtractMusicSearchKeyword(%q, %v) = %q, want %q", c.name, c.stripYear, got, c.want)
		}
	}
}

// §59.487: SACD 判别词定案锚——首 token spec 摧毁关键词案根治
// （"SACD&LP 24bit..."合集名以规格开头·旧 spec break 输出空→
// exhausted；用户实测皇后搜 "SACD LP" 直接命中站方标题形态）。
func TestExtractMusicSearchKeywordSacdKept(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"SACD&LP 24bit Classical music-古典音乐", "SACD LP"},
		{"VA - SACD&LP 24bit Classical music 2000 - FLAC 分軌", "SACD LP"},
	}
	for _, c := range cases {
		if got := ExtractMusicSearchKeyword(c.name, false); got != c.want {
			t.Errorf("ExtractMusicSearchKeyword(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
