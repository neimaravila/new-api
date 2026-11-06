/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

/** A color in the OKLab family; `hue` is null when powerless (chroma ~ 0). */
export type Color = { l: number; c: number; hue: number | null; alpha: number }

const STYLES = resolve(__dirname, '..')

function stripComments(source: string): string {
  return source.replaceAll(/\/\*[\s\S]*?\*\//g, '')
}

type Rule = {
  selector: string
  declarations: [string, string][]
  specificity: number
}

function parseRules(source: string): Rule[] {
  const rules: Rule[] = []
  const re = /([^{}]+)\{([^{}]*)\}/g
  let match: RegExpExecArray | null
  while ((match = re.exec(stripComments(source)))) {
    const selector = match[1]
      .trim()
      .replaceAll(/\s+/g, ' ')
      .replaceAll(/\(\s+/g, '(')
      .replaceAll(/\s+\)/g, ')')
    const declarations: [string, string][] = []
    for (const part of match[2].split(';')) {
      const colon = part.indexOf(':')
      if (colon < 0) continue
      const name = part.slice(0, colon).trim()
      if (!name.startsWith('--')) continue
      declarations.push([
        name,
        part
          .slice(colon + 1)
          .trim()
          .replaceAll(/\s+/g, ' '),
      ])
    }
    const specificity =
      (selector.match(/\[/g)?.length ?? 0) +
      (selector.match(/\.dark/g)?.length ?? 0) +
      (selector.includes(':root') ? 1 : 0)
    rules.push({ selector, declarations, specificity })
  }
  return rules
}

const SOURCES = ['theme.css', 'theme-presets.css', 'visual-finish.css'].map(
  (name) => parseRules(readFileSync(resolve(STYLES, name), 'utf8'))
)
const ALL_RULES = SOURCES.flat()

const NOT_PRESET = /:not\(\[data-theme-preset='([a-z-]+)'\]\)/g

function applies(
  selector: string,
  preset: string | null,
  dark: boolean
): boolean {
  let rest = selector
  const darkOnly = rest.startsWith('.dark ') || rest === '.dark'
  if (darkOnly) {
    if (!dark) return false
    rest = rest === '.dark' ? '' : rest.slice('.dark '.length)
  }
  if (rest === '' || rest === ':root') return true
  if (rest === '[data-visual-finish]') return true
  const exact = /^\[data-theme-preset='([a-z-]+)'\]$/.exec(rest)
  if (exact) return preset === exact[1]
  if (rest.startsWith('[data-theme-preset]')) {
    const tail = rest.slice('[data-theme-preset]'.length)
    const nots = [...tail.matchAll(NOT_PRESET)].map((m) => m[1])
    if (tail.replaceAll(NOT_PRESET, '') !== '') return false
    return preset !== null && !nots.includes(preset)
  }
  return false
}

/** Presets declared by an exact `[data-theme-preset='x']` block in the CSS. */
export function presetIds(): string[] {
  const ids = new Set<string>()
  for (const rule of ALL_RULES) {
    const match = /^(?:\.dark )?\[data-theme-preset='([a-z-]+)'\]$/.exec(
      rule.selector
    )
    if (match) ids.add(match[1])
  }
  ids.delete('default')
  return [...ids]
}

/** Custom properties as the cascade resolves them for one preset and mode. */
export function cascade(
  preset: string | null,
  dark: boolean
): Map<string, string> {
  const vars = new Map<string, string>()
  const ordered = ALL_RULES.map((rule, order) => ({ rule, order }))
    .filter(({ rule }) => applies(rule.selector, preset, dark))
    .sort(
      (a, b) => a.rule.specificity - b.rule.specificity || a.order - b.order
    )
  for (const { rule } of ordered) {
    for (const [name, value] of rule.declarations) vars.set(name, value)
  }
  return vars
}

function substitute(
  value: string,
  vars: Map<string, string>,
  depth = 0
): string {
  if (depth > 20) throw new Error(`var() cycle in ${value}`)
  return value.replaceAll(/var\((--[a-z0-9-]+)\)/g, (_, name: string) => {
    const raw = vars.get(name)
    if (raw === undefined) throw new Error(`undefined ${name}`)
    return substitute(raw, vars, depth + 1)
  })
}

function splitTopLevel(source: string): string[] {
  const parts: string[] = []
  let depth = 0
  let start = 0
  for (let i = 0; i < source.length; i++) {
    if (source[i] === '(') depth++
    if (source[i] === ')') depth--
    if (source[i] === ',' && depth === 0) {
      parts.push(source.slice(start, i).trim())
      start = i + 1
    }
  }
  parts.push(source.slice(start).trim())
  return parts
}

const TRANSPARENT: Color = { l: 0, c: 0, hue: null, alpha: 0 }
const CHROMA_EPSILON = 1e-4

function parseNumber(token: string): number {
  return token.endsWith('%')
    ? Number.parseFloat(token) / 100
    : Number.parseFloat(token)
}

function parseColor(source: string): Color {
  const text = source.trim()
  if (text === 'transparent') return TRANSPARENT
  const oklch =
    /^oklch\(\s*([^\s/]+)\s+([^\s/]+)\s+([^\s/]+)\s*(?:\/\s*([^\s)]+)\s*)?\)$/.exec(
      text
    )
  if (oklch) {
    const chroma = parseNumber(oklch[2])
    return {
      l: parseNumber(oklch[1]),
      c: chroma,
      hue: chroma < CHROMA_EPSILON ? null : Number.parseFloat(oklch[3]),
      alpha: oklch[4] ? parseNumber(oklch[4]) : 1,
    }
  }
  if (text.startsWith('color-mix(') && text.endsWith(')')) {
    return mix(splitTopLevel(text.slice('color-mix('.length, -1)))
  }
  throw new Error(`unsupported color: ${source}`)
}

function mixArgument(argument: string): { color: Color; share: number | null } {
  const match = /^(.*?)\s+(\d+(?:\.\d+)?)%$/.exec(argument)
  if (!match) return { color: parseColor(argument), share: null }
  return {
    color: parseColor(match[1]),
    share: Number.parseFloat(match[2]) / 100,
  }
}

function mix(parts: string[]): Color {
  const space = /^in (oklch|oklab)$/.exec(parts[0])?.[1]
  if (!space || parts.length !== 3) {
    throw new Error(`unsupported mix: ${parts.join(',')}`)
  }
  const first = mixArgument(parts[1])
  const second = mixArgument(parts[2])
  const p1 = first.share ?? (second.share === null ? 0.5 : 1 - second.share)
  const p2 = second.share ?? 1 - p1
  const alpha = first.color.alpha * p1 + second.color.alpha * p2
  if (alpha === 0) return TRANSPARENT
  const w1 = (first.color.alpha * p1) / alpha
  const w2 = (second.color.alpha * p2) / alpha
  const a = first.color
  const b = second.color
  if (space === 'oklab') {
    const toLab = (x: Color) => ({
      a: x.c * Math.cos(((x.hue ?? 0) * Math.PI) / 180),
      b: x.c * Math.sin(((x.hue ?? 0) * Math.PI) / 180),
    })
    const la = toLab(a)
    const lb = toLab(b)
    const mixedA = la.a * w1 + lb.a * w2
    const mixedB = la.b * w1 + lb.b * w2
    const chroma = Math.hypot(mixedA, mixedB)
    return {
      l: a.l * w1 + b.l * w2,
      c: chroma,
      hue:
        chroma < CHROMA_EPSILON
          ? null
          : (Math.atan2(mixedB, mixedA) * 180) / Math.PI,
      alpha,
    }
  }
  // oklch: missing (powerless) hues take the other color's hue; shorter arc.
  const hueA = a.hue ?? b.hue
  const hueB = b.hue ?? a.hue
  let hue: number | null = null
  if (hueA !== null && hueB !== null) {
    let delta = hueB - hueA
    if (delta > 180) delta -= 360
    if (delta < -180) delta += 360
    hue = (hueA + delta * w2 + 360) % 360
  }
  return { l: a.l * w1 + b.l * w2, c: a.c * w1 + b.c * w2, hue, alpha }
}

/** Resolves a custom property to a color for the given cascade. */
export function resolveColor(name: string, vars: Map<string, string>): Color {
  const raw = vars.get(name)
  if (raw === undefined) throw new Error(`undefined ${name}`)
  return parseColor(substitute(raw, vars))
}

type Rgb = [number, number, number]

function toSrgb(color: Color): Rgb {
  const hueRad = ((color.hue ?? 0) * Math.PI) / 180
  const a = color.c * Math.cos(hueRad)
  const b = color.c * Math.sin(hueRad)
  const l_ = (color.l + 0.3963377774 * a + 0.2158037573 * b) ** 3
  const m_ = (color.l - 0.1055613458 * a - 0.0638541728 * b) ** 3
  const s_ = (color.l - 0.0894841775 * a - 1.291485548 * b) ** 3
  const linear = [
    4.0767416621 * l_ - 3.3077115913 * m_ + 0.2309699292 * s_,
    -1.2684380046 * l_ + 2.6097574011 * m_ - 0.3413193965 * s_,
    -0.0041960863 * l_ - 0.7034186147 * m_ + 1.707614701 * s_,
  ]
  // Out-of-gamut colors are clipped, as browsers do for sRGB displays.
  return linear.map((v) => {
    const clipped = Math.min(1, Math.max(0, v))
    return clipped <= 0.0031308
      ? 12.92 * clipped
      : 1.055 * clipped ** (1 / 2.4) - 0.055
  }) as Rgb
}

/** Paints `top` over an opaque `bottom` (sRGB compositing) and returns the opaque result. */
export function over(top: Color, bottom: Color): Rgb {
  const t = toSrgb(top)
  const b = toSrgb(bottom)
  return [0, 1, 2].map((i) => t[i] * top.alpha + b[i] * (1 - top.alpha)) as Rgb
}

export function opaque(color: Color): Rgb {
  return toSrgb(color)
}

function luminance(rgb: Rgb): number {
  const [r, g, b] = rgb.map((v) =>
    v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4
  )
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

export function contrast(foreground: Rgb, background: Rgb): number {
  const [light, dark] = [luminance(foreground), luminance(background)].sort(
    (x, y) => y - x
  )
  return (light + 0.05) / (dark + 0.05)
}
