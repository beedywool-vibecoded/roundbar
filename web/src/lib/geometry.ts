// SVG helpers for the round preview. Angles in degrees, 0 at the top, clockwise.

export function polar(cx: number, cy: number, r: number, deg: number): [number, number] {
  const t = ((deg - 90) * Math.PI) / 180
  return [cx + r * Math.cos(t), cy + r * Math.sin(t)]
}

export function arc(cx: number, cy: number, r: number, a0: number, a1: number): string {
  if (a1 - a0 >= 359.99) a1 = a0 + 359.99
  const [x0, y0] = polar(cx, cy, r, a0)
  const [x1, y1] = polar(cx, cy, r, a1)
  const large = a1 - a0 > 180 ? 1 : 0
  return `M${x0.toFixed(2)} ${y0.toFixed(2)} A${r} ${r} 0 ${large} 1 ${x1.toFixed(2)} ${y1.toFixed(2)}`
}

// M3 Expressive wavy progress: the active part of an arc, as a sine wave around the radius.
export function wavyArc(cx: number, cy: number, r: number, a0: number, a1: number, amp = 3, wavelength = 26): string {
  const len = ((a1 - a0) * Math.PI * r) / 180
  if (len <= 0) return ''
  const steps = Math.max(8, Math.ceil(len / 2))
  const pts: string[] = []
  for (let i = 0; i <= steps; i++) {
    const a = a0 + ((a1 - a0) * i) / steps
    const d = (len * i) / steps
    // Ease the wave in and out so the ends meet the track cleanly
    const fade = Math.min(1, d / 8, (len - d) / 8)
    const rr = r + amp * fade * Math.sin((2 * Math.PI * d) / wavelength)
    const [x, y] = polar(cx, cy, rr, a)
    pts.push(`${x.toFixed(2)},${y.toFixed(2)}`)
  }
  return `M${pts.join(' L')}`
}
