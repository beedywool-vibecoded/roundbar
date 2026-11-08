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

