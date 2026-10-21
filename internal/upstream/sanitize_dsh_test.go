package upstream

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNeutralizeDSHIdentity_Basic 关键词中性化：插入零宽空格后原文案不再出现。
func TestNeutralizeDSHIdentity_Basic(t *testing.T) {
	in := "You are an AI agent powered by DeepSeek Harness."
	out := neutralizeDSHIdentity(in)
	if out == in {
		t.Fatalf("not neutralized: %q", out)
	}
	if strings.Contains(out, "DeepSeek Harness") {
		t.Fatalf("literal phrase survived: %q", out)
	}
	if !strings.Contains(out, "DeepSeek\u200b Harness") {
		t.Fatalf("expected ZWSP form, got %q", out)
	}
	// 语义等价：去掉零宽字符后应与原文一致。
	if strings.ReplaceAll(out, "\u200b", "") != in {
		t.Fatalf("semantics changed: %q -> %q", in, out)
	}
}

// TestNeutralizeDSHIdentity_PathUntouched 路径语境必须原样保留，否则工具调用会拿到坏路径。
func TestNeutralizeDSHIdentity_PathUntouched(t *testing.T) {
	for _, in := range []string{
		`D:\10_COD\DeepSeek Harness\resources\app.asar\dsh\`,
		`/opt/DeepSeek Harness/bin/dsh`,
		`C:\Users\me\DeepSeek Harness\config`,
	} {
		if out := neutralizeDSHIdentity(in); out != in {
			t.Fatalf("path rewritten: %q -> %q", in, out)
		}
	}
}

// TestNeutralizeDSHIdentity_CaseAndNBSP 大小写与 NBSP 变体同样覆盖。
func TestNeutralizeDSHIdentity_CaseAndNBSP(t *testing.T) {
	for _, in := range []string{
		"powered by deepseek harness",
		"POWERED BY DEEPSEEK HARNESS",
		"powered by DeepSeek\u00a0Harness",
	} {
		out := neutralizeDSHIdentity(in)
		if out == in {
			t.Fatalf("variant not neutralized: %q", in)
		}
	}
}

// TestNeutralizeDSHIdentity_Idempotent 重复执行不得继续插入零宽字符。
func TestNeutralizeDSHIdentity_Idempotent(t *testing.T) {
	once := neutralizeDSHIdentity("powered by DeepSeek Harness")
	twice := neutralizeDSHIdentity(once)
	if once != twice {
		t.Fatalf("not idempotent:\n once=%q\ntwice=%q", once, twice)
	}
}

// TestNeutralizeDSHIdentity_UnrelatedUnchanged 不含关键词的文本零改动。
func TestNeutralizeDSHIdentity_UnrelatedUnchanged(t *testing.T) {
	for _, in := range []string{"hello world", "DeepSeek-V3 is a model", "Harness the power", ""} {
		if out := neutralizeDSHIdentity(in); out != in {
			t.Fatalf("unrelated text changed: %q -> %q", in, out)
		}
	}
}

// TestSanitizePipeline_DSHIdentity 全链路：sanitize=true 时 system 消息被中和；
// sanitize=false（逃生门）时一个字节都不动。
func TestSanitizePipeline_DSHIdentity(t *testing.T) {
	body := `{"model":"deepseek-v4.1-flash","messages":[{"role":"system","content":"You are an AI agent powered by DeepSeek Harness."},{"role":"user","content":"hi"}]}`
	out := PrepareBodyOpt([]byte(body), true)
	if strings.Contains(string(out), "DeepSeek Harness") {
		t.Fatalf("sanitize=true left the phrase: %s", out)
	}
	var obj struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(obj.Messages[0].Content, "\u200b") {
		t.Fatalf("system message not neutralized: %q", obj.Messages[0].Content)
	}

	raw := PrepareBodyOpt([]byte(body), false)
	if !strings.Contains(string(raw), "DeepSeek Harness") {
		t.Fatalf("sanitize=false must pass through verbatim: %s", raw)
	}
}
