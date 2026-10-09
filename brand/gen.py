#!/usr/bin/env python3
"""Генератор логотипа kusec: знак (семиугольник-замок) + монолинейная надпись."""
import math
import os
import sys

OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.dirname(os.path.abspath(__file__))

GREEN = "#18a058"
GREEN_LIGHT = "#3fbf7f"
GREEN_DARK = "#0b6b3a"
INK = "#1c2b3a"
INK_DARK = "#eef3f0"


def f(v: float) -> str:
    s = f"{v:.2f}".rstrip("0").rstrip(".")
    return "0" if s == "-0" else s


def heptagon(cx: float, cy: float, r: float, round_d: float) -> str:
    """Скруглённый семиугольник: вершина вниз, плоская грань сверху."""
    pts = []
    for k in range(7):
        a = math.radians(90 + k * 360 / 7)
        pts.append((cx + r * math.cos(a), cy + r * math.sin(a)))
    n = len(pts)
    segs = []
    for i, v in enumerate(pts):
        p = pts[i - 1]
        q = pts[(i + 1) % n]
        dp = (p[0] - v[0], p[1] - v[1])
        dq = (q[0] - v[0], q[1] - v[1])
        lp = math.hypot(*dp)
        lq = math.hypot(*dq)
        a = (v[0] + dp[0] / lp * round_d, v[1] + dp[1] / lp * round_d)
        b = (v[0] + dq[0] / lq * round_d, v[1] + dq[1] / lq * round_d)
        segs.append((a, v, b))
    d = f"M {f(segs[0][0][0])},{f(segs[0][0][1])} "
    for i, (a, v, b) in enumerate(segs):
        if i:
            d += f"L {f(a[0])},{f(a[1])} "
        d += f"Q {f(v[0])},{f(v[1])} {f(b[0])},{f(b[1])} "
    return d + "Z"


def heptagon_top_y(cy: float, r: float) -> float:
    return cy + r * math.sin(math.radians(90 + 3 * 360 / 7))


def keyhole(cx: float, cy: float, r: float, slot_h: float) -> str:
    """Скважина одним контуром: круг + паз со скруглённым низом."""
    hw = r / 2
    y0 = cy + math.sqrt(r * r - hw * hw)
    yb = cy + slot_h
    return (
        f"M {f(cx - hw)},{f(y0)} A {f(r)},{f(r)} 0 1 1 {f(cx + hw)},{f(y0)} "
        f"L {f(cx + hw)},{f(yb)} A {f(hw)},{f(hw)} 0 0 1 {f(cx - hw)},{f(yb)} Z"
    )


# ---------- знак ----------
# viewBox 0 0 64 64; корпус — семиугольник, сверху дужка.
CX, CY, R = 32.0, 36.5, 22.5
BODY = heptagon(CX, CY, R, 3.2)
TOP_Y = heptagon_top_y(CY, R)  # плоская грань сверху


def shackle(r: float) -> str:
    return (
        f"M {f(CX - r)},{f(TOP_Y + 2)} V {f(TOP_Y - 3.5)} "
        f"A {f(r)},{f(r)} 0 0 1 {f(CX + r)},{f(TOP_Y - 3.5)} "
        f"V {f(TOP_Y + 2)}"
    )


def mark_a(fill: str, small: bool = False) -> str:
    """Знак: замок-семиугольник со скважиной.

    small=True — пропорции для favicon: толще дужка, крупнее скважина,
    чтобы знак читался в 16 px.
    """
    if small:
        sh, sw, hole = shackle(9.4), 5.4, keyhole(CX, CY - 2.5, 5.4, 13.5)
    else:
        sh, sw, hole = shackle(9.2), 4.8, keyhole(CX, CY - 2.5, 4.8, 13)
    body = BODY + " " + hole
    return (
        f'<path d="{sh}" fill="none" stroke="{fill}" stroke-width="{sw}" '
        f'stroke-linecap="round"/>'
        f'<path d="{body}" fill="{fill}" fill-rule="evenodd"/>'
    )


# ---------- надпись «kusec», монолиния ----------
# x-height 100, базовая линия y=100, верх строчных y=0, выносной элемент k до -40.
# Координаты — осевые линии штриха толщиной 22.
SW = 22
LETTERS = [
    # k
    ("M 11,-40 V 89 M 11,60 L 60,11 M 32,39 L 64,89", 75),
    # u
    ("M 11,11 V 59 A 30,30 0 0 0 71,59 M 71,11 V 89", 82),
    # s: два полукруга r=20, центры (35,30) и (35,70)
    ("M 55,30 A 20,20 0 1 0 35,50 A 20,20 0 1 1 15,70", 66),
    # e
    ("M 1,50 H 79 A 39,39 0 1 0 67.6,77.6", 80),
    # c
    ("M 67.6,22.4 A 39,39 0 1 0 67.6,77.6", 80),
]
GAP = 24


def wordmark_paths() -> tuple[str, float]:
    x = 0.0
    parts = []
    for d, w in LETTERS:
        parts.append(f'<path d="{d}" transform="translate({f(x)} 0)"/>')
        x += w + GAP
    return "".join(parts), x - GAP


WM, WM_W = wordmark_paths()

GRAD = (
    '<linearGradient id="kg" gradientUnits="userSpaceOnUse" x1="8" y1="4" x2="56" y2="60">'
    f'<stop offset="0" stop-color="{GREEN_LIGHT}"/>'
    f'<stop offset="1" stop-color="{GREEN_DARK}"/>'
    "</linearGradient>"
)


def write(name: str, content: str) -> None:
    with open(os.path.join(OUT, name), "w") as fh:
        fh.write(content)


# favicon: плоский цвет, ничего лишнего
write(
    "favicon.svg",
    f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">{mark_a(GREEN, small=True)}</svg>\n',
)

# иконка для «добавить на экран» (iOS/Android): фон во весь квадрат, знак белый
write(
    "apple-touch-icon.svg",
    f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><defs>{GRAD}</defs>'
    f'<rect width="64" height="64" fill="url(#kg)"/>'
    f'<g transform="translate(32 32) scale(0.78) translate(-32 -32)">{mark_a("#fff", small=True)}</g></svg>\n',
)

# знак с градиентом (для крупных размеров)
write(
    "mark.svg",
    f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><defs>{GRAD}</defs>'
    f'{mark_a("url(#kg)")}</svg>\n',
)

# полный логотип: знак + надпись, цвет надписи следует за темой ОС
MARK_H = 64
SCALE = 0.29  # 140 единиц надписи -> ~40.6
WM_X = MARK_H + 14
WM_DY = (MARK_H - 140 * SCALE) / 2  # центрируем надпись по высоте знака
total_w = WM_X + WM_W * SCALE + 2
logo_style = (
    f".wm{{fill:none;stroke:{INK};stroke-width:{SW};stroke-linecap:round;stroke-linejoin:round}}"
    f"@media (prefers-color-scheme:dark){{.wm{{stroke:{INK_DARK}}}}}"
)
write(
    "logo.svg",
    f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {f(total_w)} {MARK_H}" '
    f'width="{f(total_w)}" height="{MARK_H}" role="img" aria-label="kusec">'
    f"<style>{logo_style}</style><defs>{GRAD}</defs>"
    f'{mark_a("url(#kg)")}'
    f'<g class="wm" transform="translate({WM_X} {f(WM_DY)}) scale({SCALE})">'
    f'<g transform="translate(0 40)">{WM}</g></g>'
    "</svg>\n",
)

# версии на тёмном/светлом фоне с фиксированным цветом (для README и т.п.)
for suffix, ink in (("light", INK), ("dark", INK_DARK)):
    style = f".wm{{fill:none;stroke:{ink};stroke-width:{SW};stroke-linecap:round;stroke-linejoin:round}}"
    write(
        f"logo-{suffix}.svg",
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {f(total_w)} {MARK_H}" '
        f'width="{f(total_w)}" height="{MARK_H}" role="img" aria-label="kusec">'
        f"<style>{style}</style><defs>{GRAD}</defs>"
        f'{mark_a("url(#kg)")}'
        f'<g class="wm" transform="translate({WM_X} {f(WM_DY)}) scale({SCALE})">'
        f"<g transform=\"translate(0 40)\">{WM}</g></g>"
        "</svg>\n",
    )

