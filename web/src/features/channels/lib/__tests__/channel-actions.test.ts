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

import { QueryClient } from '@tanstack/react-query'

import type { Channel, ChannelOpsResponse, GetChannelsResponse } from '../../types'
import { channelsQueryKeys, isChannelOpsQueryKey } from '../channel-actions'

describe('isChannelOpsQueryKey', () => {
  test('matches channelsQueryKeys.ops()', () => {
    expect(isChannelOpsQueryKey(channelsQueryKeys.ops())).toBe(true)
  })

  test('does not match a list() key, whose third element is a params object', () => {
    expect(isChannelOpsQueryKey(channelsQueryKeys.list({ status: 'enabled' }))).toBe(false)
  })

  test('does not match the bare lists() prefix', () => {
    expect(isChannelOpsQueryKey(channelsQueryKeys.lists())).toBe(false)
  })
})

/**
 * channelsQueryKeys.ops() deliberately nests under lists() so existing
 * invalidateQueries({queryKey: lists()}) calls also refresh it. But
 * setQueriesData/getQueriesData match by the same key prefix, and the ops
 * payload (`{success, data: {retry_times, health}}`) has no `items` — an
 * updater written for a paginated list response throws against it. These
 * tests exercise a real QueryClient (no rendering) to prove the predicate
 * `channel-test-dialog.tsx` uses keeps the ops entry out of that updater,
 * and that the hazard is real without it.
 */
describe('setQueriesData scoped to lists() and the ops entry', () => {
  const listKey = channelsQueryKeys.list({ status: 'enabled' })
  const opsKey = channelsQueryKeys.ops()

  const listPayload: GetChannelsResponse = {
    success: true,
    data: {
      items: [{ id: 1, response_time: 0, test_time: 0 } as Channel],
      total: 1,
      page: 1,
      page_size: 20,
    },
  }

  const opsPayload: ChannelOpsResponse = {
    success: true,
    data: {
      retry_times: 3,
      health: {
        active: 1,
        disabled: 0,
        slow: 0,
        untested: 0,
        slow_threshold_ms: 1000,
      },
    },
  }

  // The exact hazard reported (channel-test-dialog.tsx:505): the updater is
  // typed and written for `GetChannelsResponse` only, exactly like the real
  // `ChannelListCache` generic, so `data?.items.length` typechecks — `items`
  // isn't optional on that type. `setQueriesData`'s generic is a promise
  // about what's in the matched cache entries, not something the library
  // enforces at runtime, so when the ops entry (a different response type)
  // slips through the same key prefix, `data.items` is actually `undefined`
  // and `.length` throws.
  function listShapedUpdater(oldData: GetChannelsResponse | undefined) {
    const data = oldData?.data
    if (!oldData || !data?.items.length) return oldData
    return {
      ...oldData,
      data: { ...data, items: data.items.map((c) => ({ ...c, test_time: 999 })) },
    }
  }

  test('without the predicate, the ops entry reaches the updater and throws', () => {
    const queryClient = new QueryClient()
    queryClient.setQueryData(listKey, listPayload)
    queryClient.setQueryData(opsKey, opsPayload)

    expect(() => {
      queryClient.setQueriesData<GetChannelsResponse>(
        { queryKey: channelsQueryKeys.lists() },
        listShapedUpdater
      )
    }).toThrow()
  })

  test('with the predicate, the ops entry is excluded and the list entry still updates', () => {
    const queryClient = new QueryClient()
    queryClient.setQueryData(listKey, listPayload)
    queryClient.setQueryData(opsKey, opsPayload)

    expect(() => {
      queryClient.setQueriesData<GetChannelsResponse>(
        {
          queryKey: channelsQueryKeys.lists(),
          predicate: (query) => !isChannelOpsQueryKey(query.queryKey),
        },
        listShapedUpdater
      )
    }).not.toThrow()

    // The ops entry was never handed to the updater, so it is byte-for-byte
    // the same object we seeded.
    expect(queryClient.getQueryData(opsKey)).toBe(opsPayload)

    // The list entry, in contrast, did go through the updater.
    const updatedList = queryClient.getQueryData<GetChannelsResponse>(listKey)
    expect(updatedList?.data?.items[0]?.test_time).toBe(999)
  })
})
