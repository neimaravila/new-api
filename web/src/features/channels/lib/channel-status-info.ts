// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2024 QuantumNous
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

export type ChannelStatusInfo = {
  statusReason: string
  statusTime: number | null
}

const EMPTY: ChannelStatusInfo = { statusReason: '', statusTime: null }

/**
 * `other_info` is a free-form string column written by several backend paths.
 * It may be empty, malformed, or missing either key, so this never throws and
 * degrades to showing the status alone.
 */
export function parseChannelStatusInfo(
  otherInfo: string | undefined
): ChannelStatusInfo {
  if (!otherInfo) {
    return EMPTY
  }
  let parsed: unknown
  try {
    parsed = JSON.parse(otherInfo)
  } catch {
    return EMPTY
  }
  if (typeof parsed !== 'object' || parsed === null) {
    return EMPTY
  }
  const record = parsed as Record<string, unknown>
  return {
    statusReason:
      typeof record.status_reason === 'string' ? record.status_reason : '',
    statusTime:
      typeof record.status_time === 'number' ? record.status_time : null,
  }
}
