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
import fs from 'node:fs/promises'
import path from 'node:path'

const repoRoot = path.resolve('..')
const localeDir = path.resolve('src/i18n/locales')
const localeNames = (await fs.readdir(localeDir))
  .filter((name) => name.endsWith('.json'))
  .map((name) => name.replace(/\.json$/, ''))
  .sort((a, b) => a.localeCompare(b))

const files = [
  path.join(repoRoot, 'service/dashboard.go'),
  path.join(repoRoot, 'service/dashboard_insights.go'),
]

const keys = new Set()
for (const file of files) {
  const source = await fs.readFile(file, 'utf8')
  for (const regex of [
    /dashboardMessage(?:Ptr)?\(\s*"((?:[^"\\]|\\.)*)"/g,
    /Title:\s*"((?:[^"\\]|\\.)*)"/g,
    /Description:\s*"((?:[^"\\]|\\.)*)"/g,
    /MetricLabel:\s*"((?:[^"\\]|\\.)*)"/g,
  ]) {
    let match
    while ((match = regex.exec(source)) !== null) {
      keys.add(JSON.parse(`"${match[1]}"`))
    }
  }
}

const missing = []
for (const locale of localeNames) {
  const json = JSON.parse(await fs.readFile(path.join(localeDir, `${locale}.json`), 'utf8'))
  for (const key of [...keys].sort((a, b) => a.localeCompare(b))) {
    if (!Object.hasOwn(json.translation, key)) missing.push({ locale, key })
  }
}

if (missing.length > 0) {
  console.log(`Missing ${missing.length} dashboard i18n entries:`)
  for (const item of missing) console.log(`${item.locale}: ${item.key}`)
  process.exitCode = 1
} else {
  console.log(`All ${keys.size} backend dashboard keys exist in ${localeNames.length} locales.`)
}
