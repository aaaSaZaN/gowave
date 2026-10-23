import type { Monaco } from '@monaco-editor/react'

import { loader } from '@monaco-editor/react'
import { useEffect, useState } from 'react'

let monacoPromise: null | Promise<Monaco> = null

// Monaco is several megabytes, so it is fetched only when an editor is first shown.
// The bundled instance must be configured before loader.init(), which otherwise
// falls back to a CDN copy.
export const loadMonaco = (): Promise<Monaco> => {
    monacoPromise ??= import('./setup-monaco')
        .then(() => loader.init())
        .catch((error: unknown) => {
            monacoPromise = null
            throw error
        })

    return monacoPromise
}

export function useMonaco(): Monaco | null {
    const [monaco, setMonaco] = useState<Monaco | null>(null)

    useEffect(() => {
        let isActive = true

        loadMonaco().then((instance) => {
            if (isActive) setMonaco(instance)
        })

        return () => {
            isActive = false
        }
    }, [])

    return monaco
}
