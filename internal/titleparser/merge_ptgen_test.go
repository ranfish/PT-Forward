package titleparser

import "testing"

// §59.254 项1: 第四源合并——year 校正三补丁/main_title 兜底/title_mismatch 可观测
func TestMergePTGen_YearCorrection(t *testing.T) {
	var logs []string
	warn := func(msg string, fields ...any) { logs = append(logs, msg) }

	// 笔误校正（§59.198 形态）
	p := &TechProfile{Year: "2023"}
	MergePTGenInto(p, PTGenMergeInput{Year: "2018"}, warn)
	if p.Year != "2018" {
		t.Errorf("校正: %q", p.Year)
	}
	if len(logs) != 1 {
		t.Errorf("year_corrected 日志: %v", logs)
	}

	// 形态例外：区间跳过
	logs = nil
	p = &TechProfile{Year: "2015-2022"}
	MergePTGenInto(p, PTGenMergeInput{Year: "2015"}, warn)
	if p.Year != "2015-2022" || len(logs) != 0 {
		t.Errorf("区间应跳过: %q logs=%v", p.Year, logs)
	}

	// 形态例外：日期跳过
	p = &TechProfile{Year: "20240326"}
	MergePTGenInto(p, PTGenMergeInput{Year: "2024"}, warn)
	if p.Year != "20240326" {
		t.Errorf("日期应跳过: %q", p.Year)
	}

	// 标题空年→良性填充
	logs = nil
	p = &TechProfile{Year: ""}
	MergePTGenInto(p, PTGenMergeInput{Year: "2018"}, warn)
	if p.Year != "2018" {
		t.Errorf("空年填充: %q", p.Year)
	}

	// 一致零动作
	logs = nil
	p = &TechProfile{Year: "2015"}
	MergePTGenInto(p, PTGenMergeInput{Year: "2015"}, warn)
	if p.Year != "2015" || len(logs) != 0 {
		t.Errorf("一致不应动: %q", p.Year)
	}

	// PTGen 非法年不参与
	p = &TechProfile{Year: "2015"}
	MergePTGenInto(p, PTGenMergeInput{Year: "未知"}, warn)
	if p.Year != "2015" {
		t.Errorf("PTGen 非法年: %q", p.Year)
	}
}

func TestMergePTGen_MainTitleBackfill(t *testing.T) {
	var logs []string
	warn := func(msg string, fields ...any) { logs = append(logs, msg) }

	// 空标题兜底
	p := &TechProfile{}
	MergePTGenInto(p, PTGenMergeInput{EnglishName: "Northeastern Bro III"}, warn)
	if p.MainTitle != "Northeastern Bro III" {
		t.Errorf("空兜底: %q", p.MainTitle)
	}

	// 纯 CJK 兜底
	p = &TechProfile{MainTitle: "东北恋歌"}
	MergePTGenInto(p, PTGenMergeInput{EnglishName: "Northeastern Bro III"}, warn)
	if p.MainTitle != "Northeastern Bro III" {
		t.Errorf("CJK 兜底: %q", p.MainTitle)
	}

	// 意语原名保留（组风格——不兜底不换）
	logs = nil
	p = &TechProfile{MainTitle: "L'ultima volta che siamo stati bambini"}
	MergePTGenInto(p, PTGenMergeInput{EnglishName: "The Last Time We Were Children"}, warn)
	if p.MainTitle != "L'ultima volta che siamo stati bambini" {
		t.Errorf("组风格应保留: %q", p.MainTitle)
	}

	// 冠词差异=归一匹配（无 mismatch 日志——站方标题与 PTGen 冠词差异合法形态）
	logs = nil
	p = &TechProfile{MainTitle: "Killing Faith"}
	MergePTGenInto(p, PTGenMergeInput{EnglishName: "The Killing Faith"}, warn)
	if len(logs) != 0 {
		t.Errorf("冠词差异零日志: %v", logs)
	}

	// 真差异（不同词）记 mismatch
	logs = nil
	p = &TechProfile{MainTitle: "Secret Sunshine"}
	MergePTGenInto(p, PTGenMergeInput{EnglishName: "Secret Sunshine 2007"}, warn)
	if len(logs) != 1 {
		t.Errorf("真差异应记 mismatch: %v", logs)
	}

	// 归一等价（大小写/标点/冠词）零日志
	logs = nil
	p = &TechProfile{MainTitle: "Jumanji"}
	MergePTGenInto(p, PTGenMergeInput{EnglishName: "jumanji"}, warn)
	if len(logs) != 0 {
		t.Errorf("归一等价零日志: %v", logs)
	}
}

func TestTitlesNormalizedEqual(t *testing.T) {
	if !titlesNormalizedEqual("A Knight of the Seven Kingdoms", "knight of the seven kingdoms") {
		t.Error("冠词+大小写应等价")
	}
	if !titlesNormalizedEqual("What Women Want", "what   women    want") {
		t.Error("空白归一")
	}
	if !titlesNormalizedEqual("Killing Faith", "The Killing Faith") {
		t.Error("冠词已滤应等价")
	}
	if titlesNormalizedEqual("Movie A", "Movie B") {
		t.Error("不同词应不等")
	}
}
