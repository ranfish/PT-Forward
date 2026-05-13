package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDiscountLevel_DownloadRatio(t *testing.T) {
	tests := []struct {
		level DiscountLevel
		want  float64
	}{
		{DiscountNone, 1.0},
		{DiscountFree, 0.0},
		{Discount2xUp, 1.0},
		{Discount2xFree, 0.0},
		{DiscountPercent25, 0.25},
		{DiscountPercent30, 0.3},
		{DiscountPercent50, 0.5},
		{Discount2x50, 0.5},
		{DiscountPercent70, 0.7},
		{DiscountPercent75, 0.75},
		{DiscountCustom, 1.0},
		{DiscountLevel("unknown"), 1.0},
	}
	for _, tt := range tests {
		got := tt.level.DownloadRatio()
		if got != tt.want {
			t.Errorf("DownloadRatio(%s) = %f, want %f", tt.level, got, tt.want)
		}
	}
}

func TestDiscountLevel_UploadRatio(t *testing.T) {
	tests := []struct {
		level DiscountLevel
		want  float64
	}{
		{DiscountNone, 1.0},
		{DiscountFree, 1.0},
		{Discount2xUp, 2.0},
		{Discount2xFree, 2.0},
		{Discount2x50, 2.0},
		{DiscountPercent50, 1.0},
		{DiscountCustom, 1.0},
	}
	for _, tt := range tests {
		got := tt.level.UploadRatio()
		if got != tt.want {
			t.Errorf("UploadRatio(%s) = %f, want %f", tt.level, got, tt.want)
		}
	}
}

func TestDiscountLevel_IsFree(t *testing.T) {
	if !DiscountFree.IsFree() {
		t.Error("Free should be free")
	}
	if !Discount2xFree.IsFree() {
		t.Error("2xFree should be free")
	}
	if DiscountNone.IsFree() {
		t.Error("None should not be free")
	}
	if Discount2xUp.IsFree() {
		t.Error("2xUp should not be free")
	}
}

func TestDiscountLevel_IsFreeOrDiscount(t *testing.T) {
	freeOrDiscount := []DiscountLevel{
		DiscountFree, Discount2xFree, Discount2xUp, Discount2x50,
		DiscountPercent25, DiscountPercent30, DiscountPercent50,
		DiscountPercent70, DiscountPercent75,
	}
	for _, level := range freeOrDiscount {
		if !level.IsFreeOrDiscount() {
			t.Errorf("%s should be free or discount", level)
		}
	}
	notFreeOrDiscount := []DiscountLevel{DiscountNone, DiscountCustom}
	for _, level := range notFreeOrDiscount {
		if level.IsFreeOrDiscount() {
			t.Errorf("%s should NOT be free or discount", level)
		}
	}
}

func TestDiscountLevel_PriorityValue(t *testing.T) {
	tests := []struct {
		level DiscountLevel
		want  int
	}{
		{Discount2xFree, 7},
		{DiscountFree, 6},
		{Discount2xUp, 5},
		{Discount2x50, 4},
		{DiscountPercent25, 3},
		{DiscountPercent30, 3},
		{DiscountPercent50, 2},
		{DiscountPercent70, 1},
		{DiscountPercent75, 1},
		{DiscountNone, 0},
		{DiscountCustom, 0},
	}
	for _, tt := range tests {
		got := tt.level.PriorityValue()
		if got != tt.want {
			t.Errorf("PriorityValue(%s) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestDiscountLevel_IsValid(t *testing.T) {
	valid := []DiscountLevel{
		DiscountNone, DiscountFree, Discount2xUp, Discount2xFree,
		DiscountPercent25, DiscountPercent30, DiscountPercent50,
		Discount2x50, DiscountPercent70, DiscountPercent75, DiscountCustom,
	}
	for _, level := range valid {
		if !level.IsValid() {
			t.Errorf("%s should be valid", level)
		}
	}
	invalid := []DiscountLevel{DiscountLevel("INVALID"), DiscountLevel("")}
	for _, level := range invalid {
		if level.IsValid() {
			t.Errorf("%q should NOT be valid", level)
		}
	}
}

func TestNewDiscountLevelFromBool(t *testing.T) {
	if NewDiscountLevelFromBool(true) != DiscountFree {
		t.Error("true should return Free")
	}
	if NewDiscountLevelFromBool(false) != DiscountNone {
		t.Error("false should return None")
	}
}

func TestValidFramework(t *testing.T) {
	if !ValidFramework("nexusphp") {
		t.Error("nexusphp should be valid")
	}
	if !ValidFramework("unit3d") {
		t.Error("unit3d should be valid")
	}
	if ValidFramework("invalid") {
		t.Error("invalid should not be valid")
	}
}

func TestValidAuthType(t *testing.T) {
	if !ValidAuthType("cookie") {
		t.Error("cookie should be valid")
	}
	if !ValidAuthType("apikey") {
		t.Error("apikey should be valid")
	}
	if ValidAuthType("invalid") {
		t.Error("invalid should not be valid")
	}
}

func TestSiteHRConfig_SeedTimeH(t *testing.T) {
	cfg := SiteHRConfig{DefaultSeedTimeH: 48}
	if cfg.SeedTimeH() != 48 {
		t.Errorf("expected 48, got %d", cfg.SeedTimeH())
	}

	cfg = SiteHRConfig{DefaultSeedTimeH: 0}
	if cfg.SeedTimeH() != 72 {
		t.Errorf("expected default 72, got %d", cfg.SeedTimeH())
	}

	cfg = SiteHRConfig{DefaultSeedTimeH: 168}
	if cfg.SeedTimeH() != 168 {
		t.Errorf("expected 168, got %d", cfg.SeedTimeH())
	}
}

func TestAppError_Error(t *testing.T) {
	err := &AppError{Code: 12345, Message: "test error"}
	if err.Error() != "test error" {
		t.Errorf("expected 'test error', got %q", err.Error())
	}
}

func TestAppError_Unwrap(t *testing.T) {
	inner := &AppError{Code: 1, Message: "inner"}
	err := &AppError{Code: 2, Message: "outer", Cause: inner}
	if err.Unwrap() != inner {
		t.Error("Unwrap should return inner error")
	}

	err2 := &AppError{Code: 3, Message: "no cause"}
	if err2.Unwrap() != nil {
		t.Error("Unwrap with no cause should return nil")
	}
}

func TestPublishGroup_TableName(t *testing.T) {
	var g PublishGroup
	if g.TableName() != "publish_groups" {
		t.Error("unexpected table name")
	}
}

func TestPublishGroupMember_TableName(t *testing.T) {
	var m PublishGroupMember
	if m.TableName() != "publish_group_members" {
		t.Error("unexpected table name")
	}
}

func TestTorrentEvent_TableName(t *testing.T) {
	var e TorrentEvent
	if e.TableName() != "torrent_events" {
		t.Error("unexpected table name")
	}
}

func TestRSSSubscription_TableName(t *testing.T) {
	var s RSSSubscription
	if s.TableName() != "rss_subscriptions" {
		t.Error("unexpected table name")
	}
}

func TestSeedingTorrentRecord_TableName(t *testing.T) {
	var r SeedingTorrentRecord
	if r.TableName() != "seeding_torrent_records" {
		t.Error("unexpected table name")
	}
}

func TestContentFingerprint_TableName(t *testing.T) {
	var f ContentFingerprint
	if f.TableName() != "content_fingerprints" {
		t.Error("unexpected table name")
	}
}

func TestAutoMigrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	tables := []string{
		"users", "sites", "clients", "rss_subscriptions",
		"publish_groups", "publish_group_members", "torrent_events",
		"seeding_torrent_records", "content_fingerprints",
		"filter_rules", "delete_rules", "publish_tasks",
		"publish_result_records", "publish_candidates",
		"notification_channels", "notification_history",
		"reseed_tasks", "reseed_matches", "ptgen_cache",
		"operation_audit_logs", "scoring_logs",
	}
	for _, table := range tables {
		if !db.Migrator().HasTable(table) {
			t.Errorf("table %q should exist after migration", table)
		}
	}
}

func TestDiscountLevel_PriorityOrdering(t *testing.T) {
	if Discount2xFree.PriorityValue() <= DiscountFree.PriorityValue() {
		t.Error("2xFree should have higher priority than Free")
	}
	if DiscountFree.PriorityValue() <= Discount2xUp.PriorityValue() {
		t.Error("Free should have higher priority than 2xUp")
	}
	if Discount2xUp.PriorityValue() <= DiscountPercent50.PriorityValue() {
		t.Error("2xUp should have higher priority than 50%")
	}
}

func TestSitePublishFullConfig(t *testing.T) {
	cfg := SitePublishFullConfig{
		UploadURL: "https://example.com/upload.php",
	}
	if cfg.UploadURL != "https://example.com/upload.php" {
		t.Error("UploadURL should be set")
	}
}

func TestPublishResponse_Fields(t *testing.T) {
	resp := PublishResponse{
		Success:    true,
		TorrentID:  "123",
		DetailURL:  "https://example.com/t/123",
		TargetSite: "test",
		InfoHash:   "abc123",
	}
	if !resp.Success {
		t.Error("should be success")
	}
	if resp.TorrentID != "123" {
		t.Error("TorrentID mismatch")
	}
}

func TestPublishCandidate_Fields(t *testing.T) {
	now := time.Now()
	c := PublishCandidate{
		SourceSite:        "site1",
		SourceTorrentID:   "t1",
		InfoHash:          "hash1",
		PublishStatus:     CandidatePending,
		DownloadCompleted: false,
		HasHR:             true,
		CompletedAt:       &now,
	}
	if c.SourceSite != "site1" {
		t.Error("SourceSite mismatch")
	}
	if c.PublishStatus != CandidatePending {
		t.Error("expected pending status")
	}
	if !c.HasHR {
		t.Error("HasHR should be true")
	}
}

func TestCompareType_Values(t *testing.T) {
	valid := []CompareType{
		CompareEquals, CompareBigger, CompareSmaller,
		CompareContain, CompareIncludeIn,
		CompareNotContain, CompareNotIncludeIn,
		CompareRegExp, CompareNotRegExp,
	}
	for _, ct := range valid {
		if string(ct) == "" {
			t.Errorf("CompareType %v has empty string value", ct)
		}
	}
}

func TestFrameworkLabels_Completeness(t *testing.T) {
	frameworks := []Framework{
		FrameworkNexusPHP, FrameworkUnit3D, FrameworkGazelle,
		FrameworkMTeam, FrameworkTNode, FrameworkLuminance,
		FrameworkRousi, FrameworkGeneric,
	}
	for _, f := range frameworks {
		if _, ok := FrameworkLabels[f]; !ok {
			t.Errorf("Framework %q missing from FrameworkLabels", f)
		}
	}
	if len(FrameworkLabels) != len(frameworks) {
		t.Errorf("FrameworkLabels has %d entries, expected %d", len(FrameworkLabels), len(frameworks))
	}
}

func TestAuthTypeLabels_Completeness(t *testing.T) {
	types := []AuthType{AuthTypeCookie, AuthTypeAPIKey, AuthTypePasskey}
	for _, at := range types {
		if _, ok := AuthTypeLabels[at]; !ok {
			t.Errorf("AuthType %q missing from AuthTypeLabels", at)
		}
	}
}

func TestSite_JSONOmitEmptyCredentials(t *testing.T) {
	s := Site{
		ID:        1,
		Domain:    "test.com",
		Name:      "Test",
		Framework: "nexusphp",
		AuthType:  "cookie",
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"passkey", "cookie", "api_key", "bearer_token", "auth_key", "auth_hash", "rss_key"} {
		if _, ok := m[field]; ok {
			t.Errorf("empty credential field %q should be omitted in JSON", field)
		}
	}
}

func TestSite_JSONIncludesCredentialsWhenSet(t *testing.T) {
	s := Site{
		Domain:    "test.com",
		Passkey:   "secret123",
		Cookie:    "session=abc",
		APIKey:    "key456",
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(data)
	for _, val := range []string{`"secret123"`, `"session=abc"`, `"key456"`} {
		if !strings.Contains(raw, val) {
			t.Errorf("set credential value %q should appear in JSON", val)
		}
	}
}

func TestClient_PasswordEncrypted(t *testing.T) {
	field, ok := reflect.TypeOf(ClientConfig{}).FieldByName("Password")
	if !ok {
		t.Fatal("ClientConfig.Password field not found")
	}
	if field.Tag.Get("encrypted") != "true" {
		t.Error("ClientConfig.Password should have encrypted:true tag")
	}
}

func TestPublishCandidateStatus_Values(t *testing.T) {
	statuses := []PublishCandidateStatus{
		CandidatePending, CandidateDownloading, CandidateCompleted,
		CandidatePublishing, CandidateDone, CandidateFailed,
		CandidateSkipped, CandidateOrphan,
	}
	seen := map[PublishCandidateStatus]bool{}
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate PublishCandidateStatus: %q", s)
		}
		seen[s] = true
	}
}

func TestMemberStatus_Values(t *testing.T) {
	statuses := []MemberStatus{
		MemberStatusNew, MemberStatusUploaded, MemberStatusUploading,
		MemberStatusInjected, MemberStatusSeedingConfirmed,
		MemberStatusDownloading, MemberStatusPaused,
		MemberStatusError, MemberStatusBanned, MemberStatusDeleted,
	}
	seen := map[MemberStatus]bool{}
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate MemberStatus: %q", s)
		}
		seen[s] = true
	}
}

func TestReseedMatchStatus_Values(t *testing.T) {
	statuses := []ReseedMatchStatus{
		MatchStatusMatched, MatchStatusInjecting,
		MatchStatusInjected, MatchStatusFailed, MatchStatusSkipped,
	}
	seen := map[ReseedMatchStatus]bool{}
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate ReseedMatchStatus: %q", s)
		}
		seen[s] = true
	}
}

func TestSideLoadStatus_Values(t *testing.T) {
	statuses := []SideLoadStatus{
		SideLoadNotRequired, SideLoadPending, SideLoading,
		SideLoadCompleted, SideLoadFailed,
	}
	seen := map[SideLoadStatus]bool{}
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate SideLoadStatus: %q", s)
		}
		seen[s] = true
	}
}

func TestHashStrategy_Values(t *testing.T) {
	strategies := []HashStrategy{
		HashGuid, HashBencode, HashFakeFromID,
		HashXMLTag, HashURLParam, HashGuidSuffix, HashNone,
	}
	for _, s := range strategies {
		if string(s) == "" {
			t.Errorf("HashStrategy has empty string value")
		}
	}
}

func TestClientSelectionMode_Values(t *testing.T) {
	modes := []ClientSelectionMode{
		SelectionFixed, SelectionMostSpace,
		SelectionLeastLoad, SelectionRoundRobin, SelectionBestFit,
	}
	for _, m := range modes {
		if string(m) == "" {
			t.Errorf("ClientSelectionMode has empty string value")
		}
	}
}

func TestFilterRule_TableName(t *testing.T) {
	r := FilterRule{}
	if r.TableName() != "filter_rules" {
		t.Errorf("expected filter_rules, got %q", r.TableName())
	}
}

func TestRuleCondition_JSONRoundtrip(t *testing.T) {
	cond := RuleCondition{
		Key:         "size",
		CompareType: CompareBigger,
		Value:       "1073741824",
	}
	data, err := json.Marshal(cond)
	if err != nil {
		t.Fatal(err)
	}
	var decoded RuleCondition
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Key != cond.Key || decoded.CompareType != cond.CompareType || decoded.Value != cond.Value {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, cond)
	}
}

func TestDecisionType_Values(t *testing.T) {
	decisions := []DecisionType{
		DecisionMatch, DecisionMatchSizeOnly, DecisionMatchPartial,
		DecisionReleaseGroupMismatch, DecisionResolutionMismatch,
		DecisionSourceMismatch, DecisionProperRepackMismatch,
		DecisionFuzzySizeMismatch, DecisionSizeMismatch,
		DecisionFileTreeMismatch, DecisionPartialSizeMismatch,
		DecisionSameInfoHash, DecisionAlreadyExists,
		DecisionDownloadFailed, DecisionNoDownloadLink,
		DecisionBlockedRelease,
	}
	seen := map[DecisionType]bool{}
	for _, d := range decisions {
		if seen[d] {
			t.Errorf("duplicate DecisionType: %q", d)
		}
		seen[d] = true
	}
}

func TestPublishGroupStatus_Values(t *testing.T) {
	statuses := []PublishGroupStatus{
		GroupActive, GroupPublishing, GroupMonitoring,
		GroupPartiallyPaused, GroupAllPaused,
		GroupDeleting, GroupDeleted, GroupPublishFailed,
	}
	seen := map[PublishGroupStatus]bool{}
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate PublishGroupStatus: %q", s)
		}
		seen[s] = true
	}
}
