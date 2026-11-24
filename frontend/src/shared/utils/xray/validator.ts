import { fetchWithProgress } from '@shared/utils/fetch-with-progress'
import { loadScript } from '@shared/utils/load-script'
import { app } from 'src/config'

type ValidatorState =
    | { status: 'idle' }
    | { status: 'loading'; progress: number }
    | { status: 'ready' }
    | { status: 'failed'; error: string }

const listeners = new Set<(state: ValidatorState) => void>()
let state: ValidatorState = { status: 'idle' }
let startup: Promise<void> | undefined
let wasmBytes: ArrayBuffer | undefined

function publish(value: ValidatorState): void {
    state = value
    for (const listener of listeners) listener(state)
}

function fail(reason: unknown): void {
    const error = reason instanceof Error ? reason.message : String(reason)
    publish({ status: 'failed', error })
}

async function initialize(): Promise<void> {
    await loadScript(app.configEditor.wasmJsUrl)
    const Go = window.Go
    if (!Go) throw new Error('Xray validator runtime could not start')

    wasmBytes ??= await fetchWithProgress(app.configEditor.wasmUrl, (progress) => {
        publish({ status: 'loading', progress })
    })
    publish({ status: 'loading', progress: 100 })
    const go = new Go()
    const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject)
    let notifyInitialized: (() => void) | undefined
    let timeout: ReturnType<typeof setTimeout> | undefined

    const initialized = new Promise<void>((resolve, reject) => {
        timeout = setTimeout(
            () => reject(new Error('Xray validator initialization timed out')),
            60_000
        )
        notifyInitialized = resolve
        window.onWasmInitialized = resolve
    })

    try {
        const stopped = go.run(instance).then(() => {
            throw new Error('Xray validator exited')
        })
        await Promise.race([initialized, stopped])
        if (typeof window.XrayParseConfig !== 'function') {
            throw new Error('Xray validator did not export its parser')
        }
        publish({ status: 'ready' })
        void stopped.catch(fail)
    } finally {
        clearTimeout(timeout)
        if (window.onWasmInitialized === notifyInitialized) delete window.onWasmInitialized
    }
}

export function getXrayValidatorState(): ValidatorState {
    return state
}

export function subscribeXrayValidator(listener: (value: ValidatorState) => void): () => void {
    listeners.add(listener)
    listener(state)
    return () => {
        listeners.delete(listener)
    }
}

export function startXrayValidator(): void {
    if (startup || state.status === 'ready') return
    publish({ status: 'loading', progress: 0 })
    startup = initialize()
        .catch(fail)
        .finally(() => {
            startup = undefined
        })
}
