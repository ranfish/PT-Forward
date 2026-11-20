package metadata

import (
	"testing"
	"time"
)

// §59.542 追记十三锚：keepfrds h1 badge 形态经分类器的两态语义
// （badge 纯文本进分类器——与列表行同管线·详情页优先判定定案的
// 下游收口）。

func TestClassifyNoTransfer_H1BadgeForm(t *testing.T) {
	now := time.Now()
	t.Run("永久禁转badge", func(t *testing.T) {
		limited, permanent, _ := ClassifyNoTransfer("<b>[禁转]</b>", "2月2天前", now)
		if limited || !permanent {
			t.Errorf("[禁转] 应判 permanent: limited=%v permanent=%v", limited, permanent)
		}
	})
	t.Run("限时禁转badge带窗口", func(t *testing.T) {
		limited, permanent, until := ClassifyNoTransfer("<b>[限时禁转]</b>", "20时46分前", now)
		if !limited || permanent {
			t.Errorf("[限时禁转] 应判 limited: limited=%v permanent=%v", limited, permanent)
		}
		if until.Before(now) {
			t.Errorf("窗口应在未来: until=%v", until)
		}
	})
	t.Run("非禁转badge不误判（词形把关）", func(t *testing.T) {
		limited, permanent, _ := ClassifyNoTransfer("<b>[50%%]</b>", "3天前", now)
		if limited || permanent {
			t.Errorf("折扣 badge 不应判禁转: limited=%v permanent=%v", limited, permanent)
		}
	})
	t.Run("纯文本badge无标签同判", func(t *testing.T) {
		_, permanent, _ := ClassifyNoTransfer("[禁转]", "1天前", now)
		if !permanent {
			t.Error("纯文本 [禁转] 应判 permanent（剥标签后同管线）")
		}
	})
}
