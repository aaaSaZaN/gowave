import type { CustomRemarksSchema, HwidSettingsSchema } from '@remnawave/backend-contract'
import type { z } from 'zod'

import { responseHeaders } from './headers'

export type CustomRemarks = z.infer<typeof CustomRemarksSchema>
export type HwidSettings = z.infer<typeof HwidSettingsSchema>

export const subscriptionOptions = {
    serveJsonAtBaseSubscription: 'subscription-settings.widget.serve-json-at-base-subscription',
    isShowCustomRemarks: 'subscription-tabs.widget.show-custom-remarks',
    randomizeHosts: 'subscription-tabs.widget.randomize-hosts'
} as const

export function textValue(data: FormData, key: string): string {
    const value = data.get(key)
    return typeof value === 'string' ? value : ''
}

export function readHwidSettings(data: FormData): HwidSettings {
    return {
        enabled: data.has('enabled'),
        fallbackDeviceLimit: Number(textValue(data, 'fallbackDeviceLimit')),
        maxDevicesAnnounce: textValue(data, 'maxDevicesAnnounce').trim() || null
    }
}

export function readRemarks(data: FormData): CustomRemarks {
    const lines = (name: string) =>
        data
            .getAll(name)
            .flatMap((value) => String(value).split('\n'))
            .map((line) => line.trim())
            .filter(Boolean)

    return {
        expiredUsers: lines('expiredUsers'),
        limitedUsers: lines('limitedUsers'),
        disabledUsers: lines('disabledUsers'),
        emptyHosts: lines('emptyHosts'),
        HWIDMaxDevicesExceeded: lines('HWIDMaxDevicesExceeded'),
        HWIDNotSupported: lines('HWIDNotSupported')
    }
}

export function readHeaders(data: FormData): Record<string, string> {
    const values = data.getAll('headerValue')
    return responseHeaders(
        data.getAll('headerName').map((key, index) => ({
            key: String(key),
            value: String(values[index] ?? '')
        }))
    )
}
