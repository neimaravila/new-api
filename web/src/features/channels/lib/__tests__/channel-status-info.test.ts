// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 QuantumNous
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { parseChannelStatusInfo } from '../channel-status-info'

const empty = { statusReason: '', statusTime: null }

describe('parseChannelStatusInfo', () => {
  test('reads both fields', () => {
    assert.deepEqual(
      parseChannelStatusInfo('{"status_reason":"401 unauthorized","status_time":1700000000}'),
      { statusReason: '401 unauthorized', statusTime: 1700000000 }
    )
  })

  test('returns empty for an empty string', () => {
    assert.deepEqual(parseChannelStatusInfo(''), empty)
  })

  test('returns empty for undefined', () => {
    assert.deepEqual(parseChannelStatusInfo(undefined), empty)
  })

  test('returns empty for malformed JSON instead of throwing', () => {
    assert.deepEqual(parseChannelStatusInfo('{"status_reason":'), empty)
  })

  test('returns empty for JSON that is not an object', () => {
    assert.deepEqual(parseChannelStatusInfo('"just a string"'), empty)
  })

  test('tolerates a missing status_reason', () => {
    assert.deepEqual(parseChannelStatusInfo('{"status_time":1700000000}'), {
      statusReason: '',
      statusTime: 1700000000,
    })
  })

  test('ignores a non-numeric status_time', () => {
    assert.deepEqual(parseChannelStatusInfo('{"status_time":"yesterday"}'), empty)
  })
})
