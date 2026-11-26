package adapter

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func TestFactory_Create_NexusPHP(t *testing.T) {
	f := NewFactory(zap.NewNop(), nil)
	a := f.Create("nexusphp", NewHTTPDoer())
	if a.Framework() != "nexusphp" {
		t.Errorf("expected nexusphp, got %s", a.Framework())
	}
}

func TestFactory_Create_Generic(t *testing.T) {
	f := NewFactory(zap.NewNop(), nil)
	a := f.Create("unknown_framework", NewHTTPDoer())
	if a.Framework() != "unknown_framework" {
		t.Errorf("expected unknown_framework, got %s", a.Framework())
	}
}

// 08 刀3：基准改单一来源——遍历 FrameworkDefaults 自身（原手抄 6 项
// 子集→现在全 10 项覆盖）。hdarea/hxpt/piggo 不注册是设计决策
// （§59.533·API-only 站——附录G §1.3 禁止机械补齐）。
func TestFrameworkDefaults_ContainsAll(t *testing.T) {
	if len(FrameworkDefaults) != 10 {
		t.Errorf("FrameworkDefaults has %d entries, expected 10（有意不注册 hdarea/hxpt/piggo——附录G §1.3）", len(FrameworkDefaults))
	}
	for fw, def := range FrameworkDefaults {
		if def.HashStrategy == "" {
			t.Errorf("%q has empty HashStrategy", fw)
		}
		if def.SizeStrategy == "" {
			t.Errorf("%q has empty SizeStrategy", fw)
		}
		if def.IDStrategy == "" {
			t.Errorf("%q has empty IDStrategy", fw)
		}
		if def.IDPattern == "" {
			t.Errorf("%q has empty IDPattern", fw)
		}
	}
}

func TestNewHTTPDoer(t *testing.T) {
	doer := NewHTTPDoer()
	if doer == nil {
		t.Fatal("NewHTTPDoer returned nil")
	}
	if doer.Client == nil {
		t.Fatal("HTTPDoer.Client is nil")
	}
}

// 08 刀3：基准改单一来源——cases 从 model.FrameworkLabels 派生
// （原手抄 6 项→现在 13 项全覆盖）。类型断言锁 Labels ⊆ factory：
// default 兜底也回显输入（TestFactory_Create_Generic 证明）——回显
// 锚无法区分真 case 与兜底，必须断言具体类型（generic/luminance 项
// 除外——luminance 是显式 case 无专属结构体）。新增 framework 忘进 Labels 时本锚测不到
// （factory ⊃ Labels 方向）——靠附录G §1.4 checklist 纪律+model_test
// 基数锁（13）兜住。
func TestFactory_Create_AllFrameworks(t *testing.T) {
	f := NewFactory(zap.NewNop(), nil)
	for fw := range model.FrameworkLabels {
		t.Run(string(fw), func(t *testing.T) {
			a := f.Create(string(fw), NewHTTPDoer())
			if a.Framework() != string(fw) {
				t.Errorf("expected %s, got %s", string(fw), a.Framework())
			}
			// luminance 无专属 adapter（factory.go :103 显式 case 返回
			// GenericAdapter("luminance")）——与 generic 同走 GenericAdapter 合法
			if _, isGeneric := a.(*GenericAdapter); isGeneric && fw != model.FrameworkGeneric && fw != model.FrameworkLuminance {
				t.Errorf("framework %q fell into GenericAdapter default branch（Labels ⊆ factory 破坏·case 缺失或值拼错）", fw)
			}
		})
	}
}

func TestNewHTTPDoerWithSite_NoProxy(t *testing.T) {
	doer := NewHTTPDoerWithSite("", false)
	if doer == nil {
		t.Fatal("NewHTTPDoerWithSite returned nil")
	}
	if doer.Client == nil {
		t.Fatal("HTTPDoer.Client is nil")
	}
}

func TestNewHTTPDoerWithSite_WithProxy(t *testing.T) {
	doer := NewHTTPDoerWithSite("http://127.0.0.1:8080", false)
	if doer == nil {
		t.Fatal("NewHTTPDoerWithSite returned nil")
	}
	if doer.Client == nil {
		t.Fatal("HTTPDoer.Client is nil")
	}
}

func TestNewHTTPDoerWithSite_InvalidProxy(t *testing.T) {
	doer := NewHTTPDoerWithSite("://invalid", false)
	if doer == nil {
		t.Fatal("NewHTTPDoerWithSite returned nil for invalid proxy")
	}
	if doer.Client == nil {
		t.Fatal("HTTPDoer.Client is nil")
	}
}

func TestFrameworkDefaults_Rousi(t *testing.T) {
	def, ok := FrameworkDefaults["rousi"]
	if !ok {
		t.Fatal("FrameworkDefaults missing rousi")
	}
	if def.HashStrategy != "guid" {
		t.Errorf("expected guid, got %s", def.HashStrategy)
	}
	if def.SizeStrategy != "enclosure" {
		t.Errorf("expected enclosure, got %s", def.SizeStrategy)
	}
	if def.IDStrategy != "path_segment" {
		t.Errorf("expected path_segment, got %s", def.IDStrategy)
	}
	if def.IDPattern != "uuid" {
		t.Errorf("expected uuid, got %s", def.IDPattern)
	}
}

func TestFrameworkDefaults_TNode(t *testing.T) {
	def, ok := FrameworkDefaults["tnode"]
	if !ok {
		t.Fatal("FrameworkDefaults missing tnode")
	}
	if def.HashStrategy != "guid" {
		t.Errorf("expected guid, got %s", def.HashStrategy)
	}
	if def.SizeStrategy != "enclosure" {
		t.Errorf("expected enclosure, got %s", def.SizeStrategy)
	}
	if def.IDStrategy != "query_param" {
		t.Errorf("expected query_param, got %s", def.IDStrategy)
	}
	if def.IDPattern != "id" {
		t.Errorf("expected id, got %s", def.IDPattern)
	}
}
