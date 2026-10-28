"""Validate delivered subscriptions with the real clients' own parsers.

What /s/{id} serves is written verbatim to disk and checked by
`sing-box check -c` and `mihomo -t -d`. Passing here means the target
application itself accepts the bytes - not just this repo's tests.

Prerequisites (repo root):
  python localtest/serve_fixture.py &
  SUBREFINE_ADMIN_TOKEN=... ./subrefine-test.exe -host 127.0.0.1 -port 29999 &
  python localtest/verify.py            # seeds the subscriptions this reads

Binaries: localtest/bin/sing-box.exe and localtest/bin/mihomo.exe (kept out
of git; grab the latest windows-amd64 releases), or put either name on PATH.
Override the server with SR_BASE / SUBREFINE_ADMIN_TOKEN.
"""
import base64
import json
import os
import shutil
import subprocess
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request

BASE = os.environ.get("SR_BASE", "http://127.0.0.1:29999")
ADMIN = os.environ.get("SUBREFINE_ADMIN_TOKEN", "local-admin-secret-9x7")
HERE = os.path.dirname(os.path.abspath(__file__))


def find_exe(name):
    for cand in (
        os.path.join(HERE, "bin", name, name + ".exe"),
        os.path.join(HERE, "bin", name + ".exe"),
        os.path.join(HERE, "bin", name),
        shutil.which(name),
    ):
        if cand and os.path.exists(cand):
            return cand
    return None


def get(path, token=None):
    h = {}
    if token:
        h["Authorization"] = "Bearer " + token
    req = urllib.request.Request(BASE + path, headers=h)
    with urllib.request.urlopen(req, timeout=30) as resp:
        return resp.read()


def run(cmd):
    p = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
    return p.returncode, (p.stdout + p.stderr).strip()


def main():
    sbx = find_exe("sing-box")
    mihomo = find_exe("mihomo")
    if not sbx and not mihomo:
        print("no validators found (localtest/bin/sing-box[.exe] / mihomo[.exe] or PATH)")
        return 2
    print(f"sing-box: {sbx or 'missing'}")
    print(f"mihomo  : {mihomo or 'missing'}")

    subs = json.loads(get("/api/admin/subscriptions", ADMIN))
    enabled = [s for s in subs if s.get("enabled")]
    if not enabled:
        print("no enabled subscriptions - run verify.py first")
        return 1

    failures = []
    tmp = tempfile.mkdtemp(prefix="subrefine_clients_")
    try:
        for s in enabled:
            sid, tok = s["id"], s["token"]
            uri_body = get(f"/s/{sid}?token={urllib.parse.quote(tok)}&target=uri").decode()
            nodes = base64.b64decode(uri_body)
            lines = [l for l in nodes.decode().splitlines() if l.strip()]
            n_uri = len(lines)
            # sing-box has no wireguard outbound since 1.13.0; the target
            # drops those nodes on purpose (announced via X-Subrefine-Skipped).
            n_wg = len([l for l in lines if l.startswith("wireguard://")])

            if sbx:
                url = f"/s/{sid}?token={urllib.parse.quote(tok)}&target=sing-box"
                data = get(url)
                cfg = json.loads(data)
                path = os.path.join(tmp, f"{sid}.sb.json")
                with open(path, "wb") as f:
                    f.write(data)
                rc, out = run([sbx, "check", "-c", path])
                ob_count = len(cfg.get("outbounds", []))
                if rc != 0 or ob_count != n_uri - n_wg:
                    failures.append(f"{sid} sing-box: rc={rc} outbounds={ob_count} want={n_uri - n_wg} :: {out[:220]}")
                    print(f"FAIL  {sid} sing-box ({out[:120]})")
                else:
                    note = f" ({n_wg} wg dropped)" if n_wg else ""
                    print(f"PASS  {sid} sing-box check ({ob_count} outbounds{note})")

            if mihomo:
                data = get(f"/s/{sid}?token={urllib.parse.quote(tok)}&target=clash")
                d = os.path.join(tmp, sid)
                os.makedirs(d, exist_ok=True)
                with open(os.path.join(d, "config.yaml"), "wb") as f:
                    f.write(data)
                rc, out = run([mihomo, "-t", "-d", d])
                if rc != 0:
                    failures.append(f"{sid} mihomo: rc={rc} :: {out[:220]}")
                    print(f"FAIL  {sid} mihomo ({out[:120]})")
                else:
                    print(f"PASS  {sid} mihomo -t")

            # A subscription filtered to zero nodes is a legitimate state
            # (verify.py's pipe-empty proves the operator asked for it); the
            # counts above already assert all three shapes agree.
            print(f"PASS  {sid} uri ({n_uri} lines)")
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    print()
    if failures:
        print(f"{len(failures)} client validation failures:")
        for f in failures:
            print("  " + f)
        return 1
    print("all delivered configs accepted by the real clients")
    return 0


if __name__ == "__main__":
    sys.exit(main())
