import { useGetConfigProfile, useGetSnippets } from '@shared/api/hooks'
import { ROUTES } from '@shared/constants'
import { consola } from '@shared/lib/logger'
import { runAsync } from '@shared/lib/run-async'
import { LoadingScreen } from '@shared/ui'
import { fetchWithProgress } from '@shared/utils/fetch-with-progress'
import { loadScript } from '@shared/utils/load-script'
import { useCallback, useLayoutEffect, useRef, useState } from 'react'
import { Navigate, useParams } from 'react-router'
import { app } from 'src/config'

import { ConfigProfileByUuidPageComponent } from '../components/config-profile-by-uuid.page.component'

const WASM_INIT_TIMEOUT_MS = 60_000

export function ConfigProfileByUuidPageConnector() {
    const { uuid } = useParams()

    // The validator is Xray-core compiled to WebAssembly (~60 MB, ~12 MB gzip). It loads in the
    // background so the editor opens immediately; validation starts once it is ready.
    const [wasmProgress, setWasmProgress] = useState(0)
    const [isWasmLoading, setIsWasmLoading] = useState(true)
    const [isWasmCrashed, setIsWasmCrashed] = useState(false)
    const wasmBytesCache = useRef<ArrayBuffer | null>(null)

    const { data: configProfile, isLoading: isConfigProfileLoading } = useGetConfigProfile({
        route: { uuid: uuid ?? '' },
        rQueryParams: {
            enabled: !!uuid,
            refetchOnWindowFocus: false
        }
    })

    const { data: snippets, isLoading: isSnippetsLoading } = useGetSnippets({})

    const initWasm = useCallback(
        () =>
            loadScript(app.configEditor.wasmJsUrl)
                .then(async () => {
                    const Go = window.Go
                    if (!Go) throw new Error('WASM runtime is not initialized')
                    const go = new Go()
                    const wasmInitialized = new Promise<void>((resolve, reject) => {
                        const timeout = setTimeout(
                            () => reject(new Error('WASM module did not initialize in time')),
                            WASM_INIT_TIMEOUT_MS
                        )
                        window.onWasmInitialized = () => {
                            clearTimeout(timeout)
                            consola.info('WASM module initialized')
                            resolve()
                        }
                    })

                    let wasmBytes: ArrayBuffer
                    if (wasmBytesCache.current) {
                        wasmBytes = wasmBytesCache.current
                    } else {
                        wasmBytes = await fetchWithProgress(
                            app.configEditor.wasmUrl,
                            setWasmProgress
                        )
                        wasmBytesCache.current = wasmBytes
                    }

                    const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject)

                    go.run(instance).then(() => {
                        consola.warn('WASM module exited unexpectedly')
                        setIsWasmCrashed(true)
                    })

                    await wasmInitialized

                    if (typeof window.XrayParseConfig !== 'function') {
                        throw new Error('XrayParseConfig not initialized')
                    }
                })
                .then(
                    () => {
                        setIsWasmLoading(false)
                    },
                    (err: unknown) => {
                        consola.error('WASM initialization error:', err)
                        setIsWasmCrashed(true)
                        setIsWasmLoading(false)
                    }
                ),
        []
    )

    const restartWasm = useCallback(() => {
        setIsWasmLoading(true)
        setIsWasmCrashed(false)
        void initWasm()
    }, [initWasm])

    useLayoutEffect(() => {
        runAsync(initWasm())

        return () => {
            delete window.onWasmInitialized
        }
    }, [initWasm])

    if (!uuid) {
        return <Navigate to={ROUTES.DASHBOARD.MANAGEMENT.CONFIG_PROFILES} />
    }

    if (isConfigProfileLoading || !configProfile || isSnippetsLoading || !snippets) {
        return <LoadingScreen />
    }

    return (
        <ConfigProfileByUuidPageComponent
            configProfile={configProfile}
            isWasmCrashed={isWasmCrashed}
            isWasmLoading={isWasmLoading}
            onRestartWasm={restartWasm}
            snippets={snippets}
            wasmProgress={wasmProgress}
        />
    )
}
