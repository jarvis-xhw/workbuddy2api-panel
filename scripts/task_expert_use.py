#!/usr/bin/env python3
"""expert_use_* 体验任务闭环：点名专家召唤 + 真实对话（桌面指纹）。

任务族（2026-10 新下发，growth 域默认口径）：
  expert_use_<slug>「体验「XXX」」  在专家页搜索并召唤「XXX」，完成一次对话。
  target=1 / +50 分 +5 能

判据（与 expert_5 同源，仅专家 id 绑定）：
  1) 专家市场拉真实 expert_id（任务 jump_url 的 deeplink 里已带 ex_ id，市场按
     keyword 复核名称/版本/分类，事件载荷才与真实召唤样本一致）；
  2) 召唤链 expert_summon_click + expert_summoned（+ web_element_click）；
  3) 真实对话 POST /v2/chat/completions（X-Expert-Id）拿**服务端 requestId**；
  4) expert_actual_use（JOIN 服务端 requestId）+ 完整 chat 事件链 → 桌面指纹
     POST /v2/report；
  5) 回读进度 → Web 域领奖。

用法：
  python3 task_expert_use.py 5edbf674                 # 预演（不写）
  python3 task_expert_use.py 5edbf674 --yes           # 全部 expert_use_* 一次跑完
  python3 task_expert_use.py 5edbf674 --yes --only expert_use_douyinyin
  python3 task_expert_use.py ALL --yes --gap 10       # 全池（每账号串行）

环境：
  WB2A_AUTHS   auths 目录（默认 /root/workbuddy2api/auths）
"""
import argparse
import glob
import hashlib
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request

AUTHS = os.environ.get("WB2A_AUTHS", "/root/workbuddy2api/auths")
CHAT_BASE = "https://copilot.tencent.com"
WEB_BASE = "https://www.workbuddy.cn"
CLI_UA = "CLI/2.63.2 CodeBuddy/2.63.2"
DESKTOP_UA = "WorkBuddy/5.5.6 WorkBuddy/5.5.6 CLI/2.137.1"
ID_RE = re.compile(r"^(cmb-)?[0-9a-f]{32}$")

# 需点名专家但 code 不在 expert_use_ 前缀下的活动任务（jump_url 只有 workbuddy://expert，
# 专家按标题书名号里的名字在专家市场反查）。
EXTRA_EXPERT_TASKS = {"WorkBuddy_Valorant"}


# ---------------------------------------------------------------- 账号 / 传输

def load_auths(selector):
    files = sorted(glob.glob(os.path.join(AUTHS, "workbuddy-*.json")))
    out = []
    for p in files:
        d = json.load(open(p, encoding="utf-8"))
        a, acc = d.get("auth", {}), d.get("account", {})
        uid = acc.get("uid", "")
        if selector != "ALL" and not uid.startswith(selector):
            continue
        out.append({
            "token": a.get("accessToken", ""),
            "domain": a.get("domain") or "",
            "uid": uid,
            "nick": acc.get("nickname", ""),
            "file": os.path.basename(p),
        })
    return out


def call(method, url, body=None, headers=None, timeout=40, raw=False):
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(url, data=data, headers=headers or {}, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            payload = r.read()
            if raw:
                return r.status, payload
            try:
                return r.status, json.loads(payload.decode("utf-8", "replace"))
            except Exception:
                return r.status, {"raw": payload[:300].decode("utf-8", "replace")}
    except urllib.error.HTTPError as e:
        t = e.read().decode("utf-8", "replace")
        try:
            return e.code, json.loads(t)
        except Exception:
            return e.code, {"raw": t[:400]}
    except Exception as e:  # 网络层错误统一返回 -1
        return -1, {"err": repr(e)[:300]}


def cli_headers(c):
    h = {
        "Authorization": "Bearer " + c["token"],
        "Accept": "application/json",
        "Content-Type": "application/json",
        "User-Agent": CLI_UA,
        "Origin": "https://www.codebuddy.cn",
        "Referer": "https://www.codebuddy.cn/",
        "X-User-Id": c["uid"],
    }
    if c["domain"]:
        h["X-Domain"] = c["domain"]
    return h


def web_headers(c):
    h = {
        "Authorization": "Bearer " + c["token"],
        "Accept": "application/json, text/plain, */*",
        "Content-Type": "application/json",
        "Origin": WEB_BASE,
        "Referer": WEB_BASE + "/profile/growth-center",
        "x-client-platform": "web",
        "X-User-Id": c["uid"],
    }
    return h


def desktop_headers(c):
    h = {
        "Authorization": "Bearer " + c["token"],
        "Accept": "application/json, text/plain, */*",
        "Content-Type": "application/json;charset=UTF-8",
        "User-Agent": DESKTOP_UA,
        "X-Domain": CHAT_BASE,
        "X-Product": "SaaS",
        "X-Request-ID": derive(c, "req") + str(int(time.time() * 1e6) % 1000000),
        "X-User-Id": c["uid"],
    }
    return h


def derive(c, salt):
    return hashlib.sha256((salt + ":" + c["uid"]).encode()).hexdigest()[:36]


# ---------------------------------------------------------------- growth 域

def list_tasks(c):
    st, d = call("GET", CHAT_BASE + "/v2/activity/growth/tasks", headers=cli_headers(c))
    if st != 200:
        raise RuntimeError(f"list_tasks http={st} {d}")
    return ((d.get("data") or {}).get("tasks")) or []


def task_of(c, code):
    for t in list_tasks(c):
        if t.get("task_code") == code:
            prog = t.get("progress") or {}
            cur = prog.get("current", t.get("current", 0)) or 0
            tgt = prog.get("target", t.get("target", 0)) or 0
            return {
                "code": code,
                "title": t.get("title", ""),
                "desc": t.get("task_desc") or t.get("description") or "",
                "jump": t.get("jump_url", ""),
                "current": int(cur),
                "target": int(tgt),
                "accept": t.get("accept_status", ""),
                "status": t.get("status", ""),
                "credit": t.get("reward_credit", 0),
                "energy": t.get("reward_energy", 0),
            }
    return None


def claim(c, code):
    st, d = call("POST", WEB_BASE + "/activity/growth/tasks/" + code + "/claim", headers=web_headers(c))
    if st != 200:
        return 0, 0, f"claim http={st} {d}"
    data = d.get("data") or {}
    if data.get("already_claimed"):
        return 0, 0, "already_claimed"
    return int(data.get("credit", 0)), int(data.get("energy", 0)), ""


# ---------------------------------------------------------------- 专家市场

def expert_id_from_jump(jump):
    """从任务 jump_url 的 deeplink 解析 expertId（workbuddy://experts?expertId=ex_xxx）。"""
    if "expertId%3D" in jump:
        seg = jump.split("expertId%3D", 1)[1]
        return seg.split("%26", 1)[0].split("&", 1)[0]
    if "expertId=" in jump:
        seg = jump.split("expertId=", 1)[1]
        return seg.split("&", 1)[0]
    return ""


def market_search(c, keyword, want_id=""):
    body = {"page": 1, "page_size": 50, "keyword": keyword, "expert_type": "agent"}
    st, d = call("POST", CHAT_BASE + "/portal/operation-platform/market/expert/list",
                 body=body, headers=desktop_headers(c))
    if st != 200:
        return None, f"market http={st} {str(d)[:160]}"
    experts = d.get("experts") or (d.get("data") or {}).get("experts") or []
    if want_id:
        for e in experts:
            if e.get("expert_id") == want_id:
                return e, ""
    if not experts:
        return None, "market 无结果"
    # 名称完全匹配兜底（任务标题与专家名一致）
    for e in experts:
        if e.get("display_name_zh") == keyword:
            return e, ""
    return experts[0], ""


# ---------------------------------------------------------------- 桌面事件

def fingerprint(c):
    now = int(time.time() * 1000)
    return {
        "timezone": "Asia/Shanghai", "reportDelay": 2000,
        "userId": c["uid"], "username": c["nick"], "userNickname": c["nick"],
        "product": "SaaS", "releaseDate": 1789036585355,
        "commit": "5f9692923c93033111c51ad7b003eb80204a9b75",
        "ideName": "WorkBuddy", "ideType": "WorkBuddy", "ideVersion": "5.5.6",
        "machineId": derive(c, "machine"), "sessionId": derive(c, "session"),
        "extName": "workbuddy-desktop", "extVersion": "5.5.6",
        "os": "win32", "arch": "x64", "osVersion": "10.0.26220",
        "cpuCores": 20, "memorySize": 24,
        "timestamp": now, "presentAt": now,
    }


def report_desktop(c, events):
    fp = fingerprint(c)
    arr = []
    for ev in events:
        m = dict(fp)
        m.update(ev)
        arr.append(m)
    st, d = call("POST", CHAT_BASE + "/v2/report", body=arr, headers=desktop_headers(c))
    if st != 200:
        return f"report http={st} {str(d)[:200]}"
    return ""


def cat_of(e):
    cats = e.get("categories") or []
    if cats and isinstance(cats[0], str):
        return cats[0]
    return "expert-all"


def summon_events(e):
    ver = e.get("version") or "1.0.0"
    return [
        {"eventCode": "web_element_click", "source": e.get("expert_id", ""),
         "type": cat_of(e), "version": ver, "elementId": "expert_summon_click",
         "elementName": "立即召唤",
         "pageURL": "/C:/Program%20Files/WorkBuddy/resources/app.asar/renderer/index.html"},
        {"eventCode": "expert_summon_click", "id": e.get("expert_id", ""),
         "name": e.get("display_name_zh", ""), "expertTitle": e.get("profession_zh", ""),
         "type": "expert-all", "position": 0, "expertType": e.get("expert_type", "agent"),
         "version": ver, "mode": "LOCAL"},
        {"eventCode": "expert_summoned", "id": e.get("expert_id", ""),
         "name": e.get("display_name_zh", ""), "expertTitle": e.get("profession_zh", ""),
         "type": "expert-all"},
    ]


def actual_use_event(e, conv, req):
    ver = e.get("version") or "1.0.0"
    return {
        "eventCode": "expert_actual_use",
        "id": e.get("expert_id", ""), "name": e.get("display_name_zh", ""),
        "expertTitle": e.get("profession_zh", ""), "type": cat_of(e),
        "expertType": e.get("expert_type", "agent"), "source": "builtin", "version": ver,
        "cost": 9000, "characterCount": 14,
        "conversationId": conv, "requestId": req, "messageId": "msg-" + req[-8:],
        "requestModelId": "fast-model", "requestModelName": "fast-model", "mode": "craft",
    }


def chat_sequence(conv, req, msg_id, model="fast-model"):
    now = int(time.time() * 1000)
    base = {"conversationId": conv, "rootRequestId": req, "parentConversationId": conv,
            "agentName": "cli", "agentType": "main"}
    j = lambda **kw: {**base, **kw}
    return [
        {"eventCode": "agent_task_created", "source": "LOCAL", "name": "working",
         "task_target": "local", "mode": "craft", "requestModelId": model,
         "requestModelName": model, "has_repo": False, "repo_type": "none",
         "workspace_type": "empty", "has_connector": False, "connector_types": [],
         "has_mention": False, "mention_types": [], "has_template": False,
         "action": "", "template_name": "", "has_expert": True,
         "expert_id": "", "expert_name": "", "expert_industry_id": "",
         "has_skill": False, "skill_names": [], "conversationId": conv,
         "messageId": msg_id, "buddyId": "", "buddyName": ""},
        j(**{"eventCode": "chat_message_send", "messageId": msg_id + "-assistant",
             "historyCount": 0, "isContextTruncated": False, "currentStepCount": 1,
             "traceId": req}),
        j(**{"eventCode": "chat_request_send", "inputLength": 24, "isPlan": False,
             "isAutoExecuteTerminal": False, "isAutoModify": False, "codebaseEnable": False,
             "maxToken": 0, "maxSteps": 500, "temperature": 0, "maxRetries": 0,
             "mentionContexts": [], "knowledgeId": [], "knowledgeName": [], "codebaseId": "",
             "mentionContextCount": 0, "command": "", "recommendId": "", "skillId": "",
             "skillCount": 0, "totalCount": 0, "traceId": req,
             "codebuddy.session_id": conv, "codebuddy.conversation_request_id": req}),
        j(**{"eventCode": "chat_message_response", "messageId": msg_id + "-assistant",
             "responseModelId": model, "inputToken": 120, "outputToken": 80, "totalToken": 200,
             "cachedTokens": 0, "cachedWriteTokens": 0, "cachedMissTokens": 0,
             "isSuccessful": True, "messageErrorCode": "", "finishReason": "stop",
             "firstTokenAt": now, "traceId": req, "conversationId": conv,
             "codebuddy.session_id": conv, "codebuddy.conversation_request_id": req}),
        j(**{"eventCode": "chat_message_status", "messageId": msg_id + "-assistant",
             "messageErrorCode": "0", "traceId": req}),
        j(**{"eventCode": "chat_request_response", "mode": "craft", "toolCallCount": 0,
             "inputToken": 120, "outputToken": 80, "totalToken": 200, "cachedTokens": 0,
             "cachedWriteTokens": 0, "cachedMissTokens": 0, "isSuccessful": True,
             "messageErrorCode": "", "finishReason": "stop"}),
    ]


# ---------------------------------------------------------------- 真实专家对话

def chat_with_expert(c, expert_id, prompt="你好，请用一句话介绍你自己。"):
    conv = "wb2api-conv-%d" % time.time_ns()
    body = {
        "model": "fast-model",
        "messages": [
            {"role": "system", "content": "You are a helpful assistant. 当前处于中文环境，使用简体中文回答。"},
            {"role": "user", "content": prompt},
        ],
        "agent": "cli", "temperature": 1, "stream": True,
        "stream_options": {"include_usage": True},
    }
    h = {
        "Authorization": "Bearer " + c["token"],
        "Content-Type": "application/json",
        "Accept": "text/event-stream",
        "User-Agent": DESKTOP_UA,
        "X-Domain": CHAT_BASE,
        "X-Product": "SaaS",
        "X-User-Id": c["uid"],
        "X-Conversation-ID": conv,
        "X-Request-ID": str(time.time_ns()),
        "X-Agent-Intent": "craft", "X-Agent-Type": "main",
        "X-IDE-Name": "WorkBuddy", "X-IDE-Type": "WorkBuddy", "X-IDE-Version": "5.5.6",
        "x-codebuddy-request": "1",
    }
    if expert_id:
        h["X-Expert-Id"] = expert_id
    req = urllib.request.Request(CHAT_BASE + "/v2/chat/completions",
                                 data=json.dumps(body).encode(), headers=h, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=90) as r:
            if r.status != 200:
                return "", "", f"chat http={r.status}"
            buf = b""
            from_pos = 0
            while True:
                chunk = r.read(8192)
                if not chunk:
                    break
                buf += chunk
                while True:
                    i = buf.find(b'"id":"', from_pos)
                    if i < 0:
                        break
                    rest = buf[i + 6:]
                    end = rest.find(b'"')
                    if end <= 0:
                        break
                    cand = rest[:end].decode("utf-8", "replace")
                    if ID_RE.match(cand):
                        return conv, cand, ""
                    from_pos = i + 1
                if len(buf) > 1 << 20:
                    break
            return "", "", "SSE 未返回服务端 requestId"
    except urllib.error.HTTPError as e:
        return "", "", f"chat http={e.code} {e.read()[:200]}"
    except Exception as e:
        return "", "", f"chat err {e!r}"[:200]


# ---------------------------------------------------------------- 单任务闭环

def run_one(c, code, dry=False, gap=8.0, poll=8):
    t = task_of(c, code)
    if t is None:
        return f"{code}: 任务未下发"
    if t["accept"] == "claimed":
        return f"{code}: 已领取"
    if t["target"] > 0 and t["current"] >= t["target"]:
        cr, en, err = claim(c, code)
        return f"{code}: 已达标 → 领奖 {('+%dc +%de' % (cr, en)) if not err else err}"
    eid = expert_id_from_jump(t["jump"])
    name = ""
    if "「" in t["title"]:
        name = t["title"].split("「", 1)[1].split("」", 1)[0]
    if dry:
        return f"{code}: 预演 expert={eid} name={name} today={t['current']}/{t['target']}"
    e, warn = market_search(c, name or code, eid)
    if e is None:
        return f"{code}: 市场未找到专家 {eid or name}（{warn}）"
    if eid and e.get("expert_id") != eid:
        return f"{code}: 市场 id 不匹配（任务 {eid} vs 市场 {e.get('expert_id')}）"
    if not eid and name and e.get("display_name_zh") != name:
        return f"{code}: 市场名称不匹配（任务「{name}」vs 市场「{e.get('display_name_zh')}」）"
    eid = e.get("expert_id", "")

    err = report_desktop(c, summon_events(e))
    if err:
        return f"{code}: 召唤链上报失败 {err}"
    time.sleep(min(gap, 5.0))
    conv, req, err = chat_with_expert(c, eid)
    if err:
        return f"{code}: 真实对话失败 {err}"
    time.sleep(2.0)
    events = chat_sequence(conv, req, "msg-" + req[-8:]) + [actual_use_event(e, conv, req)]
    err = report_desktop(c, events)
    if err:
        return f"{code}: 使用事件上报失败 {err}"

    for i in range(poll):
        time.sleep(4)
        t2 = task_of(c, code)
        if t2 is None:
            continue
        if t2["accept"] == "claimed":
            return f"{code}: 已领取（{t2['current']}/{t2['target']}）"
        if t2["target"] > 0 and t2["current"] >= t2["target"]:
            cr, en, cerr = claim(c, code)
            if cerr:
                return f"{code}: 达标但领奖失败 {cerr}"
            return f"{code}: 点亮并领奖 +{cr}c +{en}e"
    return f"{code}: 已上报但进度未点亮（异步计分，稍后复查）"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("selector", help="uid 前缀 / ALL")
    ap.add_argument("--only", default="", help="只跑指定 task_code")
    ap.add_argument("--yes", action="store_true", help="真正写入（默认预演）")
    ap.add_argument("--gap", type=float, default=8.0, help="召唤链与对话之间的间隔秒")
    ap.add_argument("--poll", type=int, default=8, help="进度回读轮数（每轮 4s）")
    args = ap.parse_args()

    accounts = load_auths(args.selector)
    if not accounts:
        raise SystemExit("没有匹配的账号")
    for c in accounts:
        print("=" * 70)
        print(f"账号 {c['nick']} ({c['uid']})")
        try:
            tasks = list_tasks(c)
        except Exception as e:
            print("  任务列表失败:", e)
            continue
        codes = []
        for t in tasks:
            code = t.get("task_code", "")
            if args.only and code != args.only:
                continue
            if (code.startswith("expert_use_") or code in EXTRA_EXPERT_TASKS) \
                    and t.get("accept_status") != "claimed":
                codes.append(code)
        if not codes:
            print("  没有待完成的 expert_use_* 任务")
            continue
        print("  待完成:", ", ".join(codes))
        for i, code in enumerate(codes):
            print("  -", run_one(c, code, dry=not args.yes, gap=args.gap, poll=args.poll))
            if i < len(codes) - 1:
                time.sleep(args.gap)


if __name__ == "__main__":
    main()
