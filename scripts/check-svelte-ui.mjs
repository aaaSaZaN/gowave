import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../frontend/src/svelte/', import.meta.url))
const rules = [
    {
        pattern: /\bwindow\s*\.\s*(confirm|alert|prompt)\s*\(/g,
        replacement: 'Use the shared confirmation or notification UI.'
    },
    {
        pattern: /<dialog\b/g,
        allowed: ['ui/Overlay.svelte', 'nodes/UsageFilters.svelte'],
        replacement: 'Use Overlay for dialogs and drawers.'
    },
    {
        pattern: /<select\b/g,
        replacement: 'Use SelectField for selection controls.'
    },
    {
        pattern: /<table\b/g,
        allowed: ['ui/DataTable.svelte'],
        replacement: 'Use DataTable for data tables.'
    },
    {
        pattern: /\bm_[0-9a-z]{7,8}\b/g,
        replacement: 'Use a semantic class or shared UI component.'
    }
]

let failures = 0
for (const entry of readdirSync(root, { recursive: true, withFileTypes: true })) {
    if (!entry.isFile() || !/\.(svelte|ts|css)$/.test(entry.name)) continue
    const file = join(entry.parentPath, entry.name)
    const path = relative(root, file)
    const source = readFileSync(file, 'utf8')
    for (const { pattern, allowed, replacement } of rules) {
        if (allowed?.includes(path)) continue
        for (const match of source.matchAll(pattern)) {
            const line = source.slice(0, match.index).split('\n').length
            console.error(`${path}:${line} ${match[0]} ${replacement}`)
            failures += 1
        }
    }
}
if (failures) process.exitCode = 1
