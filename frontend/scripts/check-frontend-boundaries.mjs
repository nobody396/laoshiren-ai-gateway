import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'

const root = resolve('src')
const apiRoot = join(root, 'api')
const extensions = ['.ts', '.tsx', '.js', '.vue']
const files = []
function walk(dir) {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name)
    if (statSync(path).isDirectory()) walk(path)
    else if (extensions.some((extension) => path.endsWith(extension)) && !path.includes('/__tests__/')) files.push(path)
  }
}
walk(apiRoot)

const importPattern = /(?:import|export)\s+(?:[^'";]+?\s+from\s+)?['"]([^'"]+)['"]/g
const graph = new Map()
const failures = []
function resolveImport(from, specifier) {
  if (!(specifier.startsWith('@/api') || specifier.startsWith('./') || specifier.startsWith('../'))) return null
  const base = specifier.startsWith('@/') ? join(root, specifier.slice(2)) : resolve(dirname(from), specifier)
  for (const candidate of [base, ...extensions.map((extension) => base + extension), ...extensions.map((extension) => join(base, 'index' + extension))]) {
    if (existsSync(candidate) && !statSync(candidate).isDirectory()) return candidate
  }
  return null
}

for (const file of files) {
  const source = readFileSync(file, 'utf8')
  if (/from\s+['"]@\/(?:stores|router|components|views)\b/.test(source)) {
    failures.push(`${relative(root, file)} imports UI/runtime ownership`)
  }
  const edges = []
  for (const match of source.matchAll(importPattern)) {
    const target = resolveImport(file, match[1])
    if (target?.startsWith(apiRoot)) edges.push(target)
  }
  graph.set(file, edges)
}

const visiting = new Set()
const visited = new Set()
function visit(file, path = []) {
  if (visiting.has(file)) {
    failures.push(`API cycle: ${[...path, file].map((item) => relative(apiRoot, item)).join(' -> ')}`)
    return
  }
  if (visited.has(file)) return
  visiting.add(file)
  for (const target of graph.get(file) ?? []) visit(target, [...path, file])
  visiting.delete(file)
  visited.add(file)
}
for (const file of graph.keys()) visit(file)

if (failures.length) {
  failures.forEach((failure) => console.error(failure))
  process.exit(1)
}
console.log(`frontend boundaries passed (${files.length} API modules)`)
