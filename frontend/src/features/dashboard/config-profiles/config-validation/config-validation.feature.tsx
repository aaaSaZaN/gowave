import type { GetSnippetsCommand } from '@remnawave/backend-contract'
import { isRecord } from '@shared/lib/guards'
import consola from '@shared/lib/logger'
import dayjs from 'dayjs'
import type { editor } from 'monaco-editor'
import type { RefObject } from 'react'

const PROTECTED_ROOT_KEYS = new Set(['api', 'inbounds', 'metrics', 'snippets', 'stats'])

const replaceSnippetsInRoot = (
    config: Record<string, unknown>,
    snippetsMap: Map<string, unknown>
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
const replaceSnippetsInArray = (array: unknown[], snippetsMap: Map<string, unknown>): void => {
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

export const ConfigValidationFeature = {
    validate: (
        editorRef: RefObject<editor.IStandaloneCodeEditor | null>,

        setResult: (message: string) => void,
        setIsConfigValid: (isValid: boolean) => void,
        snippetsMap: Map<
            string,
            GetSnippetsCommand.Response['response']['snippets'][number]['snippet']
        >
    ) => {
        try {
            if (!editorRef.current) return

            const currentValue = editorRef.current.getValue()
            let clonedCurrentValue: unknown
            try {
                clonedCurrentValue = JSON.parse(currentValue)
            } catch {
                setResult(`${dayjs().format('HH:mm:ss')} | Invalid JSON.`)
                setIsConfigValid(false)
                return
            }

            if (!isRecord(clonedCurrentValue)) {
                setResult(`${dayjs().format('HH:mm:ss')} | Config must be a JSON object.`)
                setIsConfigValid(false)
                return
            }
            replaceSnippetsInRoot(clonedCurrentValue, snippetsMap)

            if (Array.isArray(clonedCurrentValue['outbounds'])) {
                replaceSnippetsInArray(clonedCurrentValue['outbounds'], snippetsMap)
            }
            const routing = clonedCurrentValue['routing']
            if (isRecord(routing)) {
                for (const key of ['rules', 'balancers']) {
                    if (Array.isArray(routing[key]))
                        replaceSnippetsInArray(routing[key], snippetsMap)
                }
            }

            if (typeof window.XrayParseConfig !== 'function') {
                setResult(
                    `${dayjs().format('HH:mm:ss')} | Xray validator (WASM) is not loaded: the config was not checked.`
                )
                setIsConfigValid(false)
                return
            }

            const validationResult = window.XrayParseConfig(JSON.stringify(clonedCurrentValue))

            setResult(
                `${dayjs().format('HH:mm:ss')} | ${validationResult || 'Xray config is valid.'}`
            )
            setIsConfigValid(!validationResult)
        } catch (err: unknown) {
            const message = err instanceof Error ? err.message : String(err)
            if (message?.includes('Go program has already exited')) {
                setResult(`${dayjs().format('HH:mm:ss')} | WASM module crashed, restarting...`)
            } else {
                setResult(`${dayjs().format('HH:mm:ss')} | Validation error: ${message}`)
            }
            setIsConfigValid(false)
        }
    }
}
