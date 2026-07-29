import fs from 'node:fs/promises'
import path from 'node:path'

const webRoot = process.cwd()
const localeDir = path.join(webRoot, 'src/i18n/locales')
const localeNames = (await fs.readdir(localeDir))
  .filter((n) => n.endsWith('.json'))
  .map((n) => n.replace(/\.json$/, ''))

// Collect t('...') keys from the reports components.
const compDir = path.join(webRoot, 'src/features/dashboard/components/reports')
const files = []
async function walk(dir) {
  for (const e of await fs.readdir(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name)
    if (e.isDirectory()) await walk(p)
    else if (/\.(tsx?|jsx?)$/.test(e.name)) files.push(p)
  }
}
await walk(compDir)

// Also include the dashboard index (section tab label) for the Reports entry.
files.push(path.join(webRoot, 'src/features/dashboard/index.tsx'))

const keys = new Set()
const skip = new Set([
  // dynamic data values rendered as t() but not locale keys
  'gpt-4o', 'claude', 'OpenAI', '350', '4', '500',
  'prompt 100 · completion 200 · cache 50',
])
for (const file of files) {
  const src = await fs.readFile(file, 'utf8')
  const re = /\bt\(\s*'((?:[^'\\]|\\.)*)'/g
  let m
  while ((m = re.exec(src)) !== null) {
    const k = m[1]
    if (!skip.has(k) && /[A-Za-z]/.test(k) && k.length > 1) keys.add(k)
  }
}

const missing = []
for (const locale of localeNames) {
  const json = JSON.parse(await fs.readFile(path.join(localeDir, `${locale}.json`), 'utf8'))
  for (const key of [...keys].sort()) {
    if (!Object.hasOwn(json.translation, key)) missing.push({ locale, key })
  }
}
if (missing.length) {
  console.log(`Missing ${missing.length} report i18n entries:`)
  for (const it of missing) console.log(`${it.locale}: ${it.key}`)
  process.exitCode = 1
} else {
  console.log(`All ${keys.size} report UI keys exist in ${localeNames.length} locales.`)
}
