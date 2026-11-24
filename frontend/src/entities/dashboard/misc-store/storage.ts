import { isRecord } from '@shared/lib/guards'

import type { IState } from './interfaces'

export const MISC_STORAGE_KEY = 'miscStore'
export const MISC_STORAGE_VERSION = 3

export function migrateMiscPreferences(value: unknown, version: number): IState {
    const state = isRecord(value) ? value : {}
    return {
        disclaimerAccepted: version !== 1 && version !== 2 && state['disclaimerAccepted'] === true,
        mobileWarningClosed: state['mobileWarningClosed'] === true,
        srrAdvancedModalClosed: version !== 1 && state['srrAdvancedModalClosed'] === true
    }
}

export function readMiscPreferences(storage: Pick<Storage, 'getItem'>): IState {
    try {
        const stored: unknown = JSON.parse(storage.getItem(MISC_STORAGE_KEY) ?? 'null')
        if (isRecord(stored)) {
            const version = stored['version']
            return migrateMiscPreferences(
                stored['state'],
                typeof version === 'number' ? version : MISC_STORAGE_VERSION
            )
        }
    } catch {
        // Start with defaults when saved preferences are unavailable.
    }
    return migrateMiscPreferences(null, MISC_STORAGE_VERSION)
}

export function writeMiscPreferences(
    storage: Pick<Storage, 'getItem' | 'setItem'>,
    changes: Partial<IState>
): void {
    const state = { ...readMiscPreferences(storage), ...changes }
    storage.setItem(MISC_STORAGE_KEY, JSON.stringify({ state, version: MISC_STORAGE_VERSION }))
}
