import { isRecord } from '@shared/lib/guards'
import consola from '@shared/lib/logger'

const PROTECTED_ROOT_KEYS = new Set(['api', 'inbounds', 'metrics', 'snippets', 'stats'])

const replaceSnippetsInRoot = (
    config: Record<string, unknown>,
    snippetsMap: ReadonlyMap<string, unknown>
): void => {
    const names = config['snippets']

    delete config['snippets']

    if (!Array.isArray(names)) return

    const merged: Record<string, unknown> = {}

    for (const name of names) {
        if (typeof name !== 'string') continue
        const snippet = snippetsMap.get(name)

        if (!snippet) {
            consola.error(`Snippet ${name} not found`)
            continue
        }

        for (const part of Array.isArray(snippet) ? snippet : [snippet]) {
            if (!part || typeof part !== 'object' || Array.isArray(part)) continue

            Object.assign(merged, part)
        }
    }

    for (const [key, value] of Object.entries(merged)) {
        if (PROTECTED_ROOT_KEYS.has(key) || key in config) continue

        config[key] = value
    }
}
const replaceSnippetsInArray = (
    array: unknown[],
    snippetsMap: ReadonlyMap<string, unknown>
): void => {
    for (let i = array.length - 1; i >= 0; i--) {
        const item = array[i]
        if (!isRecord(item)) continue

        if (typeof item['snippet'] === 'string') {
            const snippet = snippetsMap.get(item['snippet'])

            if (snippet) {
                if (Array.isArray(snippet)) {
                    array.splice(i, 1, ...snippet)
                } else {
                    array[i] = snippet
                }
            } else {
                consola.error(`Snippet ${item['snippet']} not found`)
                array.splice(i, 1)
            }
        }
    }
}

export function prepareXrayConfig(
    text: string,
    snippets: ReadonlyMap<string, unknown>
): { config: string } | { error: string } {
    let config: unknown
    try {
        config = JSON.parse(text)
    } catch {
        return { error: 'Invalid JSON.' }
    }
    if (!isRecord(config)) return { error: 'Config must be a JSON object.' }

    replaceSnippetsInRoot(config, snippets)
    if (Array.isArray(config['outbounds'])) replaceSnippetsInArray(config['outbounds'], snippets)
    const routing = config['routing']
    if (isRecord(routing)) {
        for (const key of ['rules', 'balancers']) {
            const entries = routing[key]
            if (Array.isArray(entries)) replaceSnippetsInArray(entries, snippets)
        }
    }
    return { config: JSON.stringify(config) }
}
