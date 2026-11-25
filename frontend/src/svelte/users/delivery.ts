import { renderSVG } from 'uqr'

export function connectionKeyLabel(key: string, fallback: string): string {
    const fragment = key.indexOf('#')
    if (fragment === -1 || fragment === key.length - 1) return fallback
    const remark = key.slice(fragment + 1)
    try {
        return decodeURIComponent(remark)
    } catch {
        return remark
    }
}

export function qrImage(data: string, foreground: string, background: string): string {
    const svg = renderSVG(data, { blackColor: foreground, whiteColor: background })
    return `data:image/svg+xml,${encodeURIComponent(svg)}`
}
