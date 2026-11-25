package publish

import (
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ranfish/pt-forward/internal/testutil"
)

// §59.319 P1: 扫描队列——簇去重/串行消费/err 回调/状态快照。
// §59.573 附（CI 首红根修）：不调 Start()——无消费 goroutine·
// 全部断言确定性（原懒启动下 worker 立即消费+快速失败任务会穿透
// pending/dedup 同步断言——CI race 层 pending=1 want=2 实证）。
func TestBDInfoQueue_DedupAndStatus(t *testing.T) {
	q := NewBDInfoScanQueue(NewBDInfoScanner(zap.NewNop()), zap.NewNop())
	task := BDInfoScanTask{ClientUID: 2, SavePath: "/home/pt/pt3/HDT", Name: "Some.Disc-BDMV", DiscPath: "/home/pt/pt3/HDT/Some.Disc-BDMV"}

	if !q.Enqueue(task) {
		t.Fatal("首次入队应成功")
	}
	if q.Enqueue(task) {
		t.Error("同簇重复入队应被拒")
	}
	other := task
	other.Name = "Another.Disc"
	other.DiscPath = "/x/Another.Disc"
	if !q.Enqueue(other) {
		t.Error("异簇应可入队")
	}
	st := q.Status()
	if st.PendingCount != 2 {
		t.Errorf("pending = %d, want 2", st.PendingCount)
	}
	// 无效任务拒收
	if q.Enqueue(BDInfoScanTask{Name: "x"}) {
		t.Error("缺 ClientUID/DiscPath 应拒收")
	}
}

func TestBDInfoQueue_SerialErrCallback(t *testing.T) {
	scanner := NewBDInfoScanner(zap.NewNop())
	q := NewBDInfoScanQueue(scanner, zap.NewNop())
	var mu sync.Mutex
	var done []BDInfoScanTask
	var errs []error
	q.SetOnDone(func(task BDInfoScanTask, report string, err error) {
		mu.Lock()
		defer mu.Unlock()
		done = append(done, task)
		if err != nil {
			errs = append(errs, err)
		}
	})
	q.Start() // §59.573 附：显式启动消费（生产=main 装配期同款）
	// 不存在的盘路径：Scan 失败 → onDone 收 err（成功路径=真盘集成验收 P3）
	q.Enqueue(BDInfoScanTask{ClientUID: 1, SavePath: "/x", Name: "Bad.Disc", DiscPath: "/nonexistent/Bad.Disc"})
	deadline := time.After(10 * time.Second)
	for {
		mu.Lock()
		n := len(done)
		mu.Unlock()
		if n == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("回调未触发（10s）")
		case <-time.After(50 * time.Millisecond):
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(errs) != 1 {
		t.Errorf("应恰 1 个 err 回调, got %d", len(errs))
	}
	// 失败任务不残留：running 清空（§59.573 附——回调先于 done() 触发·
	// 轮询清空条件替代即时断言〔原窗口=回调已至而 running 未清〕）
	if !testutil.WaitFor(t, 2*time.Second, func() bool {
		st := q.Status()
		return st.Running == "" && st.PendingCount == 0
	}) {
		st := q.Status()
		t.Errorf("队列应清空, got %+v", st)
	}
}
