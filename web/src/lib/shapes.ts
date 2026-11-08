// Material 3 Expressive shapes (polar formula shared with the portfolio and the app)
export const SHAPES = {
  cookie: [9, 0.07],
  clover: [4, 0.17],
  sunny: [8, 0.1],
  flower: [6, 0.13],
  burst: [12, 0.05],
} as const

export type ShapeKind = keyof typeof SHAPES

export function shapePath(kind: ShapeKind, cx = 50, cy = 50, r = 46, steps = 96): string {
  const [lobes, depth] = SHAPES[kind]
  // One decimal is plenty in a 100-unit viewBox; 0..1 clip paths need more
  const digits = r < 1 ? 4 : 1
  const pts: string[] = []
  for (let i = 0; i < steps; i++) {
    const t = (2 * Math.PI * i) / steps
    const rr = r * (1 + depth * Math.cos(lobes * t))
    pts.push(
      `${(cx + rr * Math.cos(t)).toFixed(digits)},${(cy + rr * Math.sin(t)).toFixed(digits)}`,
    )
  }
  return `M${pts.join(' L')} Z`
}


// ---- button shapes (Button.shape, docs/spec.md section 1) ----

export const BUTTON_SHAPES = ['circle', 'square', 'cookie', 'clover', 'sunny', 'flower', 'burst'] as const
export type ButtonShape = (typeof BUTTON_SHAPES)[number]

// Squircle |x|^4 + |y|^4 = r^4
export function squirclePath(cx: number, cy: number, r: number, steps = 96): string {
  const pts: string[] = []
  for (let i = 0; i < steps; i++) {
    const t = (2 * Math.PI * i) / steps
    const c = Math.cos(t)
    const s = Math.sin(t)
    const x = cx + r * Math.sign(c) * Math.sqrt(Math.abs(c))
    const y = cy + r * Math.sign(s) * Math.sqrt(Math.abs(s))
    pts.push(`${x.toFixed(2)},${y.toFixed(2)}`)
  }
  return `M${pts.join(' L')} Z`
}

// Outline of a button of `size` px centered on (cx, cy). Lobed shapes are scaled so their
// outermost points stay inside the size, like a circle of the same size.
export function buttonPath(shape: ButtonShape, cx: number, cy: number, size: number): string {
  const r = size / 2
  if (shape === 'circle') return `M${cx - r},${cy} a${r},${r} 0 1,0 ${2 * r},0 a${r},${r} 0 1,0 ${-2 * r},0 Z`
  if (shape === 'square') return squirclePath(cx, cy, r)
  const [, depth] = SHAPES[shape]
  return shapePath(shape, cx, cy, r / (1 + depth), 144)
}
