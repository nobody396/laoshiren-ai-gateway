import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = dirname(fileURLToPath(import.meta.url))
const frontendRoot = resolve(scriptDir, '..')
const contentDir = resolve(frontendRoot, 'src/docs/content')
const configSource = readFileSync(resolve(frontendRoot, 'src/docs/config.ts'), 'utf8')
const itemPattern = /\{\s*title:\s*'((?:\\'|[^'])+)'\s*,\s*slug:\s*'((?:\\'|[^'])+)'[\s\S]*?description:\s*'((?:\\'|[^'])+)'[\s\S]*?\}/g
const items = [...configSource.matchAll(itemPattern)].map((match) => ({
  title: match[1].replace(/\\'/g, "'"),
  slug: match[2].replace(/\\'/g, "'"),
}))
const slugs = new Set(items.map((item) => item.slug))
const failures = []

if (items.length !== slugs.size) failures.push('docsConfig contains duplicate slugs')
if (items.length !== 17) failures.push(`expected 17 primary docs, found ${items.length}`)

for (const item of items) {
  const path = resolve(contentDir, `${item.slug}.md`)
  if (!existsSync(path)) {
    failures.push(`${item.slug}: markdown file is missing`)
    continue
  }

  const markdown = readFileSync(path, 'utf8')
  const heading = markdown.match(/^#\s+(.+)$/m)?.[1]?.trim()
  if (heading !== item.title) {
    failures.push(`${item.slug}: H1 must be "${item.title}", found "${heading || '<missing>'}"`)
  }
  if (/CC[ -]?Switch/i.test(markdown)) {
    failures.push(`${item.slug}: primary docs must not depend on CC Switch`)
  }
  if (/\bsk-[A-Za-z0-9_-]{8,}\b/.test(markdown)) {
    failures.push(`${item.slug}: possible API key committed in documentation`)
  }

  for (const target of markdown.matchAll(/\]\(([^)]+)\)/g)) {
    const href = target[1].trim()
    if (!href || /^(?:https?:\/\/|mailto:|tel:|#)/.test(href)) continue
    const linkedSlug = href.split('#')[0]
    if (linkedSlug && !slugs.has(linkedSlug)) {
      failures.push(`${item.slug}: unresolved primary-doc link ${href}`)
    }
  }

  if (item.slug.startsWith('integration-')) {
    for (const required of ['**配置状态：**', '## 1.', '## 3.', '## 4.']) {
      const isPending = /\*\*配置状态：\*\* (?:即将支持|实验支持)/.test(markdown)
      if (!isPending && !markdown.includes(required)) {
        failures.push(`${item.slug}: verified integration is missing ${required}`)
      }
    }
  }
}

if (failures.length > 0) {
  console.error(failures.map((failure) => `- ${failure}`).join('\n'))
  process.exit(1)
}

console.log(`docs content check passed (${items.length} primary pages)`)
