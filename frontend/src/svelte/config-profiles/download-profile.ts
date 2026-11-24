export function downloadProfile(name: string, json: string): void {
    const url = URL.createObjectURL(new Blob([json], { type: 'application/json' }))
    const link = document.createElement('a')
    link.href = url
    link.download = `${name}.json`
    link.click()
    setTimeout(() => URL.revokeObjectURL(url), 0)
}
