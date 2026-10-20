package upstream

import (
	"encoding/json"
	"testing"
)

// firstRole 取 messages[0].role。
func firstRole(t *testing.T, body []byte) string {
	t.Helper()
	var obj struct {
		Messages []struct {
			Role string `json:"role"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(obj.Messages) == 0 {
		return ""
	}
	return obj.Messages[0].Role
}

// TestEnsureConsoleSystem_PrependsForUserFirst 复现 issue #231 的根因形态：
// 多数 OpenAI 兼容客户端首条是 user，国际版上游要求首条必须是 system，
// 否则 400 code=11-128「first message is not system prompt」。
func TestEnsureConsoleSystem_PrependsForUserFirst(t *testing.T) {
	in := []byte(`{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"yo"}]}`)
	out := ensureConsoleSystem(in)
	if got := firstRole(t, out); got != "system" {
		t.Fatalf("first role = %q, want system", got)
	}
	var obj struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatal(err)
	}
	if len(obj.Messages) != 3 {
		t.Fatalf("messages len = %d, want 3（原两条 + 兜底 system）", len(obj.Messages))
	}
	// 原消息逐字保留（顺序与内容都不动），只在最前面插入一条。
	if obj.Messages[1]["content"] != "hi" || obj.Messages[2]["content"] != "yo" {
		t.Fatalf("original messages mutated: %+v", obj.Messages[1:])
	}
}

// TestEnsureConsoleSystem_KeepsSystemFirst 首条已是 system 时不得重复注入。
func TestEnsureConsoleSystem_KeepsSystemFirst(t *testing.T) {
	in := []byte(`{"messages":[{"role":"system","content":"be nice"},{"role":"user","content":"hi"}]}`)
	out := ensureConsoleSystem(in)
	if string(out) != string(in) {
		t.Fatalf("system-first body changed:\n in=%s\nout=%s", in, out)
	}
}

// TestEnsureConsoleSystem_RoleCaseAndSpace 首条 role 带空白/大写也视为 system（与上游同口径）。
func TestEnsureConsoleSystem_RoleCaseAndSpace(t *testing.T) {
	in := []byte(`{"messages":[{"role":"  SYSTEM ","content":"x"}]}`)
	if out := ensureConsoleSystem(in); string(out) != string(in) {
		t.Fatalf("case/space variant should be treated as system, got %s", out)
	}
}

// TestEnsureConsoleSystem_DeveloperNormalizedEarlier 全链路下首条 developer 先被
// normalizeRoles 归一为 system，ensureConsoleSystem 因此不该再插一条。
func TestEnsureConsoleSystem_DeveloperNormalizedEarlier(t *testing.T) {
	out := PrepareBodyOpt([]byte(`{"messages":[{"role":"developer","content":"x"}]}`), false)
	if got := firstRole(t, out); got != "system" {
		t.Fatalf("pipeline first role = %q, want system", got)
	}
	var obj struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatal(err)
	}
	if len(obj.Messages) != 1 {
		t.Fatalf("developer path must not add an extra system message, len=%d", len(obj.Messages))
	}
	if again := ensureConsoleSystem(out); string(again) != string(out) {
		t.Fatalf("ensureConsoleSystem should be a no-op after normalization")
	}
}

// TestEnsureConsoleSystem_PassthroughOnBadBody 坏 body / 无 messages 原样返回，不做二次错误化。
func TestEnsureConsoleSystem_PassthroughOnBadBody(t *testing.T) {
	for _, in := range []string{``, `not json`, `{"model":"x"}`, `{"messages":[]}`, `{"messages":null}`} {
		if out := ensureConsoleSystem([]byte(in)); string(out) != in {
			t.Fatalf("input %q changed to %q", in, out)
		}
	}
}
