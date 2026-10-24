// autotask_expert_test.go 「点名专家」任务族（expert_use_* / WorkBuddy_Valorant）的
// 静态解析与动作登记回归。
//
// 背景（2026-10-24 实测）：这两个任务族的判据与 expert_5 同源（expert_actual_use），
// 只是要求 expert_id 等于任务点名的那位专家。任务码随活动下发，无法静态登记进
// autoActions，改由 autoActionFor 前缀匹配 + dynamicAutoActions 按列表展开——
// 本测试锁住「能查到动作」「能解出专家 id / 名称」两条前提。
package panel

import (
	"testing"

	"github.com/linguo2625469/workbuddy2api-panel/internal/upstream"
)

func TestExpertIDFromJump(t *testing.T) {
	cases := []struct {
		name string
		jump string
		want string
	}{
		{
			name: "未编码 deeplink",
			jump: "https://www.workbuddy.cn/work/launch/?deeplink=workbuddy%3A%2F%2Fexperts%3FexpertId%3Dex_FbY5nlWTqDod%26buddy_type%3Dworkbuddy",
			want: "ex_FbY5nlWTqDod",
		},
		{
			name: "明文 query",
			jump: "workbuddy://experts?expertId=ex_WZY136wV5aMt&buddy_type=workbuddy",
			want: "ex_WZY136wV5aMt",
		},
		{
			name: "无 expertId（WorkBuddy_Valorant 形态）",
			jump: "workbuddy://expert",
			want: "",
		},
		{
			name: "空串",
			jump: "",
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := expertIDFromJump(c.jump); got != c.want {
				t.Fatalf("expertIDFromJump(%q) = %q, want %q", c.jump, got, c.want)
			}
		})
	}
}

func TestQuotedTitleName(t *testing.T) {
	cases := map[string]string{
		"体验「抖音策略师」":       "抖音策略师",
		"体验「商家宣传页与海报设计师」": "商家宣传页与海报设计师",
		"体验「无畏契约拼豆专家」":    "无畏契约拼豆专家",
		"没有书名号的标题":        "",
		"未闭合「专家":          "",
	}
	for title, want := range cases {
		if got := quotedTitleName(title); got != want {
			t.Fatalf("quotedTitleName(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestAutoActionForNamedExpert(t *testing.T) {
	for _, code := range []string{"expert_use_douyinyin", "expert_use_maidehao", "WorkBuddy_Valorant"} {
		if autoActionFor(code) == nil {
			t.Fatalf("%s 应登记为可自动完成", code)
		}
		if !isNamedExpertTask(code) {
			t.Fatalf("%s 应属于点名专家族", code)
		}
	}
	if autoActionFor("no_such_task") != nil {
		t.Fatal("未知任务不应有动作")
	}
	if isNamedExpertTask("expert_5") {
		t.Fatal("expert_5 是静态任务，不应命中前缀规则")
	}
	// 队列顺序：排在 expert_5 之后（同属真实专家链）。
	if autoActionIndex("expert_use_douyinyin") != autoActionIndex("expert_5") {
		t.Fatal("点名专家任务应排在 expert_5 之后")
	}
}

func TestDynamicAutoActionsDedup(t *testing.T) {
	tasks := []upstream.Task{
		{TaskCode: "expert_use_douyinyin"},
		{TaskCode: "expert_use_douyinyin"},
		{TaskCode: "WorkBuddy_Valorant"},
		{TaskCode: "chat_5"},
	}
	acts := dynamicAutoActions(tasks)
	if len(acts) != 2 {
		t.Fatalf("dynamicAutoActions 去重后应为 2 条，得到 %d", len(acts))
	}
}
