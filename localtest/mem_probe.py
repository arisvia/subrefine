"""Measure the live subrefine process working set, idle vs loaded."""
import ctypes
import json
import re
import subprocess
import sys
import time
import urllib.error
import urllib.request


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


def working_set(pid):
    h = ctypes.windll.kernel32.OpenProcess(0x1000, False, pid)
    if not h:
        raise SystemExit("OpenProcess failed")
    pmc = PMC()
    pmc.cb = ctypes.sizeof(PMC)
    ctypes.windll.psapi.GetProcessMemoryInfo(h, ctypes.byref(pmc), pmc.cb)
    ctypes.windll.kernel32.CloseHandle(h)
    return pmc.WorkingSetSize, pmc.PeakWorkingSetSize


def get(path, headers=None):
    req = urllib.request.Request(f"http://127.0.0.1:29999{path}", headers=headers or {})
    try:
        with urllib.request.urlopen(req, timeout=20) as r:
            return r.read()
    except urllib.error.HTTPError:
        return b""  # disabled/invalid subs are expected to 403


port = 29999
pid = win_pid(port)
ws, peak = working_set(pid)
print(f"idle    : {ws/1048576:.2f} MiB (peak {peak/1048576:.2f})")

admin = {"Authorization": "Bearer local-admin-secret-9x7"}
subs = json.loads(get("/api/admin/subscriptions", admin))
print(f"subs    : {len(subs)}")

# Load: pull every subscription in every format, plus protocol-heavy paths.
for s in subs:
    for t in ("sing-box", "clash", "uri"):
        get(f"/s/{s['id']}?token={s['token']}&target={t}", {"User-Agent": "sing-box/1.15.0"})

# The embedded console is demand-loaded from the binary's read-only section, so
# a served bundle counts against the same working set as subscription data.
shell = get("/").decode("utf-8", "replace")
for asset in re.findall(r'(?:src|href)="(/assets/[^"]+\.(?:js|css))"', shell):
    get(asset)
    get(asset, {"Accept-Encoding": "br, gzip"})

ws, peak = working_set(pid)
print(f"loaded  : {ws/1048576:.2f} MiB (peak {peak/1048576:.2f})")

time.sleep(2)
ws, peak = working_set(pid)
print(f"settled : {ws/1048576:.2f} MiB (peak {peak/1048576:.2f})")
# 18 MiB gate above the measured 17.0 MiB plateau (image pages + embedded console;
# the Go live heap itself stays near 1 MiB).
print("target < 18 MiB:", "PASS" if ws < 18 else "FAIL")
