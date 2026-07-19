package publish

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// ===== existing_strategy 测试 =====

func TestParseExistingStrategy(t *testing.T) {
	cases := map[string]model.ExistingStrategy{
		"skip":   model.ExistingSkip,
		"update": model.ExistingUpdate,
		"force":  model.ExistingForce,
		"":       model.ExistingSkip,
		"invalid": model.ExistingSkip,
	}
	for in, want := range cases {
		if got := model.ParseExistingStrategy(in); got != want {
			t.Errorf("ParseExistingStrategy(%q) = %q, want %q", in, got, want)
		}
	}
}

// ===== edit_permission 测试 =====

func TestCheckEditPermission_NexusPHP_NoEditButton(t *testing.T) {
	html := `<html><body>无编辑按钮的页面</body></html>`
	allowed, reason := CheckEditPermission(html, "123", "nexusphp")
	if allowed {
		t.Errorf("should not allow without edit button, reason: %s", reason)
	}
}

func TestCheckEditPermission_NexusPHP_WithEditButton(t *testing.T) {
	html := `<html><body>
<a href="edit.php?id=123">编辑</a>
<a href="userdetails.php?id=456">发布者用户名</a>
</body></html>`
	allowed, _ := CheckEditPermission(html, "123", "nexusphp")
	if !allowed {
		t.Error("should allow with edit button present")
	}
}

func TestCheckEditPermission_EmptyHTML(t *testing.T) {
	allowed, reason := CheckEditPermission("", "123", "nexusphp")
	if allowed {
		t.Error("empty HTML should not allow")
	}
	if reason == "" {
		t.Error("should have reason")
	}
}

func TestCheckEditPermission_APIFramework(t *testing.T) {
	allowed, _ := CheckEditPermission("", "123", "mteam")
	if !allowed {
		t.Error("API framework should allow (权限由 API 决定)")
	}
}

// ===== edit_merger 测试 =====

func TestMergeEditFields_DescOverwritten(t *testing.T) {
	existing := map[string]string{
		"descr":    "旧描述",
		"category": "movie",
	}
	newValues := map[string]string{
		"descr": "新描述",
	}
	merged := MergeEditFields(existing, newValues)
	if merged["descr"] != "新描述" {
		t.Errorf("descr should be overwritten, got %q", merged["descr"])
	}
}

func TestMergeEditFields_CategoryPreserved(t *testing.T) {
	existing := map[string]string{
		"descr":    "旧描述",
		"category": "movie",
	}
	newValues := map[string]string{
		"descr":    "新描述",
		"category": "tv", // 不应覆盖
	}
	merged := MergeEditFields(existing, newValues)
	if merged["category"] != "movie" {
		t.Errorf("category should be preserved, got %q", merged["category"])
	}
}

func TestMergeEditFields_MediaInfoOverwritten(t *testing.T) {
	existing := map[string]string{"mediadesc": "旧 MI"}
	newValues := map[string]string{"mediadesc": "新 MI"}
	merged := MergeEditFields(existing, newValues)
	if merged["mediadesc"] != "新 MI" {
		t.Errorf("mediadesc should be overwritten")
	}
}

func TestMergeEditFields_NilExisting(t *testing.T) {
	merged := MergeEditFields(nil, map[string]string{"descr": "新描述"})
	if merged["descr"] != "新描述" {
		t.Errorf("nil existing: descr should come from newValues")
	}
}

// ===== auto_update 测试 =====

// mockEditPublisher 测试用 mock。
type mockEditPublisher struct {
	editForm *model.EditForm
	submitErr error
	submitted bool
}

func (m *mockEditPublisher) GetEditForm(_ context.Context, _ string, _ *model.SiteConfig) (*model.EditForm, error) {
	return m.editForm, nil
}

func (m *mockEditPublisher) SubmitEdit(_ context.Context, _ *model.EditRequest) error {
	m.submitted = true
	return m.submitErr
}

func TestAutoUpdate_Skip(t *testing.T) {
	pub := &mockEditPublisher{}
	result, err := AutoUpdate(
		context.Background(), pub, nil,
		"", "123", nil,
		model.ExistingSkip, "nexusphp", nil,
	)
	if err != nil {
		t.Fatalf("skip should not error: %v", err)
	}
	if !result.Skipped {
		t.Error("skip: should be skipped")
	}
	if pub.submitted {
		t.Error("skip: should not call SubmitEdit")
	}
}

func TestAutoUpdate_Force(t *testing.T) {
	pub := &mockEditPublisher{}
	result, err := AutoUpdate(
		context.Background(), pub, nil,
		"", "123", nil,
		model.ExistingForce, "nexusphp", nil,
	)
	if err != nil {
		t.Fatalf("force should not error: %v", err)
	}
	if !result.Edited {
		t.Error("force: should be marked edited")
	}
	if pub.submitted {
		t.Error("force: should not call SubmitEdit")
	}
}

func TestAutoUpdate_Update_PermissionDenied(t *testing.T) {
	pub := &mockEditPublisher{}
	result, _ := AutoUpdate(
		context.Background(), pub, nil,
		"<html>无编辑按钮</html>", "123", nil,
		model.ExistingUpdate, "nexusphp", zap.NewNop(),
	)
	if !result.Skipped {
		t.Error("update without permission: should be skipped")
	}
	if pub.submitted {
		t.Error("should not submit when permission denied")
	}
}

func TestAutoUpdate_Update_Success(t *testing.T) {
	pub := &mockEditPublisher{
		editForm: &model.EditForm{
			Fields: map[string]string{"descr": "旧描述", "category": "movie"},
		},
	}
	html := `<a href="edit.php?id=123">编辑</a>`
	result, err := AutoUpdate(
		context.Background(), pub, nil,
		html, "123",
		map[string]string{"descr": "新描述"},
		model.ExistingUpdate, "nexusphp", zap.NewNop(),
	)
	if err != nil {
		t.Fatalf("update success should not error: %v", err)
	}
	if !result.Edited {
		t.Error("update: should be edited")
	}
	if !pub.submitted {
		t.Error("should call SubmitEdit")
	}
}

func TestAutoUpdate_UnknownStrategy(t *testing.T) {
	pub := &mockEditPublisher{}
	result, _ := AutoUpdate(
		context.Background(), pub, nil,
		"", "123", nil,
		model.ExistingStrategy("unknown"), "nexusphp", nil,
	)
	if !result.Skipped {
		t.Error("unknown strategy: should be skipped")
	}
}
