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
import { describe, expect, it } from 'vitest'

import {
  cascade,
  colorDistance,
  contrast,
  opaque,
  over,
  presetIds,
  resolveColor,
} from './theme-cascade'

const TONES = ['success', 'danger', 'warning', 'info', 'primary', 'neutral']
const JSON_TOKENS = ['key', 'string', 'number', 'literal', 'punct']
const ROLES = ['user', 'tool', 'system', 'error']
const MINIMUM = 4.5

const variants = [null, ...presetIds()].flatMap((preset) =>
  [false, true].map((dark) => ({
    name: `${preset ?? 'default'} ${dark ? 'dark' : 'light'}`,
    preset,
    dark,
  }))
)

describe('text tokens reach WCAG AA on their surfaces', () => {
  it('covers the default theme and every preset in both modes', () => {
    expect(presetIds().length).toBeGreaterThanOrEqual(9)
  })

  it.each(variants)(
    'tone text on card and its own fill in $name',
    (variant) => {
      const vars = cascade(variant.preset, variant.dark)
      const card = resolveColor('--card', vars)
      const background = resolveColor('--background', vars)
      for (const tone of TONES) {
        const text = opaque(resolveColor(`--vf-${tone}-text`, vars))
        const fill = resolveColor(`--vf-${tone}-fill`, vars)
        const surfaces = {
          card: opaque(card),
          background: opaque(background),
          fill: over(fill, over(card, background) && card),
        }
        for (const [surface, rgb] of Object.entries(surfaces)) {
          expect(
            contrast(text, rgb),
            `${tone} text on ${surface} in ${variant.name}`
          ).toBeGreaterThanOrEqual(MINIMUM)
        }
      }
    }
  )

  it.each(variants)(
    'JSON syntax on card and role tints in $name',
    (variant) => {
      const vars = cascade(variant.preset, variant.dark)
      const card = resolveColor('--card', vars)
      const surfaces: Record<string, ReturnType<typeof opaque>> = {
        card: opaque(card),
        background: opaque(resolveColor('--background', vars)),
      }
      for (const role of ROLES) {
        surfaces[`${role} tint`] = over(
          resolveColor(`--vf-role-${role}-bg`, vars),
          card
        )
      }
      for (const token of JSON_TOKENS) {
        const text = opaque(resolveColor(`--vf-json-${token}`, vars))
        for (const [surface, rgb] of Object.entries(surfaces)) {
          expect(
            contrast(text, rgb),
            `json ${token} on ${surface} in ${variant.name}`
          ).toBeGreaterThanOrEqual(MINIMUM)
        }
      }
    }
  )
})

describe('role tokens stay distinguishable', () => {
  // Largest distance in OKLab at which two dots still read as the same color.
  const MIN_DISTANCE = 0.06

  it.each(variants)('tool and user dots differ in $name', (variant) => {
    const vars = cascade(variant.preset, variant.dark)
    expect(vars.get('--vf-role-tool-dot')).not.toBe(
      vars.get('--vf-role-user-dot')
    )
    const tool = resolveColor('--vf-role-tool-dot', vars)
    const user = resolveColor('--vf-role-user-dot', vars)
    expect(
      colorDistance(tool, user),
      `tool vs user in ${variant.name}`
    ).toBeGreaterThanOrEqual(MIN_DISTANCE)
  })

  it('separates tool from user in the lake-view preset by more than the warning color did', () => {
    const vars = cascade('lake-view', false)
    const user = resolveColor('--vf-role-user-dot', vars)
    expect(
      colorDistance(resolveColor('--vf-role-tool-dot', vars), user)
    ).toBeGreaterThan(colorDistance(resolveColor('--chart-4', vars), user))
  })
})
