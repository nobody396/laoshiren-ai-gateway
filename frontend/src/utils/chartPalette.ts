/**
 * The single source of chart color, resolved from CSS custom properties.
 *
 * Chart.js draws to a canvas, and canvas cannot resolve `var(--x)` — it needs
 * a concrete color string. So unlike the rest of the app, charts can't just
 * reference a token; they have to read it at runtime. This module is that
 * bridge, and it is the ONLY place allowed to do it.
 *
 * Before this existed, each chart carried its own pasted copy of the palette
 * behind a `palette="greco"` opt-in prop that defaulted to *off* — so most
 * charts silently rendered in Tailwind's stock blue/emerald/violet while the
 * page around them was warm paper. The copies had also already drifted
 * (ten entries in one file, twelve in two others).
 *
 * To re-theme every chart in the app, edit --chart-1…12 in styles/theme.css.
 */

import { computed, ref } from 'vue'

const SERIES_COUNT = 12

/**
 * Fallback used when no document is available (unit tests, SSR) or when a
 * variable is missing. Mirrors --chart-1…12; kept in sync by
 * utils/__tests__/chartPalette.spec.ts, which parses theme.css.
 */
const FALLBACK = [
  '#9a3b1f', '#3f5a3a', '#9a6a1f', '#315f71', '#7a4f2b', '#6f4f87',
  '#8a7d63', '#5e2210', '#b77a28', '#26361f', '#6f8b80', '#a85f42',
] as const

/** "154 59 31" → "rgb(154 59 31)"; alpha 0.2 → "rgb(154 59 31 / 0.2)" */
function toColor(triple: string, alpha?: number): string {
  const t = triple.trim()
  return alpha === undefined ? `rgb(${t})` : `rgb(${t} / ${alpha})`
}

function readTriple(index: number): string | null {
  if (typeof document === 'undefined') return null
  const raw = getComputedStyle(document.documentElement)
    .getPropertyValue(`--chart-${index + 1}`)
  return raw.trim() || null
}

/**
 * The chart series palette, in order. Read fresh on every call: the values
 * live on :root, so a theme switch (or a `.dark` toggle that redefines them)
 * takes effect the next time a chart rebuilds, with no cache to invalidate.
 *
 * @param alpha optional opacity, for fills under a line series
 */
export function chartPalette(alpha?: number): string[] {
  return Array.from({ length: SERIES_COUNT }, (_, i) => {
    const triple = readTriple(i)
    if (triple) return toColor(triple, alpha)
    const hex = FALLBACK[i]
    // Fallback path can't express alpha as a hex without 8-digit notation,
    // which is fine everywhere we target.
    return alpha === undefined
      ? hex
      : hex + Math.round(alpha * 255).toString(16).padStart(2, '0')
  })
}

/**
 * One series color by zero-based index, wrapping past the end of the palette.
 * Prefer chartPalette() when you need the whole list.
 */
export function chartColor(index: number, alpha?: number): string {
  return chartPalette(alpha)[index % SERIES_COUNT]
}

/* ───────────────────────────────────────────────────────────────────────────
 * Reactive access
 *
 * chartPalette() reads the DOM, so a plain `computed(() => chartPalette())`
 * has no reactive dependency — it would never recompute when the user toggles
 * dark mode, and every chart would keep its light-mode colors on a dark
 * surface. This shared "theme epoch" counter is that missing dependency.
 *
 * One MutationObserver for the whole app, created lazily on first use and
 * never torn down: it's a single observer on <html class>, and charts mount
 * and unmount constantly.
 * ─────────────────────────────────────────────────────────────────────────── */

const themeEpoch = ref(0)
let observing = false

function ensureThemeObserver(): void {
  if (observing || typeof document === 'undefined') return
  observing = true
  new MutationObserver(() => { themeEpoch.value++ })
    .observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
}

/** Reactive series palette. Recomputes when the dark-mode class flips. */
export function useChartPalette(alpha?: number) {
  ensureThemeObserver()
  return computed(() => {
    themeEpoch.value // dependency — do not remove
    return chartPalette(alpha)
  })
}

/** Reactive axis/grid/tooltip colors. Same contract as useChartPalette. */
export function useChartInk() {
  ensureThemeObserver()
  return computed(() => {
    themeEpoch.value // dependency — do not remove
    return chartInk()
  })
}

/** Reactive semantic colors. Same contract as useChartPalette. */
export function useChartSemantic(alpha?: number) {
  ensureThemeObserver()
  return computed(() => {
    themeEpoch.value // dependency — do not remove
    return chartSemantic(alpha)
  })
}

/**
 * Resolve any theme token to a concrete color string.
 *
 * For the handful of places that need a real color in JavaScript rather than
 * in CSS: canvas, and SVG `:stop-color` / `:stroke` bindings. Everywhere else
 * — every stylesheet, every class — must use `rgb(var(--token))` directly and
 * never call this.
 *
 * @param name  full custom-property name, e.g. '--color-terracotta'
 * @param fallback used when there's no document (tests, SSR) or no such token
 */
export function themeColor(name: string, fallback: string, alpha?: number): string {
  if (typeof document === 'undefined') return fallback
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  if (!raw) return fallback
  return alpha === undefined ? `rgb(${raw})` : `rgb(${raw} / ${alpha})`
}

const readToken = themeColor

/**
 * Chrome colors — axis text, grid lines, tooltip surface. Not series colors.
 * Same rule as everything else here: read the token, never paste the hex.
 */
export function chartInk(): {
  text: string
  muted: string
  grid: string
  surface: string
  border: string
} {
  return {
    text: readToken('--color-ink', '#1f1a12'),
    muted: readToken('--color-muted', '#8a7d63'),
    // Grid lines must recede: the stone token at partial alpha, never a
    // separate grey, or the chart reintroduces a cool neutral.
    grid: readToken('--color-stone', 'rgba(239, 230, 207, 0.9)', 0.9),
    surface: readToken('--color-marble', '#faf6ec'),
    border: readToken('--color-stone', '#efe6cf'),
  }
}

/**
 * Meaning-carrying colors, for charts whose series ARE semantic — an error
 * trend, a success/failure split, a latency threshold. Use these instead of
 * chartPalette() there: on those charts "red" has to mean bad, and taking
 * series 1 by position would make it terracotta.
 *
 * `alpha` produces the matching fill for a line's area.
 */
export function chartSemantic(alpha?: number): {
  danger: string
  success: string
  warning: string
  info: string
  neutral: string
  accent: string
} {
  return {
    danger: readToken('--color-danger', '#9f2f23', alpha),
    success: readToken('--color-success', '#3f6f43', alpha),
    warning: readToken('--color-warning', '#9a6a1f', alpha),
    info: readToken('--color-info', '#315f71', alpha),
    neutral: readToken('--color-muted', '#8a7d63', alpha),
    // The plum series color — the only non-semantic entry, for charts that
    // need a second "other" line that must not read as a status.
    accent: readToken('--chart-6', '#6f4f87', alpha),
  }
}
