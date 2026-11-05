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
import { useQuery, useQueryClient } from '@tanstack/react-query'

import { useDebounce } from '@/hooks/use-debounce'

import { getLogContent } from '../../api'
import type { UsageLog } from '../../data/schema'

/** Wait between lookups while a call's content is still being saved. */
export const LOG_CONTENT_PENDING_REFETCH_MS = 2000
/** Lookups made while pending before the panel offers a manual Retry. */
export const LOG_CONTENT_PENDING_REFETCH_LIMIT = 6
/** Pause on a log before fetching, so holding an arrow key sends few lookups. */
const SWITCH_DEBOUNCE_MS = 150

/** Only consume and error logs of a call carry captured content. */
export function logHasContent(log: UsageLog): boolean {
  return (log.type === 2 || log.type === 5) && !!log.request_id
}

/**
 * The captured content of the call behind `log`. Nothing is fetched for logs
 * that cannot have content, and a pending result is looked up again a few
 * times.
 */
export function useLogContent(log: UsageLog, isAdmin: boolean) {
  const requestId = log.request_id
  const settled = useDebounce(requestId, SWITCH_DEBOUNCE_MS) === requestId
  const queryKey = ['log-content', isAdmin, requestId]
  const queryClient = useQueryClient()
  const result = useQuery({
    queryKey,
    queryFn: () => getLogContent(requestId, isAdmin),
    enabled: logHasContent(log) && settled,
    retry: false,
    // The panel's error state is the only message for a failed lookup.
    meta: { errorToast: false },
    staleTime: 30_000,
    refetchInterval: (query) =>
      query.state.data?.reason === 'pending' &&
      query.state.dataUpdateCount < LOG_CONTENT_PENDING_REFETCH_LIMIT
        ? LOG_CONTENT_PENDING_REFETCH_MS
        : false,
  })
  // Pending lookups are used up: the panel then offers a manual Retry.
  const updates = queryClient.getQueryState(queryKey)?.dataUpdateCount ?? 0
  return {
    ...result,
    pendingExhausted:
      result.data?.reason === 'pending' &&
      updates >= LOG_CONTENT_PENDING_REFETCH_LIMIT,
  }
}
