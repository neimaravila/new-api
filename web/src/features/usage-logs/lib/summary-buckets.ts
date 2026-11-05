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
import type { LogSummaryBucket } from '../types'

// The backend keeps a range within 60 buckets; this only guards a bad response.
const MAX_FILLED_BUCKETS = 120

/**
 * The backend leaves out buckets without rows. Walk the whole period so an idle
 * stretch shows as a gap of zero bars and the time axis stays even.
 */
export function fillSummaryBuckets(
  buckets: LogSummaryBucket[],
  bucketSeconds: number,
  startSeconds: number | undefined,
  endSeconds: number | undefined
): LogSummaryBucket[] {
  if (bucketSeconds <= 0 || buckets.length === 0) return buckets
  const byTime = new Map(buckets.map((bucket) => [bucket.timestamp, bucket]))
  const first = Math.min(...buckets.map((bucket) => bucket.timestamp))
  const last = Math.max(...buckets.map((bucket) => bucket.timestamp))
  const now = Math.floor(Date.now() / 1000)
  const from = Math.min(
    first,
    startSeconds != null
      ? Math.floor(startSeconds / bucketSeconds) * bucketSeconds
      : first
  )
  const to = Math.max(
    last,
    Math.floor(Math.min(endSeconds ?? now, now) / bucketSeconds) * bucketSeconds
  )
  if ((to - from) / bucketSeconds >= MAX_FILLED_BUCKETS) return buckets

  const filled: LogSummaryBucket[] = []
  for (let ts = from; ts <= to; ts += bucketSeconds) {
    filled.push(byTime.get(ts) ?? { timestamp: ts, requests: 0, failures: 0 })
  }
  return filled
}
