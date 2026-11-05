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
import i18next from 'i18next'
import { expect, it } from 'vitest'

import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'

import { renderAuditContent } from '../format'

it('renders a content read audit row in the viewer language', async () => {
  const i18n = i18next.createInstance()
  await i18n.init({
    lng: 'en',
    resources: { en, zh },
    nsSeparator: false,
    interpolation: { escapeValue: false },
  })
  const other = {
    op: {
      action: 'log_content.view',
      params: { request_id: 'req-42', owner_user_id: 7 },
    },
  }
  expect(renderAuditContent(other, i18n.t)).toBe(
    'Viewed the content of request req-42'
  )
  await i18n.changeLanguage('zh')
  expect(renderAuditContent(other, i18n.t)).toBe('查看了请求 req-42 的内容')
})
