"""Leak / plateau check: hammer all endpoints N passes, sample working set each pass."""
import ctypes
import json
import os
import re
import subprocess
import time
import urllib.error
import urllib.request

BASE = "http://127.0.0.1:29999"
ADMIN = {"Authorization": "Bearer local-admin-secret-9x7"}


class PMC(ctypes.Structure):
    _fields_ = [("cb", ctypes.c_uint32), ("PageFaultCount", ctypes.c_uint32),
                ("PeakWorkingSetSize", ctypes.c_size_t), ("WorkingSetSize", ctypes.c_size_t),
                ("QuotaPeakPagedPoolUsage", ctypes.c_size_t), ("QuotaPagedPoolUsage", ctypes.c_size_t),
                ("QuotaPeakNonPagedPoolUsage", ctypes.c_size_t), ("QuotaNonPagedPoolUsage", ctypes.c_size_t),
                ("PagefileUsage", ctypes.c_size_t), ("PeakPagefileUsage", ctypes.c_size_t)]


def win_pid(port):
    out = subprocess.check_output(["netstat", "-ano"], text=True, errors="replace")
    for line in out.splitlines():
        if f"127.0.0.1:{port}" in line and "LISTENING" in line:
            return int(line.split()[-1])
    raise SystemExit(f"no listener on {port}")


def ws_bytes(pid):
    h = ctypes.windll.kernel32.OpenProcess(0x1000, False, pid)
    pmc = PMC()
    pmc.cb = ctypes.sizeof(PMC)
    ctypes.windll.psapi.GetProcessMemoryInfo(h, ctypes.byref(pmc), pmc.cb)
    ctypes.windll.kernel32.CloseHandle(h)
    # Working set includes demand-loaded embedded chunks that Windows declines
    # to trim; private bytes show whether the process is actually growing.
    return pmc.WorkingSetSize, pmc.PagefileUsage


def sample(pid):
    ws, priv = ws_bytes(pid)
    return f"{ws / 1048576:.2f} MiB (private {priv / 1048576:.2f})"


def get(path, headers=None):
    req = urllib.request.Request(BASE + path, headers=headers or {})
    try:
        with urllib.request.urlopen(req, timeout=20) as r:
            return r.read()
    except urllib.error.HTTPError:
        return b""


PASSES = int(os.environ.get("SR_PASSES", "20"))
ROUNDS = int(os.environ.get("SR_ROUNDS", "1"))
TRAFFIC = os.environ.get("SR_TRAFFIC", "both")  # subs | console | both
IDLE = float(os.environ.get("SR_IDLE", "3"))

pid = win_pid(29999)
subs = json.loads(get("/api/admin/subscriptions", ADMIN))
enabled = [s for s in subs if s.get("enabled")]
shell = get("/").decode("utf-8", "replace")
chunks = re.findall(r'(?:src|href)="(/assets/[^"]+\.(?:js|css))"', shell)
print(f"pid={pid} subs={len(enabled)} chunks={len(chunks)} traffic={TRAFFIC} passes={PASSES} rounds={ROUNDS}")
print(f"baseline        : {sample(pid)}")


def one_pass():
    if TRAFFIC in ("subs", "both"):
        for s in enabled:
            for t in ("sing-box", "clash", "uri"):
                get(f"/s/{s['id']}?token={s['token']}&target={t}")
            get(f"/s/{s['id']}?token={s['token']}", {"User-Agent": "sing-box/1.15.0"})
            get(f"/s/{s['id']}?token={s['token']}", {"User-Agent": "ClashMi/1.19"})
        get("/api/admin/subscriptions", ADMIN)
    if TRAFFIC in ("console", "both"):
        get("/")
        for asset in chunks:
            get(asset)


# A second round answers the bounded-vs-leak question: a warm-up ceiling is
# re-entered flat, growth continues from wherever the first round stopped.
for r in range(1, ROUNDS + 1):
    for i in range(1, PASSES + 1):
        one_pass()
        time.sleep(0.4)
        print(f"round {r} pass {i:<3}  : {sample(pid)}")
    time.sleep(IDLE)
    print(f"round {r} idle     : {sample(pid)}")
