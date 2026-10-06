package publish

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// BDInfoScanTask 一次原盘扫描任务（簇级——同簇 59-74 站共享一次扫描产物）。
type BDInfoScanTask struct {
	ClientUID uint
	SavePath  string
	Name      string
	DiscPath  string // 磁盘门检测到的盘根路径（BDMV 父目录或 .iso 文件）
}

func (t BDInfoScanTask) key() string {
	return fmt.Sprintf("%d|%s|%s", t.ClientUID, t.SavePath, t.Name)
}

func (t BDInfoScanTask) String() string { return t.Name }

// BDInfoScanQueue 原盘 BDInfo 扫描任务队列（§59.319 P1）。
//
// 语义（§59.51 截图后台任务架构复刻 + 簇去重强化）：
//   - 全局单例**串行**消费——全盘扫描 IO 绑定（40G≈5min/90G≈11min），
//     串行即最优（避免多盘并发读竞争做种盘 IO）
//   - 簇键去重（client_uid|save_path|name）：pending/running 集合查重
//   - 每任务独立 Background ctx + 30min 上限（新版库支持真取消）
//   - panic recover：单任务崩溃不毒化队列
//   - onDone 回调（api 层注入）：落库 bd_info + 簇传播 + 规格重算
type BDInfoScanQueue struct {
	scanner *BDInfoScanner
	logger  *zap.Logger

	mu      sync.Mutex
	pending []BDInfoScanTask
	running *BDInfoScanTask
	onDone  func(task BDInfoScanTask, report string, err error)

	startOnce sync.Once
	wake      chan struct{}
}

func NewBDInfoScanQueue(scanner *BDInfoScanner, logger *zap.Logger) *BDInfoScanQueue {
	return &BDInfoScanQueue{
		scanner: scanner,
		logger:  logger,
		wake:    make(chan struct{}, 1),
	}
}

// SetOnDone 注册完成回调（装配期一次性调用，运行期只读）。
func (q *BDInfoScanQueue) SetOnDone(fn func(task BDInfoScanTask, report string, err error)) {
	q.mu.Lock()
	q.onDone = fn
	q.mu.Unlock()
}

// Enqueue 入队（簇键去重：已在 pending/running 则跳过返回 false）。
// 不查 DB 簇内是否已有产物——调用方负责（簇查重需要 DB，队列保持纯内存）。
func (q *BDInfoScanQueue) Enqueue(task BDInfoScanTask) bool {
	if task.DiscPath == "" || task.Name == "" || task.ClientUID == 0 {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.running != nil && q.running.key() == task.key() {
		return false
	}
	for _, p := range q.pending {
		if p.key() == task.key() {
			return false
		}
	}
	q.pending = append(q.pending, task)
	q.startOnce.Do(func() { go q.loop() })
	select {
	case q.wake <- struct{}{}:
	default:
	}
	q.logger.Info("bdinfo scan enqueued",
		zap.String("name", task.Name),
		zap.Int("pending", len(q.pending)))
	return true
}

// BDInfoQueueStatus 队列状态快照（P3 手动批量端点/前端轮询共用）。
type BDInfoQueueStatus struct {
	Running      string   `json:"running"`
	RunningSince string   `json:"running_since,omitempty"`
	PendingCount int      `json:"pending_count"`
	Pending      []string `json:"pending"`
}

func (q *BDInfoScanQueue) Status() BDInfoQueueStatus {
	q.mu.Lock()
	defer q.mu.Unlock()
	st := BDInfoQueueStatus{Pending: make([]string, 0, len(q.pending))}
	if q.running != nil {
		st.Running = q.running.Name
		st.RunningSince = time.Now().Format(time.RFC3339)
	}
	for _, p := range q.pending {
		st.Pending = append(st.Pending, p.Name)
	}
	st.PendingCount = len(q.pending)
	return st
}

func (q *BDInfoScanQueue) next() (BDInfoScanTask, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return BDInfoScanTask{}, false
	}
	task := q.pending[0]
	q.pending = q.pending[1:]
	q.running = &task
	return task, true
}

func (q *BDInfoScanQueue) done() {
	q.mu.Lock()
	q.running = nil
	q.mu.Unlock()
}

func (q *BDInfoScanQueue) loop() {
	for range q.wake {
		for {
			task, ok := q.next()
			if !ok {
				break
			}
			q.runOne(task)
			q.done()
		}
	}
}

func (q *BDInfoScanQueue) runOne(task BDInfoScanTask) {
	// §59.51 审计同款：panic 防护——毒任务不毒化队列
	defer func() {
		if p := recover(); p != nil {
			q.logger.Error("bdinfo scan panic",
				zap.String("name", task.Name), zap.Any("panic", p))
			q.fireDone(task, "", fmt.Errorf("internal panic: %v", p))
		}
	}()
	// 脱离调用方 ctx（批量获取=Background/单条=HTTP 请求均不适配 5-11min 长任务）
	// + 30min 上限（90G 慢盘余量；新版库 ctx 取消真实生效——P0 对拍同源）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	start := time.Now()
	report, err := q.scanner.Scan(ctx, task.DiscPath, nil)
	q.logger.Info("bdinfo scan finished",
		zap.String("name", task.Name),
		zap.Int64("elapsed_sec", int64(time.Since(start).Seconds())),
		zap.Bool("ok", err == nil),
		zap.Int("report_len", len(report)))
	q.fireDone(task, report, err)
}

func (q *BDInfoScanQueue) fireDone(task BDInfoScanTask, report string, err error) {
	q.mu.Lock()
	fn := q.onDone
	q.mu.Unlock()
	if fn != nil {
		fn(task, report, err)
	}
}
