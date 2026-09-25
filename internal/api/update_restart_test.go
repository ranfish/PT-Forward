package api

import "testing"

// §59.281: OTA 任务门控——BusyTaskDesc 空闲态
func TestBusyTaskDescIdle(t *testing.T) {
	h := &PublishTorrentsHandler{}
	h.siteBatch.tasks = map[string]*siteBatchTask{}
	h.siteBatch.active = map[string]string{}
	if d := h.BusyTaskDesc(); d != "" {
		t.Errorf("空闲态应空, got %q", d)
	}
}
