package upstream

import (
	"encoding/json"
	"testing"
)

// decodeIDs 取出出站 messages 里 tool_calls[].id 与 tool 结果的 tool_call_id，按序返回。
func decodeIDs(t *testing.T, out []byte) (calls, results []string) {
	t.Helper()
	var obj struct {
		Messages []struct {
			Role       string `json:"role"`
			ToolCallID string `json:"tool_call_id"`
			ToolCalls  []struct {
				ID string `json:"id"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, m := range obj.Messages {
		for _, c := range m.ToolCalls {
			calls = append(calls, c.ID)
		}
		if m.ToolCallID != "" {
			results = append(results, m.ToolCallID)
		}
	}
	return
}

// TestDedupeToolCallIDs_TruncatedParallelBatch 复现 issue #81 的报文形状：
// 客户端把并行调用的 id 截断成同一个值，三条 tool_calls 与三条 tool 结果全部撞车。
// 期望：首条保留，其余改名为 -2/-3，且结果侧按顺序同步。
func TestDedupeToolCallIDs_TruncatedParallelBatch(t *testing.T) {
	body := `{"model":"deepseek-v4.1-flash","messages":[
		{"role":"user","content":"go"},
		{"role":"assistant","content":"","tool_calls":[
			{"id":"toolu_call-9156406b-551d-4f12-a8bc-6736e","type":"function","function":{"name":"a","arguments":"{}"}},
			{"id":"toolu_call-9156406b-551d-4f12-a8bc-6736e","type":"function","function":{"name":"b","arguments":"{}"}},
			{"id":"toolu_call-9156406b-551d-4f12-a8bc-6736e","type":"function","function":{"name":"c","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"toolu_call-9156406b-551d-4f12-a8bc-6736e","content":"1"},
		{"role":"tool","tool_call_id":"toolu_call-9156406b-551d-4f12-a8bc-6736e","content":"2"},
		{"role":"tool","tool_call_id":"toolu_call-9156406b-551d-4f12-a8bc-6736e","content":"3"}]}`
	out := PrepareBodyOpt([]byte(body), false)
	calls, results := decodeIDs(t, out)
	want := []string{"toolu_call-9156406b-551d-4f12-a8bc-6736e", "toolu_call-9156406b-551d-4f12-a8bc-6736e-2", "toolu_call-9156406b-551d-4f12-a8bc-6736e-3"}
	if len(calls) != len(want) {
		t.Fatalf("calls=%v", calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls[%d]=%q want %q (all=%v)", i, calls[i], want[i], calls)
		}
	}
	if len(results) != len(want) {
		t.Fatalf("results=%v", results)
	}
	for i := range want {
		if results[i] != want[i] {
			t.Fatalf("results[%d]=%q want %q (all=%v)", i, results[i], want[i], results)
		}
	}
}

// TestDedupeToolCallIDs_HealthyRequestUnchanged 健康请求（id 唯一）必须零改动。
func TestDedupeToolCallIDs_HealthyRequestUnchanged(t *testing.T) {
	body := `{"messages":[
		{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"a","arguments":"{}"}},{"id":"call_2","type":"function","function":{"name":"b","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"call_1","content":"1"},
		{"role":"tool","tool_call_id":"call_2","content":"2"}]}`
	out := PrepareBodyOpt([]byte(body), false)
	calls, results := decodeIDs(t, out)
	if len(calls) != 2 || calls[0] != "call_1" || calls[1] != "call_2" {
		t.Fatalf("calls changed: %v", calls)
	}
	if len(results) != 2 || results[0] != "call_1" || results[1] != "call_2" {
		t.Fatalf("results changed: %v", results)
	}
}

// TestDedupeToolCallIDs_AvoidsCollision 改名不能撞上正文里已经存在的 id。
func TestDedupeToolCallIDs_AvoidsCollision(t *testing.T) {
	body := `{"messages":[
		{"role":"assistant","tool_calls":[{"id":"dup","type":"function","function":{"name":"a","arguments":"{}"}},{"id":"dup","type":"function","function":{"name":"b","arguments":"{}"}}]},
		{"role":"assistant","tool_calls":[{"id":"dup-2","type":"function","function":{"name":"c","arguments":"{}"}}]},
		{"role":"tool","tool_call_id":"dup","content":"1"},
		{"role":"tool","tool_call_id":"dup-2","content":"2"}]}`
	out := PrepareBodyOpt([]byte(body), false)
	calls, results := decodeIDs(t, out)
	if len(calls) != 3 {
		t.Fatalf("calls=%v", calls)
	}
	if calls[0] != "dup" || calls[2] != "dup-2" {
		t.Fatalf("unexpected calls=%v", calls)
	}
	if calls[1] == "dup" || calls[1] == "dup-2" {
		t.Fatalf("second duplicate not renamed: %v", calls)
	}
	// 第一个 tool 结果跟首个 dup；第二个结果原本属于 dup-2 那条合法调用，不应被改。
	if len(results) != 2 || results[0] != "dup" || results[1] != "dup-2" {
		t.Fatalf("results=%v", results)
	}
}

// TestDedupeToolCallIDs_NoMessages 缺 messages / 空数组不 panic。
func TestDedupeToolCallIDs_NoMessages(t *testing.T) {
	for _, body := range []string{`{"model":"x"}`, `{"messages":[]}`, `{"messages":null}`} {
		if out := PrepareBodyOpt([]byte(body), false); len(out) == 0 {
			t.Fatalf("empty output for %s", body)
		}
	}
}
