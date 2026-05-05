package adapter

import (
	"testing"

	"go.uber.org/zap"
)

func TestFactory_Create_NexusPHP(t *testing.T) {
	f := NewFactory(zap.NewNop())
	a := f.Create("nexusphp", NewHTTPDoer())
	if a.Framework() != "nexusphp" {
		t.Errorf("expected nexusphp, got %s", a.Framework())
	}
}

func TestFactory_Create_Generic(t *testing.T) {
	f := NewFactory(zap.NewNop())
	a := f.Create("unknown_framework", NewHTTPDoer())
	if a.Framework() != "unknown_framework" {
		t.Errorf("expected unknown_framework, got %s", a.Framework())
	}
}

func TestFrameworkDefaults_ContainsAll(t *testing.T) {
	expected := []string{"nexusphp", "unit3d", "gazelle", "mteam", "luminance", "generic"}
	for _, fw := range expected {
		def, ok := FrameworkDefaults[fw]
		if !ok {
			t.Errorf("FrameworkDefaults missing %q", fw)
			continue
		}
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

func TestFactory_Create_AllFrameworks(t *testing.T) {
	f := NewFactory(zap.NewNop())
	cases := []struct {
		framework string
		want      string
	}{
		{"tnode", "tnode"},
		{"mteam", "mteam"},
		{"unit3d", "unit3d"},
		{"gazelle", "gazelle"},
		{"rousi", "rousi"},
		{"luminance", "luminance"},
	}
	for _, tc := range cases {
		t.Run(tc.framework, func(t *testing.T) {
			a := f.Create(tc.framework, NewHTTPDoer())
			if a.Framework() != tc.want {
				t.Errorf("expected %s, got %s", tc.want, a.Framework())
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
