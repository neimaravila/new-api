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
/**
 * Zod schema of the captured content of one call, as the content routes
 * return it (service/logcontent/view.go).
 */
import { z } from 'zod'

export const logContentBlockSchema = z.object({
  type: z.string(),
  text: z.string().default(''),
  id: z.string().default(''),
  name: z.string().default(''),
  arguments: z.string().default(''),
  media_type: z.string().default(''),
  size_bytes: z.number().default(0),
  is_error: z.boolean().default(false),
})

export const logContentMessageSchema = z.object({
  role: z.string(),
  source: z.string(),
  blocks: z.array(logContentBlockSchema).default([]),
})

export const logContentToolSchema = z.object({
  name: z.string(),
  description: z.string().default(''),
  schema: z.unknown().optional(),
  called: z.number().default(0),
})

export const logContentBodySchema = z.object({
  text: z.string().default(''),
  original_bytes: z.number().default(0),
  truncated: z.boolean().default(false),
  head_only: z.boolean().default(false),
  undecodable: z.boolean().default(false),
  media_omitted: z.number().default(0),
})

const emptyBody = logContentBodySchema.parse({})

export const logContentAttemptSchema = z.object({
  attempt: z.number(),
  channel_id: z.number().default(0),
  channel_name: z.string().default(''),
  multi_key_index: z.number().default(-1),
  status_code: z.number().default(0),
  error_message: z.string().default(''),
  started_at_ms: z.number().default(0),
  duration_ms: z.number().default(0),
  method: z.string().default(''),
  path: z.string().default(''),
  request_unavailable: z.boolean().default(false),
  request: logContentBodySchema.default(emptyBody),
  response: logContentBodySchema.default(emptyBody),
})

export const logContentSchema = z.object({
  request_id: z.string().default(''),
  available: z.boolean().default(false),
  reason: z.string().default(''),
  format: z.string().default(''),
  is_stream: z.boolean().default(false),
  incomplete: z.boolean().default(false),
  http_status: z.number().default(0),
  conversation: z
    .object({
      available: z.boolean().default(false),
      response_available: z.boolean().default(false),
      messages: z
        .array(logContentMessageSchema)
        .nullish()
        .transform((v) => v ?? []),
    })
    .default({ available: false, response_available: false, messages: [] }),
  tools: z
    .array(logContentToolSchema)
    .nullish()
    .transform((v) => v ?? []),
  tools_available: z.number().nullable().default(null),
  tool_calls: z.number().nullable().default(null),
  client: z
    .object({
      request: logContentBodySchema.default(emptyBody),
      response: logContentBodySchema.default(emptyBody),
    })
    .default({ request: emptyBody, response: emptyBody }),
  attempts: z
    .array(logContentAttemptSchema)
    .nullish()
    .transform((v) => v ?? []),
})

export type LogContentView = z.infer<typeof logContentSchema>
export type LogContentMessage = z.infer<typeof logContentMessageSchema>
export type LogContentBlock = z.infer<typeof logContentBlockSchema>
export type LogContentTool = z.infer<typeof logContentToolSchema>
export type LogContentBody = z.infer<typeof logContentBodySchema>
export type LogContentAttempt = z.infer<typeof logContentAttemptSchema>
