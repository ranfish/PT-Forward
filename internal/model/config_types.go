package model

// §33.1.2 — DiscountLevel 枚举（Sprint 91 canonical）

type DiscountLevel string

const (
	DiscountNone        DiscountLevel = "NONE"
	DiscountFree        DiscountLevel = "FREE"
	Discount2xUp        DiscountLevel = "2XUP"
	Discount2xFree      DiscountLevel = "2XFREE"
	DiscountPercent25   DiscountLevel = "PERCENT_25"
	DiscountPercent30   DiscountLevel = "PERCENT_30"
	DiscountPercent50   DiscountLevel = "PERCENT_50"
	Discount2x50        DiscountLevel = "2X50"
	DiscountPercent70   DiscountLevel = "PERCENT_70"
	DiscountPercent75   DiscountLevel = "PERCENT_75"
	DiscountCustom      DiscountLevel = "CUSTOM"
	DiscountAssumeFree  DiscountLevel = "ASSUME_FREE"
)

func (d DiscountLevel) DownloadRatio() float64 {
	switch d {
	case DiscountFree, Discount2xFree:
		return 0.0
	case Discount2x50:
		return 0.5
	case DiscountPercent25:
		return 0.25
	case DiscountPercent30:
		return 0.3
	case DiscountPercent50:
		return 0.5
	case DiscountPercent70:
		return 0.7
	case DiscountPercent75:
		return 0.75
	default:
		return 1.0
	}
}

func (d DiscountLevel) UploadRatio() float64 {
	switch d {
	case Discount2xFree, Discount2x50:
		return 2.0
	case Discount2xUp:
		return 2.0
	default:
		return 1.0
	}
}

func (d DiscountLevel) IsFree() bool {
	if d == DiscountAssumeFree {
		return true
	}
	return d.DownloadRatio() == 0.0
}

func (d DiscountLevel) IsFreeOrDiscount() bool {
	return d.DownloadRatio() < 1.0 || d.UploadRatio() > 1.0
}

func (d DiscountLevel) PriorityValue() int {
	switch d {
	case Discount2xFree:
		return 7
	case DiscountFree:
		return 6
	case Discount2xUp:
		return 5
	case Discount2x50:
		return 4
	case DiscountPercent25:
		return 3
	case DiscountPercent30:
		return 3
	case DiscountPercent50:
		return 2
	case DiscountPercent70:
		return 1
	case DiscountPercent75:
		return 1
	default:
		return 0
	}
}

func (d DiscountLevel) IsValid() bool {
	switch d {
	case DiscountNone, DiscountFree, Discount2xUp, Discount2xFree,
		DiscountPercent25, DiscountPercent30, DiscountPercent50,
		Discount2x50, DiscountPercent70, DiscountPercent75, DiscountCustom,
		DiscountAssumeFree:
		return true
	default:
		return false
	}
}

func NewDiscountLevelFromBool(isFree bool) DiscountLevel {
	if isFree {
		return DiscountFree
	}
	return DiscountNone
}

// §33.1.55 — SideLoadStatus 枚举（Sprint 89）
type SideLoadStatus string

const (
	SideLoadNotRequired SideLoadStatus = "not_required"
	SideLoadPending     SideLoadStatus = "pending"
	SideLoading         SideLoadStatus = "downloading"
	SideLoadCompleted   SideLoadStatus = "completed"
	SideLoadFailed      SideLoadStatus = "failed"
)

// §33.1.56 — MemberStatus 枚举（Sprint 89）
type MemberStatus string

const (
	MemberStatusNew              MemberStatus = "new"
	MemberStatusUploaded         MemberStatus = "uploaded"
	MemberStatusUploading        MemberStatus = "uploading"
	MemberStatusInjected         MemberStatus = "injected"
	MemberStatusSeedingConfirmed MemberStatus = "seeding_confirmed"
	MemberStatusDownloading      MemberStatus = "downloading"
	MemberStatusPaused           MemberStatus = "paused"
	MemberStatusError            MemberStatus = "error"
	MemberStatusBanned           MemberStatus = "banned"
	MemberStatusDeleted          MemberStatus = "deleted"
)

// §33.1.57 — ClientSelectionMode 枚举（Sprint 89）
type ClientSelectionMode string

const (
	SelectionFixed      ClientSelectionMode = "fixed"
	SelectionMostSpace  ClientSelectionMode = "most_space"
	SelectionLeastLoad  ClientSelectionMode = "least_load"
	SelectionRoundRobin ClientSelectionMode = "round_robin"
	SelectionBestFit    ClientSelectionMode = "best_fit"
)

// §33.1.58 — PublishGroupStatus 枚举（Sprint 89）
type PublishGroupStatus string

const (
	GroupActive          PublishGroupStatus = "active"
	GroupPublishing      PublishGroupStatus = "publishing"
	GroupMonitoring      PublishGroupStatus = "monitoring"
	GroupPartiallyPaused PublishGroupStatus = "partially_paused"
	GroupAllPaused       PublishGroupStatus = "all_paused"
	GroupDeleting        PublishGroupStatus = "deleting"
	GroupDeleted         PublishGroupStatus = "deleted"
	GroupPublishFailed   PublishGroupStatus = "publish_failed"
)

// §33.1.59a — PublishCandidateStatus 枚举（Sprint 91）
type PublishCandidateStatus string

const (
	CandidatePending     PublishCandidateStatus = "pending"
	CandidateDownloading PublishCandidateStatus = "downloading"
	CandidateCompleted   PublishCandidateStatus = "completed"
	CandidatePublishing  PublishCandidateStatus = "publishing"
	CandidateDone        PublishCandidateStatus = "done"
	CandidateFailed      PublishCandidateStatus = "failed"
	CandidateSkipped     PublishCandidateStatus = "skipped"
	CandidateOrphan      PublishCandidateStatus = "orphan"
)

// §33.1.59b — ReseedMatchStatus 枚举（Sprint 91）
type ReseedMatchStatus string

const (
	MatchStatusMatched   ReseedMatchStatus = "matched"
	MatchStatusInjecting ReseedMatchStatus = "injecting"
	MatchStatusInjected  ReseedMatchStatus = "injected"
	MatchStatusFailed    ReseedMatchStatus = "failed"
	MatchStatusSkipped   ReseedMatchStatus = "skipped"
)

// §33.1.59c — SeedingTorrentStatus 枚举（Sprint 91）
type SeedingTorrentStatus string

const (
	SeedingStatusPending       SeedingTorrentStatus = "pending"
	SeedingStatusSeeding       SeedingTorrentStatus = "seeding"
	SeedingStatusPausedFreeEnd SeedingTorrentStatus = "paused_free_end"
	SeedingStatusPausedRule    SeedingTorrentStatus = "paused_rule"
	SeedingStatusDeleting      SeedingTorrentStatus = "deleting"
	SeedingStatusDeleteFailed  SeedingTorrentStatus = "delete_failed"
	SeedingStatusDeleted       SeedingTorrentStatus = "deleted"
	SeedingStatusArchived      SeedingTorrentStatus = "archived"
	SeedingStatusUnregistered  SeedingTorrentStatus = "unregistered"
)

// §33.1.59d — PublishCandidateRole 枚举（Sprint 94）
type PublishCandidateRole string

const (
	RoleDownload PublishCandidateRole = "download"
	RoleSource   PublishCandidateRole = "source"
	RoleManual   PublishCandidateRole = "manual"
)

// §33.1.62 — MediaInfoFormat 枚举（Sprint 90）
type MediaInfoFormat string

const (
	MediaInfoFormatBBCode   MediaInfoFormat = "bbcode"
	MediaInfoFormatMarkdown MediaInfoFormat = "markdown"
	MediaInfoFormatHTML     MediaInfoFormat = "html"
)

// §33.1.63 — ReseedTaskStatus 枚举（Sprint 90）
type ReseedTaskStatus string

const (
	ReseedTaskIdle       ReseedTaskStatus = "idle"
	ReseedTaskRunning    ReseedTaskStatus = "running"
	ReseedTaskCancelling ReseedTaskStatus = "cancelling"
	ReseedTaskCompleted  ReseedTaskStatus = "completed"
	ReseedTaskCancelled  ReseedTaskStatus = "cancelled"
	ReseedTaskFailed     ReseedTaskStatus = "failed"
)

// §33.1.80 — PublishTaskStatus 枚举（Sprint 96）
type PublishTaskStatus string

const (
	PublishTaskPending    PublishTaskStatus = "pending"
	PublishTaskChecked    PublishTaskStatus = "checked"
	PublishTaskPublishing PublishTaskStatus = "publishing"
	PublishTaskCompleted  PublishTaskStatus = "completed"
	PublishTaskFailed     PublishTaskStatus = "failed"
)

// §33.1.81 — PublishTaskType 枚举（Sprint 96）
type PublishTaskType string

const (
	PublishTaskTypeManual     PublishTaskType = "manual"
	PublishTaskTypeAuto       PublishTaskType = "auto"
	PublishTaskTypeReschedule PublishTaskType = "reschedule"
)

// §33.1.83 — PublishResultStatus 枚举（Sprint 97b）
type PublishResultStatus string

const (
	PublishResultSkipped    PublishResultStatus = "skipped"
	PublishResultPublishing PublishResultStatus = "publishing"
	PublishResultCompleted  PublishResultStatus = "completed"
	PublishResultFailed     PublishResultStatus = "failed"
)

type Framework string

const (
	FrameworkNexusPHP  Framework = "nexusphp"
	FrameworkUnit3D    Framework = "unit3d"
	FrameworkGazelle   Framework = "gazelle"
	FrameworkMTeam     Framework = "mteam"
	FrameworkTNode     Framework = "tnode"
	FrameworkLuminance Framework = "luminance"
	FrameworkRousi     Framework = "rousi"
	FrameworkGeneric   Framework = "generic"
)

var FrameworkLabels = map[Framework]string{
	FrameworkNexusPHP:  "NexusPHP",
	FrameworkUnit3D:    "UNIT3D",
	FrameworkGazelle:   "Gazelle",
	FrameworkMTeam:     "M-Team",
	FrameworkTNode:     "TNode",
	FrameworkLuminance: "Luminance",
	FrameworkRousi:     "Rousi",
	FrameworkGeneric:   "Generic",
}

func ValidFramework(s string) bool {
	_, ok := FrameworkLabels[Framework(s)]
	return ok
}

type AuthType string

const (
	AuthTypeCookie  AuthType = "cookie"
	AuthTypeAPIKey  AuthType = "apikey"
	AuthTypePasskey AuthType = "passkey"
)

var AuthTypeLabels = map[AuthType]string{
	AuthTypeCookie:  "Cookie",
	AuthTypeAPIKey:  "API Key",
	AuthTypePasskey: "Passkey",
}

func ValidAuthType(s string) bool {
	_, ok := AuthTypeLabels[AuthType(s)]
	return ok
}

type HashStrategy string

const (
	HashGuid       HashStrategy = "guid"
	HashBencode    HashStrategy = "bencode"
	HashFakeFromID HashStrategy = "fake_from_id"
	HashXMLTag     HashStrategy = "xml_tag"
	HashURLParam   HashStrategy = "link_param"
	HashGuidSuffix HashStrategy = "guid_suffix"
	HashNone       HashStrategy = "none"
)

type SizeStrategy string

const (
	SizeEnclosure  SizeStrategy = "enclosure"
	SizeDescRegex  SizeStrategy = "desc_regex"
	SizeTitleRegex SizeStrategy = "title_regex"
	SizeBencode    SizeStrategy = "bencode"
	SizeXMLTag     SizeStrategy = "xml_tag"
	SizeNone       SizeStrategy = "none"
)

type IDStrategy string

const (
	IDQueryParam  IDStrategy = "query_param"
	IDPathSegment IDStrategy = "path_segment"
	IDGuidRegex   IDStrategy = "guid_regex"
	IDLinkRegex   IDStrategy = "link_regex"
	IDGuidText    IDStrategy = "guid_text"
	IDNone        IDStrategy = "none"
)

type CompareType string

const (
	CompareEquals       CompareType = "equals"
	CompareBigger       CompareType = "bigger"
	CompareSmaller      CompareType = "smaller"
	CompareContain      CompareType = "contain"
	CompareIncludeIn    CompareType = "include_in"
	CompareNotContain   CompareType = "not_contain"
	CompareNotIncludeIn CompareType = "not_include_in"
	CompareRegExp       CompareType = "regexp"
	CompareNotRegExp    CompareType = "not_regexp"
)

type DecisionType string

const (
	DecisionMatch                DecisionType = "MATCH"
	DecisionMatchSizeOnly        DecisionType = "MATCH_SIZE_ONLY"
	DecisionMatchPartial         DecisionType = "MATCH_PARTIAL"
	DecisionReleaseGroupMismatch DecisionType = "RELEASE_GROUP_MISMATCH"
	DecisionResolutionMismatch   DecisionType = "RESOLUTION_MISMATCH"
	DecisionSourceMismatch       DecisionType = "SOURCE_MISMATCH"
	DecisionProperRepackMismatch DecisionType = "PROPER_REPACK_MISMATCH"
	DecisionFuzzySizeMismatch    DecisionType = "FUZZY_SIZE_MISMATCH"
	DecisionSizeMismatch         DecisionType = "SIZE_MISMATCH"
	DecisionFileTreeMismatch     DecisionType = "FILE_TREE_MISMATCH"
	DecisionPartialSizeMismatch  DecisionType = "PARTIAL_SIZE_MISMATCH"
	DecisionSameInfoHash         DecisionType = "SAME_INFO_HASH"
	DecisionAlreadyExists        DecisionType = "INFO_HASH_ALREADY_EXISTS"
	DecisionDownloadFailed       DecisionType = "DOWNLOAD_FAILED"
	DecisionNoDownloadLink       DecisionType = "NO_DOWNLOAD_LINK"
	DecisionBlockedRelease       DecisionType = "BLOCKED_RELEASE"
)
