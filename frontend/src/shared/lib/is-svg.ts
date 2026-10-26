// Identification only; sanitizeSvg remains responsible for safe rendering.
export default function isSvg(value: string): boolean {
    const document = new DOMParser().parseFromString(value, 'image/svg+xml')
    return (
        document.documentElement.localName === 'svg' &&
        document.getElementsByTagName('parsererror').length === 0
    )
}
