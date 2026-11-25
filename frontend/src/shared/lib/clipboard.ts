export function installClipboardFallback(): void {
    if (typeof window === 'undefined' || (window.isSecureContext && navigator.clipboard)) return

    Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: {
            writeText: (text: string) =>
                new Promise<void>((resolve, reject) => {
                    const active = document.activeElement
                    const textarea = document.createElement('textarea')
                    textarea.value = text
                    textarea.style.position = 'fixed'
                    textarea.style.left = '-999999px'
                    document.body.appendChild(textarea)
                    try {
                        textarea.focus({ preventScroll: true })
                        textarea.select()
                        if (document.execCommand('copy')) resolve()
                        else reject(new Error('Clipboard copy failed'))
                    } finally {
                        textarea.remove()
                        if (active instanceof HTMLElement) active.focus({ preventScroll: true })
                    }
                })
        }
    })
}
