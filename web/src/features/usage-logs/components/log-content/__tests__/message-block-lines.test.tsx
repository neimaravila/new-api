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
import { render, screen } from '@testing-library/react'
import i18n from 'i18next'
import { afterEach, beforeAll, expect, it } from 'vitest'

import { logContentSchema } from '../../../data/log-content-schema'
import { ConversationTab } from '../conversation-tab'
import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import ptBR from '@/i18n/locales/pt-BR.json'
import ru from '@/i18n/locales/ru.json'
import vi from '@/i18n/locales/vi.json'
import zhTW from '@/i18n/locales/zh-TW.json'
import zh from '@/i18n/locales/zh.json'
import openaiFixture from './fixtures/view-openai.json'

function renderWithText(text: string) {
  const view = logContentSchema.parse({
    ...openaiFixture,
    conversation: {
      available: true,
      response_available: true,
      messages: [
        { role: 'user', source: 'request', blocks: [{ type: 'text', text }] },
      ],
    },
  })
  render(<ConversationTab view={view} />)
}

const original = i18n.language

beforeAll(() => {
  i18n.addResourceBundle('en', 'translation', en.translation, true, true)
  i18n.addResourceBundle('pt-BR', 'translation', ptBR.translation, true, true)
})

afterEach(async () => {
  await i18n.changeLanguage(original)
})

it('uses the singular form for a one-line message in English', async () => {
  await i18n.changeLanguage('en')
  renderWithText('single')
  expect(screen.getByText('1 line')).toBeVisible()
})

it('uses the plural form for a two-line message in English', async () => {
  await i18n.changeLanguage('en')
  renderWithText('first\nsecond')
  expect(screen.getByText('2 lines')).toBeVisible()
})

it('uses the singular form for a one-line message in pt-BR', async () => {
  await i18n.changeLanguage('pt-BR')
  renderWithText('single')
  expect(screen.getByText('1 linha')).toBeVisible()
})

it('uses the plural form for a two-line message in pt-BR', async () => {
  await i18n.changeLanguage('pt-BR')
  renderWithText('first\nsecond')
  expect(screen.getByText('2 linhas')).toBeVisible()
})

const locales: Record<string, Record<string, string>> = {
  en: en.translation,
  fr: fr.translation,
  ja: ja.translation,
  'pt-BR': ptBR.translation,
  ru: ru.translation,
  vi: vi.translation,
  'zh-TW': zhTW.translation,
  zh: zh.translation,
}

it.each(Object.keys(locales))(
  'defines every plural form of the line count in %s',
  (language) => {
    const categories = new Intl.PluralRules(language).resolvedOptions()
      .pluralCategories
    for (const category of categories) {
      expect(locales[language][`{{count}} lines_${category}`]).toBeTruthy()
    }
  }
)
