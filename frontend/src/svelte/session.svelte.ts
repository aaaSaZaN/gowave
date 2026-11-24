import { setAuthorizationToken } from '@shared/api/axios'

import { readSessionToken, writeSessionToken } from './session-storage'

const storedToken = restoreToken()
let token = $state(storedToken)
setAuthorizationToken(storedToken)

function restoreToken(): string {
    try {
        return readSessionToken(window.localStorage)
    } catch {
        return ''
    }
}

export const session = {
    get authenticated() {
        return token !== ''
    },
    setToken(value: string) {
        token = value
        setAuthorizationToken(value)
        try {
            writeSessionToken(window.localStorage, value)
        } catch {
            // The active session works with browser storage disabled.
        }
    }
}
