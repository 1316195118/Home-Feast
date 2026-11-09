#!/usr/bin/env python3
# 从 gancook (/dishes RSC flight) 稳健解析全部菜品。
# 关键修复：flight 被拆成多个 self.__next_f.push([1,"..."]) 分片，单个菜品对象可能跨分片，
# 必须先把所有分片的内层字符串按序拼接成完整 payload，再提取菜品对象。
import json, re, os

HTML = r"C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/scripts/_src_dishes.html"
OUT = r"C:/Users/Administrator/WorkBuddy AI/家庭点菜系统/scripts/_src_dishes.json"

# 取每个 push 分片的内层字符串（裸引号开头/结尾，内层引号都是 \" 转义）
CHUNK = re.compile(r'self\.__next_f\.push\(\[\d+,\s*"(.*?)(?<!\\)"\]\)', re.DOTALL)
IDANCHOR = re.compile(r'\\"id\\":\\"([0-9a-fA-F-]{36})\\"')

def reassemble(text):
    chunks = CHUNK.findall(text)
    return "".join(chunks)

def unescape(s):
    return s.replace('\\\\', '\\').replace('\\"', '"')

def find_object(text, anchor):
    n = len(text)
    i = anchor; in_str = True; depth = 0; open_idx = -1
    while i >= 0:
        c = text[i]; nx = text[i+1] if i+1 < n else ''
        if c == '\\' and nx == '"':
            in_str = not in_str; i -= 1; continue
        if in_str: i -= 1; continue
        if c == '}': depth += 1; i -= 1; continue
        if c == '{':
            depth -= 1
            if depth == -1: open_idx = i; break
            i -= 1; continue
        i -= 1; continue
    if open_idx < 0: return None
    i = anchor; in_str = False; depth = 0; close_idx = -1
    while i < n:
        c = text[i]; nx = text[i+1] if i+1 < n else ''
        if c == '\\' and nx == '"':
            in_str = not in_str; i += 2; continue
        if in_str: i += 1; continue
        if c == '{': depth += 1; i += 1; continue
        if c == '}':
            depth -= 1
            if depth == -1: close_idx = i; break
            i += 1; continue
        i += 1; continue
    if close_idx < 0: return None
    return text[open_idx:close_idx+1]

def main():
    html = open(HTML, encoding='utf-8', errors='replace').read()
    payload = reassemble(html)
    print("分片数:", len(CHUNK.findall(html)), " payload 长度:", len(payload))
    dishes = []; seen = set(); fail = 0
    for m in IDANCHOR.finditer(payload):
        uid = m.group(1)
        if uid in seen: continue
        seen.add(uid)
        sub = find_object(payload, m.start())
        if not sub:
            fail += 1; continue
        try:
            obj = json.loads(unescape(sub))
        except Exception:
            fail += 1; continue
        dishes.append({
            "id": obj.get("id"),
            "name": obj.get("name"),
            "description": obj.get("description") or "",
            "imagePath": obj.get("imagePath") or "",
            "tags": obj.get("tags") or [],
        })
    print("解析菜品:", len(dishes), " 失败:", fail)
    json.dump(dishes, open(OUT, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    print("已写出:", OUT)
    no_img = [d["name"] for d in dishes if not d["imagePath"]]
    print("缺图:", no_img)
    print("菜名清单:", "、".join(d["name"] for d in dishes))

if __name__ == "__main__":
    main()
