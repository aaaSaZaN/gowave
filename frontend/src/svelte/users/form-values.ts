export function textField(data: FormData, name: string): string {
    const value = data.get(name)
    return typeof value === 'string' ? value : ''
}

export function numberField(data: FormData, name: string): number | undefined {
    const value = textField(data, name).trim()
    return value ? Number(value) : undefined
}

export function localDateTime(date: Date): string {
    const local = new Date(date.valueOf() - date.getTimezoneOffset() * 60_000)
    return local.toISOString().slice(0, 16)
}

export function isoDateTime(value: string): string {
    const date = new Date(value)
    if (Number.isNaN(date.valueOf())) throw new Error('Enter a valid expiration date')
    return date.toISOString()
}
