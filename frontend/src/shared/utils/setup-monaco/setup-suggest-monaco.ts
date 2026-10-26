import { isRecord } from '@shared/lib/guards'
import type { editor } from 'monaco-editor'

const DETAILS_PATCHED = Symbol('gowave.suggestDetailsPatched')

export function setupSuggestWidget(editor: editor.IStandaloneCodeEditor): void {
    try {
        const contribution: unknown = editor.getContribution('editor.contrib.suggestController')
        if (!isRecord(contribution) || !isRecord(contribution['widget'])) return
        const suggestWidget = contribution['widget']['value']
        if (!isRecord(suggestWidget)) return

        const showDetails = suggestWidget['_setDetailsVisible']
        if (typeof showDetails === 'function') showDetails.call(suggestWidget, true)
        const persistedSize = suggestWidget['_persistedSize']
        if (isRecord(persistedSize) && typeof persistedSize['store'] === 'function') {
            persistedSize['store'].call(persistedSize, { width: 300, height: 300 })
        }
        const details = suggestWidget['_details']

        if (!isRecord(details) || DETAILS_PATCHED in details) return

        Object.defineProperty(details, DETAILS_PATCHED, { value: true })
        Object.defineProperty(details, '_userSize', {
            configurable: true,
            set: () => {},
            get: () => {
                const element = suggestWidget['element']
                const node = isRecord(element) ? element['domNode'] : null
                const anchor =
                    node instanceof HTMLElement ? node.getBoundingClientRect() : undefined
                const available = anchor
                    ? document.body.clientWidth - (anchor.left + anchor.width) - 24
                    : 640

                return { width: Math.max(320, Math.min(640, available)), height: 320 }
            }
        })

        const positionDetails = suggestWidget['_positionDetails']
        if (typeof positionDetails === 'function') positionDetails.call(suggestWidget)
    } catch {
        // silence
    }
}
