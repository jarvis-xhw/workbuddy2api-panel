package pool

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/linguo2625469/workbuddy2api-panel/internal/auth"
)

// TestModelLimitHistoryRecords 6004 命中 → 台账可见（面板「模型上限」的数据源）。
func TestModelLimitHistoryRecords(t *testing.T) {
	p := New("")
	p.Add(&auth.Auth{UID: "u1"})
	reset := time.Now().Add(30 * time.Minute).Truncate(time.Second)
	p.CooldownSoftForModel("u1", time.Minute, reset, "glm-5.2", "6004 model rate limit")
	p.CooldownSoftForModel("u1", time.Minute, reset, "glm-5.2", "6004 model rate limit")

	rows := p.ModelLimitHistory()
	if len(rows) != 1 {
		t.Fatalf("rows=%d want 1: %+v", len(rows), rows)
	}
	ev := rows[0]
	if ev.Model != "glm-5.2" || ev.Count != 2 {
		t.Fatalf("event=%+v want model=glm-5.2 count=2", ev)
	}
	if !ev.ResetAt.Equal(reset) {
		t.Fatalf("reset_at=%v want %v", ev.ResetAt, reset)
	}
	if ev.LastAt.IsZero() {
		t.Fatalf("last_at missing")
	}
}

// TestModelLimitHistoryAudit 无重置时间的 6004（AuditOnly）同样进台账。
func TestModelLimitHistoryAudit(t *testing.T) {
	p := New("")
	p.Add(&auth.Auth{UID: "u1"})
	p.RecordModelRateLimitAudit("u1", "deepseek-v4.1-flash", "6004 model rate limit (reset unknown)")
	rows := p.ModelLimitHistory()
	if len(rows) != 1 || rows[0].Model != "deepseek-v4.1-flash" || rows[0].Count != 1 {
		t.Fatalf("rows=%+v", rows)
	}
	if rows[0].Reason == "" {
		t.Fatalf("reason missing")
	}
}

// TestModelLimitHistoryPersists 跨重启保留：中午撞的上限，下午重启后仍看得到。
func TestModelLimitHistoryPersists(t *testing.T) {
	fp := filepath.Join(t.TempDir(), "state.json")
	p := New(fp)
	p.Add(&auth.Auth{UID: "u1"})
	p.CooldownSoftForModel("u1", time.Minute, time.Now().Add(20*time.Minute), "hy3", "6004 model rate limit")
	p.Flush()
	p.Close()

	p2 := New(fp)
	defer p2.Close()
	rows := p2.ModelLimitHistory()
	if len(rows) != 1 || rows[0].Model != "hy3" {
		t.Fatalf("restored rows=%+v", rows)
	}
}

// TestModelLimitHistoryEmpty 从没撞过上限 → nil（面板按空态渲染，不显示徽标）。
func TestModelLimitHistoryEmpty(t *testing.T) {
	p := New("")
	p.Add(&auth.Auth{UID: "u1"})
	if rows := p.ModelLimitHistory(); rows != nil {
		t.Fatalf("want nil, got %+v", rows)
	}
}
