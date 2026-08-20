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
import { describe, expect, test } from 'vitest'

import { parseChannelStatusInfo } from '../channel-status-info'

const empty = { statusReason: '', statusTime: null }

describe('parseChannelStatusInfo', () => {
  test('reads both fields', () => {
    expect(parseChannelStatusInfo('{"status_reason":"401 unauthorized","status_time":1700000000}')).toStrictEqual({ statusReason: '401 unauthorized', statusTime: 1700000000 })
  })

  test('returns empty for an empty string', () => {
    expect(parseChannelStatusInfo('')).toStrictEqual(empty)
  })

  test('returns empty for undefined', () => {
    expect(parseChannelStatusInfo(undefined)).toStrictEqual(empty)
  })

  test('returns empty for malformed JSON instead of throwing', () => {
    expect(parseChannelStatusInfo('{"status_reason":')).toStrictEqual(empty)
  })

  test('returns empty for JSON that is not an object', () => {
    expect(parseChannelStatusInfo('"just a string"')).toStrictEqual(empty)
  })

  test('tolerates a missing status_reason', () => {
    expect(parseChannelStatusInfo('{"status_time":1700000000}')).toStrictEqual({
      statusReason: '',
      statusTime: 1700000000,
    })
  })

  test('ignores a non-numeric status_time', () => {
    expect(parseChannelStatusInfo('{"status_time":"yesterday"}')).toStrictEqual(empty)
  })
})
