package reseed

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/adapter"
)

// §59.543 编译期防线：声明 SupportsSearchByPiecesHash()=true 的 adapter
// 必须真正满足 piecesHashSearcher 接口——此前 PiggoAdapter 声明 true 却
// 缺 SearchByPiecesHash 方法·引擎 :736 类型断言失败被静默排除（二师兄
// 特征辅种零结果根因·零日志黑洞排查一下午）。此锚让「声明与实现脱节」
// 在编译期即红·不再依赖生产日志发现。
func TestPiecesHashSearcherContract(t *testing.T) {
	var _ piecesHashSearcher = (&adapter.PiggoAdapter{})
	var _ piecesHashSearcher = (&adapter.NexusPHPAdapter{})
	var _ piecesHashSearcher = (&adapter.YemaptAdapter{})
}
