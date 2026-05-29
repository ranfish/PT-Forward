package filter

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

func BenchmarkMatchConditionExport_Equals(b *testing.B) {
	ctx := &EvalContext{Title: "The.Matrix.1999.2160p.UHD.BluRay.x265"}
	cond := model.RuleCondition{Key: "title", CompareType: model.CompareEquals, Value: "the.matrix.1999.2160p.uhd.bluray.x265"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchConditionExport(cond, ctx)
	}
}

func BenchmarkMatchConditionExport_Contain(b *testing.B) {
	ctx := &EvalContext{Title: "The.Matrix.1999.2160p.UHD.BluRay.x265-SPARKS"}
	cond := model.RuleCondition{Key: "title", CompareType: model.CompareContain, Value: "matrix"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchConditionExport(cond, ctx)
	}
}

func BenchmarkMatchConditionExport_RegExp(b *testing.B) {
	ctx := &EvalContext{Title: "The.Matrix.1999.2160p.UHD.BluRay.x265-SPARKS"}
	cond := model.RuleCondition{Key: "title", CompareType: model.CompareRegExp, Value: `(?i)(4k|2160p).*bluray`}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchConditionExport(cond, ctx)
	}
}

func BenchmarkMatchConditionExport_RegExpMiss(b *testing.B) {
	ctx := &EvalContext{Title: "The.Matrix.1999.1080p.BluRay.x264-SPARKS"}
	cond := model.RuleCondition{Key: "title", CompareType: model.CompareRegExp, Value: `(?i)(4k|2160p).*bluray`}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchConditionExport(cond, ctx)
	}
}

func BenchmarkMatchConditionExport_IncludeIn(b *testing.B) {
	ctx := &EvalContext{Title: "Movie", Category: "电影"}
	cond := model.RuleCondition{Key: "category", CompareType: model.CompareIncludeIn, Value: "电影,剧集,纪录片"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchConditionExport(cond, ctx)
	}
}

func BenchmarkMatchConditionExport_SizeCompare(b *testing.B) {
	ctx := &EvalContext{Title: "Big.File", Size: 50 << 30}
	cond := model.RuleCondition{Key: "size", CompareType: model.CompareBigger, Value: "10737418240"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchConditionExport(cond, ctx)
	}
}

func BenchmarkMatchConditionExport_ComplexRegExp(b *testing.B) {
	ctx := &EvalContext{Title: "Inception.2010.2160p.UHD.BluRay.Remux.DV.HDR.HE Atmos.TrueHD.7.1-FGT"}
	cond := model.RuleCondition{Key: "title", CompareType: model.CompareRegExp, Value: `(?i)\b(remux|fullbluray)\b.*\b(hevc|hdr|dv|atmos|uhd)\b`}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchConditionExport(cond, ctx)
	}
}

func BenchmarkMatchByIDs_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			MatchConditionExport(model.RuleCondition{
				Key:         "title",
				CompareType: model.CompareContain,
				Value:       "test",
			}, &EvalContext{Title: "test title value", Size: int64(i)})
			i++
		}
	})
}

func BenchmarkMatchConditionExport_Noop(b *testing.B) {
	ctx := context.Background()
	_ = ctx
	cond := model.RuleCondition{Key: "free", CompareType: model.CompareEquals, Value: "true"}
	evalCtx := &EvalContext{Title: "Test", Free: true}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchConditionExport(cond, evalCtx)
	}
}
