export interface HeaderRow {
    key: string
    value: string
}

// These constraints match the React editor and permit encoded multiline values.
const headerName = /^[!#$%&'*+\-.0-9A-Z^_`a-z|~]+$/
const headerValue = /^$|^[\x21-\x7E]([\x20-\x7E]*[\x21-\x7E])?$/

function normalizeHeaderName(key: string): string {
    return key.trim().toLowerCase()
}

function validateHeaderName(key: string): void {
    if (!headerName.test(key)) throw new Error(`Invalid header name: ${key}`)
}

export function responseHeaders(rows: HeaderRow[]): Record<string, string> {
    const normalized = new Map<string, string>()
    for (const row of rows) {
        const key = normalizeHeaderName(row.key)
        if (key) normalized.set(key, row.value.trim())
    }
    for (const [key, value] of normalized) {
        validateHeaderName(key)
        if (value.startsWith('rwEncodeBase64:')) continue
        if (value.includes('\n')) {
            throw new Error(`Multiline value of "${key}" requires the rwEncodeBase64: prefix`)
        }
        if (!headerValue.test(value)) throw new Error(`Invalid header value: ${value}`)
    }
    return Object.fromEntries(normalized)
}

export function removedHeaderNames(input: string): string[] {
    const keys = [
        ...new Set(
            input
                .split(/[,\s]+/)
                .map(normalizeHeaderName)
                .filter(Boolean)
        )
    ]
    for (const key of keys) validateHeaderName(key)
    return keys
}
