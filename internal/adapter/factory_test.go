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
