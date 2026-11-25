export const SESSION_STORAGE_KEY = 'sessionStore'

export function readSessionToken(storage: Pick<Storage, 'getItem'>): string {
    try {
        const data: unknown = JSON.parse(storage.getItem(SESSION_STORAGE_KEY) ?? 'null')
        if (typeof data !== 'object' || data === null || !('state' in data)) return ''
        const { state } = data
        if (typeof state !== 'object' || state === null || !('token' in state)) return ''
        return typeof state.token === 'string' ? state.token : ''
    } catch {
        return ''
    }
}

export function writeSessionToken(storage: Pick<Storage, 'setItem'>, token: string): void {
    // Zustand persists this envelope. Keep it readable by existing installations.
    storage.setItem(SESSION_STORAGE_KEY, JSON.stringify({ state: { token }, version: 0 }))
}
