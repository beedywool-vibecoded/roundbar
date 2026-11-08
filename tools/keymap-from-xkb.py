#!/usr/bin/env python3
"""Print a shared/keymaps.json entry for an XKB layout, read from the local xkeyboard-config.

    tools/keymap-from-xkb.py de "German"
    tools/keymap-from-xkb.py ch "Swiss French" fr    # layout, name, variant

Needs xkbcli (libxkbcommon-tools). Review the output, paste it into shared/keymaps.json,
then run `go run ./tools/gen-keymaps` to regenerate the firmware and configurator tables.
"""
import ctypes, ctypes.util, json, re, subprocess, sys

# Physical keys we can send, by XKB name (US positions; HID usages are in internal/keymap).
KEYS = (["TLDE"] + [f"AE{i:02}" for i in range(1, 13)] + [f"AD{i:02}" for i in range(1, 13)]
        + [f"AC{i:02}" for i in range(1, 12)] + ["BKSL", "LSGT"] + [f"AB{i:02}" for i in range(1, 11)])
DEAD = {"acute", "grave", "circumflex", "tilde", "diaeresis", "abovering", "cedilla", "caron"}

xkb = ctypes.CDLL(ctypes.util.find_library("xkbcommon") or "libxkbcommon.so.0")
xkb.xkb_keysym_from_name.restype = ctypes.c_uint32
xkb.xkb_keysym_from_name.argtypes = [ctypes.c_char_p, ctypes.c_int]
xkb.xkb_keysym_to_utf32.restype = ctypes.c_uint32
xkb.xkb_keysym_to_utf32.argtypes = [ctypes.c_uint32]


def token(sym):
    if sym.startswith("dead_"):
        return sym if sym[5:] in DEAD else ""
    cp = xkb.xkb_keysym_to_utf32(xkb.xkb_keysym_from_name(sym.encode(), 0))
    if cp < 0x20 or cp == 0x7F or cp > 0xFFFF or 0xA0 == cp:
        return ""
    return chr(cp)


def main():
    layout, name = sys.argv[1], sys.argv[2]
    variant = sys.argv[3] if len(sys.argv) > 3 else ""
    cmd = ["xkbcli", "compile-keymap", "--layout", layout] + (["--variant", variant] if variant else [])
    text = subprocess.run(cmd, check=True, capture_output=True, text=True).stdout
    keys = {}
    for m in re.finditer(r"key <(\w+)>\s*{(.*?)};", text, re.S):
        if m.group(1) not in KEYS:
            continue
        g = re.search(r"\[([^\]]*)\]", re.sub(r"symbols\[\d+\]", "", m.group(2)))
        syms = [s.strip() for s in g.group(1).split(",")][:4] if g else []
        toks = [token(s) for s in syms]
        while toks and not toks[-1]:
            toks.pop()
        if toks:
            keys[m.group(1)] = toks
    ident = f"{layout}-{variant}" if variant else layout
    xkbname = f"{layout}({variant})" if variant else layout
    rows = ",\n".join(f'      "{k}": {json.dumps(keys[k], ensure_ascii=False)}' for k in KEYS if k in keys)
    print(f'  {{\n    "id": "{ident}", "name": "{name}", "xkb": "{xkbname}",\n    "keys": {{\n{rows}\n    }}\n  }}')


main()
