#!/usr/bin/env python3
"""Builds the README visuals in .github/assets: banner, section titles and footer in
light and dark versions (Material 3 Expressive, same look as the portfolio).

    python3 tools/readme-assets.py

The banner embeds .github/assets/screens/device-media.webp, a real device screenshot
taken with `roundbar screenshot`.
"""
import base64
import io
import math
import os

from PIL import Image

ROOT = os.path.join(os.path.dirname(__file__), "..", ".github", "assets")

THEMES = {
    "dark": dict(card="#211F26", page="#141218", bar="#2B2930", text="#E6E0E9", muted="#CAC4D0", outline="#938F99",
                 line="#49454F", pc="#4F378B", on_pc="#EADDFF", secc="#4A4458", on_secc="#E8DEF8", tc="#633B48",
                 on_tc="#FFD8E4", dot="#D0BCFF", bezel="#0F0E12", bezel_hi="#3A3740"),
    "light": dict(card="#F3EDF7", page="#FEF7FF", bar="#E6E0E9", text="#1D1B20", muted="#49454F", outline="#79747E",
                  line="#CAC4D0", pc="#EADDFF", on_pc="#21005D", secc="#E8DEF8", on_secc="#1D192B", tc="#FFD8E4",
                  on_tc="#31111D", dot="#6750A4", bezel="#1D1B20", bezel_hi="#49454F"),
}

# Same (lobes, depth) as the device buttons (docs/spec.md, Button.shape).
SHAPES = {"cookie": (9, 0.07), "clover": (4, 0.17), "sunny": (8, 0.10), "flower": (6, 0.13), "burst": (12, 0.05)}

STYLE = """<style>
  text { font-family: 'Google Sans Flex','Google Sans','Roboto Flex',Roboto,'Segoe UI',system-ui,sans-serif; }
  .spin { transform-box: fill-box; transform-origin: center; animation: spin 48s linear infinite; }
  .float { animation: float 6s ease-in-out infinite; }
  .fade { animation: fade .8s cubic-bezier(.2,0,0,1) backwards; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @keyframes float { 50% { transform: translateY(-8px); } }
  @keyframes fade { from { opacity: 0; transform: translateY(12px); } }
  @media (prefers-reduced-motion: reduce) { * { animation: none !important; } }
</style>"""


def shape(kind, cx, cy, r, steps=96):
    lobes, depth = SHAPES[kind]
    pts = []
    for i in range(steps):
        t = 2 * math.pi * i / steps
        rr = r * (1 + depth * math.cos(lobes * t))
        pts.append(f"{cx + rr * math.cos(t):.1f},{cy + rr * math.sin(t):.1f}")
    return "M" + " L".join(pts) + " Z"


def webp_b64(path, size):
    img = Image.open(path).convert("RGBA").resize((size, size), Image.LANCZOS)
    buf = io.BytesIO()
    img.save(buf, "WEBP", quality=84, method=6)
    return base64.b64encode(buf.getvalue()).decode()


def write(name, svg):
    with open(os.path.join(ROOT, name), "w") as f:
        f.write(svg)


def chips(c, x, y, items):
    out = []
    for label, bg, fg in items:
        w = 26 + sum(11.5 if ch.isupper() else 8.6 for ch in label)
        out.append(f'<rect x="{x:.1f}" y="{y}" width="{w:.0f}" height="40" rx="20" fill="{bg}"/>'
                   f'<text x="{x + w / 2:.0f}" y="{y + 25}" font-size="15" font-weight="500" text-anchor="middle" fill="{fg}">{label}</text>')
        x += w + 8
    return "".join(out)


def ticks(c, cx, cy, r_in, r_out, count=48, skip=()):
    out = []
    for i in range(count):
        if i in skip:
            continue
        t = 2 * math.pi * i / count - math.pi / 2
        long_tick = i % 4 == 0
        r0 = r_in - (4 if long_tick else 0)
        out.append(f'<line x1="{cx + r0 * math.cos(t):.1f}" y1="{cy + r0 * math.sin(t):.1f}" '
                   f'x2="{cx + r_out * math.cos(t):.1f}" y2="{cy + r_out * math.sin(t):.1f}" '
                   f'stroke="{c["outline"]}" stroke-width="{2.4 if long_tick else 1.4}" stroke-linecap="round" '
                   f'opacity="{.9 if long_tick else .55}"/>')
    return "".join(out)


def arc(cx, cy, r, a0, a1):
    """SVG arc path from angle a0 to a1 (degrees, 0 = top, clockwise)."""
    def pt(a):
        t = math.radians(a - 90)
        return cx + r * math.cos(t), cy + r * math.sin(t)
    (x0, y0), (x1, y1) = pt(a0), pt(a1)
    large = 1 if (a1 - a0) % 360 > 180 else 0
    return f"M{x0:.1f},{y0:.1f} A{r},{r} 0 {large} 1 {x1:.1f},{y1:.1f}"


def banner(theme, shot):
    c = THEMES[theme]
    img = webp_b64(shot, 440)
    cx, cy = 770, 172
    return f"""<svg xmlns="http://www.w3.org/2000/svg" width="1000" height="340" viewBox="0 0 1000 340" role="img">
<title>Roundbar | Un écran rond qui suit l'app au premier plan</title>
{STYLE}
<defs>
  <clipPath id="card"><rect width="1000" height="340" rx="36"/></clipPath>
  <clipPath id="screen"><circle cx="{cx}" cy="{cy}" r="110"/></clipPath>
</defs>
<g clip-path="url(#card)">
  <rect width="1000" height="340" fill="{c['card']}"/>
  <path class="spin" d="{shape('cookie', cx, cy, 165)}" fill="{c['pc']}"/>
  <path class="spin" d="{shape('flower', 955, 330, 90)}" fill="{c['tc']}"/>
  <path d="{shape('clover', 530, 335, 60)}" fill="{c['secc']}"/>
  <g class="float"><circle cx="958" cy="44" r="13" fill="{c['dot']}"/></g>
</g>
<g class="fade">
  {ticks(c, cx, cy, 146, 154)}
  <path d="{arc(cx, cy, 150, 214, 326)}" fill="none" stroke="{c['dot']}" stroke-width="6" stroke-linecap="round"/>
  <circle cx="{cx}" cy="{cy}" r="132" fill="{c['bezel']}"/>
  <circle cx="{cx}" cy="{cy}" r="131" fill="none" stroke="{c['bezel_hi']}" stroke-width="2"/>
  <circle cx="{cx}" cy="{cy}" r="110" fill="#000"/>
  <image href="data:image/webp;base64,{img}" x="{cx - 110}" y="{cy - 110}" width="220" height="220" clip-path="url(#screen)"/>
</g>
<g class="fade">
  <rect x="48" y="52" width="212" height="32" rx="8" fill="none" stroke="{c['outline']}"/>
  <text x="62" y="73" font-size="14" font-weight="500" fill="{c['muted']}">ESP32-S3 · écran rond 1,8"</text>
  <text x="44" y="166" font-size="72" font-weight="500" letter-spacing="-1.5" fill="{c['text']}">Roundbar</text>
  <text x="48" y="212" font-size="21" fill="{c['muted']}">Un écran rond qui suit l'app au premier plan</text>
  {chips(c, 48, 250, [('Material You', c['pc'], c['on_pc']), ('ESP32-S3', c['secc'], c['on_secc']), ('GNOME', c['tc'], c['on_tc'])])}
</g>
</svg>
"""


def section(theme, num, title, kind):
    c = THEMES[theme]
    width = 84 + len(title) * 17.5 + 24
    return f"""<svg xmlns="http://www.w3.org/2000/svg" width="1000" height="100" viewBox="0 0 1000 100" role="img">
<title>{title}</title>
{STYLE}
<g transform="translate(0 28)">
<path class="spin" d="{shape(kind, 36, 36, 30)}" fill="{c['pc']}"/>
<text x="36" y="42" font-size="16" font-weight="600" text-anchor="middle" fill="{c['on_pc']}">{num}</text>
<text x="84" y="46" font-size="30" font-weight="500" fill="{c['text']}">{title}</text>
<rect x="{width:.0f}" y="35" width="{1000 - width:.0f}" height="2" rx="1" fill="{c['line']}"/>
</g>
</svg>
"""


def footer(theme):
    c = THEMES[theme]
    blobs = [("cookie", 60, 30, 70, c["pc"]), ("clover", 230, 110, 50, c["secc"]), ("sunny", 770, 10, 60, c["tc"]),
             ("flower", 940, 100, 70, c["pc"]), ("cookie", 600, 125, 40, c["secc"])]
    paths = "".join(f'<path class="spin" d="{shape(k, x, y, r)}" fill="{f}" opacity=".9"/>' for k, x, y, r, f in blobs)
    return f"""<svg xmlns="http://www.w3.org/2000/svg" width="1000" height="120" viewBox="0 0 1000 120" role="img">
<title>Roundbar</title>
{STYLE}
<defs><clipPath id="card"><rect width="1000" height="120" rx="28"/></clipPath></defs>
<g clip-path="url(#card)">
  <rect width="1000" height="120" fill="{c['card']}"/>
  {paths}
</g>
<text x="500" y="68" font-size="18" font-weight="500" text-anchor="middle" fill="{c['text']}">beedywool.com</text>
</svg>
"""


SECTIONS = [
    ("preview", "01", "Aperçu", "cookie"),
    ("features", "02", "Fonctionnalités", "clover"),
    ("install", "03", "Installation", "sunny"),
    ("firmware", "04", "Firmware et carte", "flower"),
    ("stack", "05", "Stack", "cookie"),
    ("dev", "06", "Développement", "clover"),
]


def main():
    shot = os.path.join(ROOT, "screens", "device-media.webp")
    for theme in THEMES:
        write(f"banner-{theme}.svg", banner(theme, shot))
        write(f"footer-{theme}.svg", footer(theme))
        for key, num, title, kind in SECTIONS:
            write(f"section-{key}-{theme}.svg", section(theme, num, title, kind))
    print("assets written to", os.path.normpath(ROOT))


if __name__ == "__main__":
    main()
