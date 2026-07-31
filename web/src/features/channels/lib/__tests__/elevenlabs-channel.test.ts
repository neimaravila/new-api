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
  CHANNEL_TYPE_ELEVENLABS,
  CHANNEL_TYPE_OPTIONS,
  MODEL_FETCHABLE_TYPES,
} from '../../constants'
import { CHANNEL_FORM_DEFAULT_VALUES, channelFormSchema } from '../channel-form'
import { getChannelTypeConfig } from '../channel-type-config'
import { getChannelTypeIcon, getKeyPromptForType } from '../channel-utils'

describe('ElevenLabs channel', () => {
  it('registers selection, model discovery, and icon metadata', () => {
    expect(
      CHANNEL_TYPE_OPTIONS.find(
        (item) => item.value === CHANNEL_TYPE_ELEVENLABS
      )
    ).toEqual({ value: CHANNEL_TYPE_ELEVENLABS, label: 'ElevenLabs' })
    expect(MODEL_FETCHABLE_TYPES.has(CHANNEL_TYPE_ELEVENLABS)).toBe(true)
    expect(getChannelTypeIcon(CHANNEL_TYPE_ELEVENLABS)).toBe('ElevenLabs')
    expect(getKeyPromptForType(CHANNEL_TYPE_ELEVENLABS)).toBe(
      'Format: ElevenLabs API Key (sent as xi-api-key header)'
    )
  })

  it('resolves the same icon name from the type map and the type config', () => {
    // Both call sites render `${icon}.Color`, so the two must not drift apart.
    // Whether @lobehub/icons exports that name is not asserted here: importing an
    // icon pulls in antd-style, which needs browser globals this suite does not
    // provide, and a name the package lacks degrades to a grey initial-letter
    // placeholder rather than crashing.
    expect(getChannelTypeConfig(CHANNEL_TYPE_ELEVENLABS).icon).toBe(
      getChannelTypeIcon(CHANNEL_TYPE_ELEVENLABS)
    )
  })

  it('defaults the Base URL to the ElevenLabs API host', () => {
    expect(getChannelTypeConfig(CHANNEL_TYPE_ELEVENLABS).defaultBaseUrl).toBe(
      'https://api.elevenlabs.io'
    )
  })

  it('accepts a channel that relies on the default Base URL', () => {
    const result = channelFormSchema.safeParse({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'ElevenLabs TTS',
      type: CHANNEL_TYPE_ELEVENLABS,
      base_url: '',
      key: 'sk_eleven_secret',
      models: 'eleven_multilingual_v2',
    })

    expect(result.success).toBe(true)
  })
})
