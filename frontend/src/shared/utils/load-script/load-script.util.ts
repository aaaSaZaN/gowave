const pending = new Map<string, Promise<void>>()

// Classic scripts that define globals (such as Go's wasm_exec.js) are only needed on
// a few pages, so they are injected on demand instead of from index.html.
export const loadScript = (src: string): Promise<void> => {
    let promise = pending.get(src)

    if (!promise) {
        promise = new Promise<void>((resolve, reject) => {
            const script = document.createElement('script')
            script.src = src
            script.async = true
            script.onload = () => resolve()
            script.onerror = () => {
                pending.delete(src)
                script.remove()
                reject(new Error(`Failed to load ${src}`))
            }
            document.head.appendChild(script)
        })
        pending.set(src, promise)
    }

    return promise
}
