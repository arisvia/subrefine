"""Local end-to-end verification of subrefine.

Drives a live subrefine instance (127.0.0.1:29999) plus a local fixture
subscription server (127.0.0.1:18080). Plain asserts, prints PASS/FAIL.

Fixture contents (7 nodes across 5 protocols + 1 duplicate + 1 junk node):
  ss JP-01-IPTelecom / ss JP-01-Duplicate / vless reality / trojan ws /
  hy2 / vmess ws / ss 公告-假节点
Clash fixture: 4 proxies (ss, trojan, vless reality, wireguard).
sing-box fixture: 3 outbounds + 2 non-proxy outbounds (direct, block).
"""
import base64
import json
import re
import urllib.error
import urllib.request

BASE = "http://127.0.0.1:29999"
FIXTURE = "http://127.0.0.1:18080/localtest/fixture"
ADMIN = "local-admin-secret-9x7"

results = []


def check(name, cond, detail=""):
    ok = bool(cond)
    results.append((name, ok))
    print(f"{'PASS' if ok else 'FAIL'}  {name}" + ("" if ok else f"  -> {detail}"))


def req(path, token=None, headers=None, method="GET", body=None):
    h = dict(headers or {})
    if token:
        h["Authorization"] = "Bearer " + token
    data = json.dumps(body).encode() if body is not None else None
    if data is not None:
        h["Content-Type"] = "application/json"
    r = urllib.request.Request(BASE + path, headers=h, method=method, data=data)
    try:
        with urllib.request.urlopen(r, timeout=20) as resp:
            return resp.status, resp.read().decode("utf-8", "replace"), dict(resp.headers)
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf-8", "replace"), dict(e.headers)


def mksub(idn, sources, pipeline, enabled=True):
    return {"id": idn, "name": "测试-" + idn, "token": "subtok-" + idn,
            "sources": sources, "pipeline": pipeline, "enabled": enabled}


def save(sub):
    return req("/api/admin/subscriptions", token=ADMIN, method="POST", body=sub)


def preview(idn):
    st, bd, _ = req(f"/api/admin/subscriptions/{idn}/nodes", token=ADMIN)
    return st, (json.loads(bd) if st == 200 else {})


# ---------------------------------------------------------------- health/auth
st, bd, _ = req("/healthz")
check("healthz 200 ok", st == 200 and '"ok"' in bd, f"{st} {bd}")

st, _, _ = req("/api/admin/info")
check("admin info rejects anonymous (401)", st == 401, str(st))
st, _, _ = req("/api/admin/info", token="wrong-token")
check("admin info rejects wrong token (401)", st == 401, str(st))
st, _, _ = req("/api/admin/info", token="local-admin-secret-9x8")
check("admin info rejects near-miss token (401)", st == 401, str(st))
st, bd, _ = req("/api/admin/info", token=ADMIN)
check("admin info accepts bearer token (200)", st == 200 and "version" in bd, f"{st} {bd}")

# --------------------------------------------------------------------- webui
st, bd, hdr = req("/")
check("console shell 200 html", st == 200 and "<!DOCTYPE html>" in bd, str(st))
check("console shell content-type", "text/html" in hdr.get("Content-Type", ""), hdr.get("Content-Type", ""))
check("console shell not cached", "no-store" in hdr.get("Cache-Control", ""), hdr.get("Cache-Control", ""))
check("console shell varies on encoding", "Accept-Encoding" in hdr.get("Vary", ""), hdr.get("Vary", ""))

chunks = re.findall(r'(?:src|href)="(/assets/[^"]+\.(?:js|css))"', bd)
check("console shell references hashed chunks", len(chunks) >= 2, bd[:200])
for asset in chunks:
    st, body, ahdr = req(asset, headers={"Accept-Encoding": "identity"})
    check(f"chunk {asset} served", st == 200 and len(body) > 0, str(st))
    check(f"chunk {asset} immutable", "immutable" in ahdr.get("Cache-Control", ""), ahdr.get("Cache-Control", ""))

st, entry, _ = req(next(c for c in chunks if c.endswith(".js")), headers={"Accept-Encoding": "identity"})
check("bundle drives the admin api", "/api/admin/subscriptions" in entry, "no admin api call in entry chunk")
st, _, brhdr = req(chunks[0], headers={"Accept-Encoding": "br, gzip"})
check("bundle negotiates precompressed sidecar", brhdr.get("Content-Encoding") in ("br", "gzip"), str(brhdr))

# ---------------------------------------------------------------------- CRUD
st, _, _ = req("/api/admin/subscriptions")
check("list subscriptions requires admin (401)", st == 401, str(st))
st, bd, _ = req("/api/admin/subscriptions", token=ADMIN)
check("list subscriptions 200 json array", st == 200 and isinstance(json.loads(bd), list), str(st))

st, _, _ = req("/api/admin/subscriptions", token=ADMIN, method="POST", body={"name": "missing-id"})
check("save rejects body without id (400)", st == 400, str(st))
st, _, _ = req("/api/admin/subscriptions", token=ADMIN, method="POST", body="{not json")
check("save rejects malformed body (400)", st == 400, str(st))

st, _, _ = save(mksub("demo", ["ss://YWVzLTI1Ni1nY206cDpwQDEuMi4zLjQ6ODM4OCAjTm9kZQ"], []))
check("save subscription 200", st == 200, str(st))
st, bd, _ = req("/api/admin/subscriptions", token=ADMIN)
check("saved sub listed", st == 200 and any(s["id"] == "demo" for s in json.loads(bd)), str(st))

st, _, _ = save(mksub("demo", ["ss://YWVzLTI1Ni1nY206cDpwQDEuMi4zLjQ6ODM4OCAjVjI"], []))
st2, d2, _ = req("/api/admin/subscriptions", token=ADMIN)
demo = next(s for s in json.loads(d2) if s["id"] == "demo")
check("upsert overwrites same id (no dup rows)",
      st == 200 and sum(1 for s in json.loads(d2) if s["id"] == "demo") == 1 and len(demo["sources"]) == 1,
      json.dumps(demo))

st, _, _ = req("/api/admin/subscriptions/demo", token=ADMIN, method="DELETE")
check("delete subscription 200", st == 200, str(st))
st, bd, _ = req("/api/admin/subscriptions", token=ADMIN)
check("deleted sub gone", all(s["id"] != "demo" for s in json.loads(bd)))
st, _, _ = req("/api/admin/subscriptions/demo", token=ADMIN, method="DELETE")
check("delete unknown id 404", st == 404, str(st))
st, _, _ = req("/api/admin/subscriptions/ghost/nodes", token=ADMIN)
check("preview unknown id 404", st == 404, str(st))

# Save-time contract: configuration delivery would silently ignore must be
# refused with the server's explanation, which the console surfaces verbatim.
st, bd, _ = save(mksub("badre", ["ss://YWVzLTI1Ni1nY206cDpwQDEuMi4zLjQ6ODM4OCAjTm9kZQ"], [
    {"type": "rename", "pattern": "([", "repl": "x"}]))
check("save rejects bad regex (400)", st == 400 and "invalid pattern" in bd, f"{st} {bd[:120]}")
st, bd, _ = save(mksub("badop", ["ss://YWVzLTI1Ni1nY206cDpwQDEuMi4zLjQ6ODM4OCAjTm9kZQ"], [{"type": "quantum"}]))
check("save rejects unknown operator (400)", st == 400 and "unknown operator" in bd, f"{st} {bd[:120]}")
st, bd, _ = save(mksub("bad types!", ["ss://YWVzLTI1Ni1nY206cDpwQDEuMi4zLjQ6ODM4OCAjTm9kZQ"], []))
check("save rejects unsafe id (400)", st == 400 and "must match" in bd, f"{st} {bd[:120]}")

# A deleted subscription must 404 on delivery immediately, not serve cached
# nodes until the TTL lapses.
st, _, _ = save(mksub("gone-soon", ["ss://YWVzLTI1Ni1nY206cDpwQDEuMi4zLjQ6ODM4OCAjTm9kZQ"], []))
st, _, _ = req("/s/gone-soon?token=subtok-gone-soon")
check("new sub delivers before delete", st == 200, str(st))
st, _, _ = req("/api/admin/subscriptions/gone-soon", token=ADMIN, method="DELETE")
check("delete gone-soon 200", st == 200, str(st))
st, _, _ = req("/s/gone-soon?token=subtok-gone-soon")
check("deleted sub delivery 403", st == 403, str(st))

# ------------------------------------------------- subscription format ingest
CASES = {
    "uri-plain": (f"{FIXTURE}/uri_plain.txt", 7),
    "uri-b64": (f"{FIXTURE}/uri_b64.txt", 7),
    "clash-yaml": (f"{FIXTURE}/clash_nodes.yaml", 4),
    "clash-yaml-b64": (f"{FIXTURE}/clash_nodes_b64.txt", 4),
    "singbox-json": (f"{FIXTURE}/singbox_nodes.json", 3),
    "singbox-json-b64": (f"{FIXTURE}/singbox_nodes_b64.txt", 3),
}
for cid, (url, expect) in CASES.items():
    st, _ = save(mksub("ing-" + cid, [url], []))[0], None
    st2, d2 = preview("ing-" + cid)
    got = d2.get("count") if st2 == 200 else None
    check(f"ingest {cid} -> {expect} nodes", got == expect, f"save={st} preview={st2} count={got}")

# ------------------------------------------------------------- protocol spans
st, d = preview("ing-uri-b64")
types = sorted({n["type"] for n in d.get("nodes", [])})
check("uri subscription spans 5 protocols",
      types == ["hysteria2", "shadowsocks", "trojan", "vless", "vmess"], str(types))
vless = next((n for n in d["nodes"] if n["type"] == "vless"), {})
check("vless reality parsed from uri",
      vless.get("security") == "reality" and vless.get("flow") == "xtls-rprx-vision"
      and vless.get("public_key") == "SbxgTPAIcKg2K4LTBfrIGQfCNbQLdGOGNwOrWPykWlU"
      and vless.get("short_id") == "6ba818e7" and vless.get("fingerprint") == "chrome",
      json.dumps(vless))
hy2 = next((n for n in d["nodes"] if n["type"] == "hysteria2"), {})
check("hy2 obfs parsed from uri", hy2.get("obfs_type") == "salamander" and hy2.get("obfs_pass") == "obfspass", json.dumps(hy2))
vm = next((n for n in d["nodes"] if n["type"] == "vmess"), {})
check("vmess ws+tls parsed from uri", vm.get("network") == "ws" and vm.get("path") == "/vray"
      and vm.get("security") == "tls" and vm.get("tag") == "TW-06-VMess", json.dumps(vm))

st, d = preview("ing-clash-yaml")
wg = next((n for n in d["nodes"] if n["type"] == "wireguard"), {})
check("clash wireguard parsed (key/ip/mtu)",
      wg.get("peer_public_key") == "B@fake" and wg.get("local_addr") == ["10.0.0.5/32"] and wg.get("mtu") == 1420,
      json.dumps(wg))

st, d = preview("ing-singbox-json")
tags = sorted(n["tag"] for n in d.get("nodes", []))
check("sing-box non-proxy outbounds filtered out",
      d.get("count") == 3 and "direct" not in tags and "block" not in tags, str(tags))

# ------------------------------------------------------------------- pipeline
pipe = [{"type": "filter", "include": "HK|JP"},
        {"type": "rename", "pattern": "^(HK|JP)-", "repl": "[$1] "},
        {"type": "deduplicate"},
        {"type": "decorate", "fingerprint": "firefox", "detour": "WARP"}]
st, _ = save(mksub("pipe1", [CASES["uri-b64"][0], CASES["clash-yaml"][0]], pipe))[0], None
st2, d2 = preview("pipe1")
nodes = d2.get("nodes", [])
check("pipeline ran (200)", st2 == 200, str(st2))
# 11 raw -> filter 5 -> dedup collapses cred-identical JP/HK pairs -> 2
check("pipeline filter+dedup yields 2 nodes", len(nodes) == 2, json.dumps([n["tag"] for n in nodes]))
check("pipeline rename applied",
      sorted(n["tag"] for n in nodes) == ["[HK] 03-TrojanWS", "[JP] 01-IPTelecom"],
      json.dumps([n["tag"] for n in nodes]))
check("pipeline dedup kept cred-identical duplicates once",
      len({(n["type"], n["server"], n["port"]) for n in nodes}) == 2, json.dumps(nodes))
check("pipeline decorate fingerprint applied",
      all(n.get("fingerprint") == "firefox" for n in nodes), json.dumps([n.get("fingerprint") for n in nodes]))
check("pipeline decorate detour applied",
      all(n.get("detour") == "WARP" for n in nodes), json.dumps([n.get("detour") for n in nodes]))

st, _, _ = save(mksub("pipe-empty", [CASES["uri-b64"][0]], [{"type": "filter", "include": "NOMATCHZZZ"}]))
st2, d2 = preview("pipe-empty")
check("filter excluding everything yields 0 nodes", st2 == 200 and d2.get("count") == 0, json.dumps(d2))

# protocol allow-list filter (Sub-Store style "type filter")
st, _ = save(mksub("pipe-type", [CASES["uri-b64"][0]], [{"type": "filter", "types": ["vless", "hysteria2"]}]))[0], None
st2, d2 = preview("pipe-type")
got = sorted(n["type"] for n in d2.get("nodes", []))
check("protocol filter keeps only vless+hy2", got == ["hysteria2", "vless"], str(got))

# sort by tag desc
st, _ = save(mksub("pipe-sort", [CASES["uri-b64"][0]], [{"type": "sort", "by": "tag", "order": "desc"}]))[0], None
st2, d2 = preview("pipe-sort")
order = [n["tag"] for n in d2.get("nodes", [])]
check("sort desc orders tags descending", order == sorted(order, reverse=True) and len(order) == 7, str(order))

# sort -> dedup -> rename ordering must be honoured as written
st, _ = save(mksub("pipe-chain", [CASES["uri-b64"][0]], [
    {"type": "sort", "by": "protocol", "order": "asc"},
    {"type": "deduplicate"},
    {"type": "rename", "pattern": "^", "repl": "#"},
]))[0], None
st2, d2 = preview("pipe-chain")
chain = [n["tag"] for n in d2.get("nodes", [])]
check("chained sort+dedup+rename stable", st2 == 200 and len(chain) == 5 and all(t.startswith("#") for t in chain), str(chain))

# ---------------------------------------------------------- subscribe + token
st, _, _ = req("/s/pipe1")
check("subscribe without token 403", st == 403, str(st))
st, _, _ = req("/s/pipe1?token=nope")
check("subscribe wrong token 403", st == 403, str(st))
st, _, _ = req("/s/ghost?token=subtok-ghost")
check("subscribe unknown id 403", st == 403, str(st))
st, bd, _ = req("/s/pipe1", token="subtok-pipe1")
check("subscribe token via Authorization header 200", st == 200, f"{st} {bd[:120]}")

st, _, _ = save(mksub("off1", [CASES["uri-plain"][0]], [], enabled=False))
st, _, _ = req("/s/off1?token=subtok-off1&target=uri")
check("disabled subscription 403", st == 403, str(st))

# ------------------------------------------------------------ format delivery
st, bd, hdr = req("/s/ing-uri-b64?token=subtok-ing-uri-b64&target=sing-box")
j = json.loads(bd) if st == 200 else {}
types = sorted({o["type"] for o in j.get("outbounds", [])})
check("target=sing-box emits outbounds for 7 nodes",
      st == 200 and len(j.get("outbounds", [])) == 7 and types == ["hysteria2", "shadowsocks", "trojan", "vless", "vmess"],
      f"{st} {types}")
vl = next((o for o in j.get("outbounds", []) if o["type"] == "vless"), {})
check("sing-box vless carries reality + vision flow + utls",
      vl.get("flow") == "xtls-rprx-vision" and vl["tls"]["reality"]["enabled"] is True
      and vl["tls"]["reality"]["public_key"].startswith("Sbxg") and vl["tls"]["utls"]["fingerprint"] == "chrome",
      json.dumps(vl))
check("sing-box content-type json", "application/json" in hdr.get("Content-Type", ""), hdr.get("Content-Type", ""))

st, bd, hdr = req("/s/ing-clash-yaml?token=subtok-ing-clash-yaml&target=clash")
check("target=clash emits proxies + proxy-groups",
      st == 200 and "proxies:" in bd and "proxy-groups:" in bd and "url-test" in bd, f"{st} {bd[:160]}")
check("clash keeps wireguard proxy", st == 200 and "type: wireguard" in bd and "public-key: B@fake" in bd, bd[:300])
check("clash content-type yaml", "yaml" in hdr.get("Content-Type", ""), hdr.get("Content-Type", ""))

st, bd, _ = req("/s/pipe1?token=subtok-pipe1&target=uri")
decoded = base64.b64decode(bd).decode() if st == 200 else ""
check("target=uri emits base64 of 2 URIs", st == 200 and decoded.count("\n") == 2 and decoded.count("://") == 2,
      f"{st} {decoded[:200]}")

st, bd, _ = req("/s/ing-uri-b64?token=subtok-ing-uri-b64", headers={"User-Agent": "sing-box/1.15.0"})
check("UA sing-box negotiates JSON", st == 200 and '"outbounds"' in bd, str(st))
st, bd, _ = req("/s/ing-uri-b64?token=subtok-ing-uri-b64", headers={"User-Agent": "ClashMi/1.19.0 (iOS 18)"})
check("UA mihomo/clash negotiates YAML", st == 200 and "proxies:" in bd and '"outbounds"' not in bd, str(st))
st, bd, _ = req("/s/ing-uri-b64?token=subtok-ing-uri-b64", headers={"User-Agent": "v2rayNG/1.10.0"})
check("UA unknown falls back to base64 URI", st == 200 and "://" in base64.b64decode(bd).decode(), str(st))
st, bd, _ = req("/s/ing-uri-b64?token=subtok-ing-uri-b64&target=sb", headers={"User-Agent": "sing-box/1.15.0"})
check("explicit target overrides UA", st == 200 and '"outbounds"' in bd, str(st))

# ------------------------------------------------------------ uri round-trip
st, bd, _ = req("/s/ing-uri-b64?token=subtok-ing-uri-b64&target=uri")
back = save(mksub("rt1", [bd], []))
st2, d2 = preview("rt1")
orig_st, orig_d = preview("ing-uri-b64")
sig = lambda ns: sorted((n["type"], n["server"], n["port"], n.get("uuid", ""), n.get("password", "")) for n in ns)
check("uri -> re-ingest preserves every node",
      st2 == 200 and d2.get("count") == 7 and sig(d2["nodes"]) == sig(orig_d["nodes"]),
      f"{d2.get('count')} vs {orig_d.get('count')}")

# ------------------------------------------------------- persistence on disk
st, bd, _ = req("/api/admin/subscriptions", token=ADMIN)
ids = sorted(s["id"] for s in json.loads(bd))
check("all created subs listed for restart check", all(x in ids for x in ("pipe1", "ing-uri-b64", "ing-clash-yaml")), str(ids))

fails = [n for n, ok in results if not ok]
print(f"\n{len(results) - len(fails)}/{len(results)} checks passed")
if fails:
    print("FAILED: " + "; ".join(fails))
    raise SystemExit(1)
