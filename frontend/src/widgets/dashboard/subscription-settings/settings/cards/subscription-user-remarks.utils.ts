import type { SubscriptionSettings } from '@shared/api/schemas/subscription-settings.schema'

export const processRemarks = (remarksData: string | string[] | undefined): string[] => {
    if (!remarksData) return ['']
    if (typeof remarksData === 'string') {
        const filtered = remarksData.split('\n').filter(Boolean)
        return filtered.length > 0 ? filtered : ['']
    }
    return Array.isArray(remarksData) && remarksData.length > 0 ? remarksData : ['']
}

export const computeRemarks = (settings: SubscriptionSettings) => {
    return {
        expired: processRemarks(settings.customRemarks.expiredUsers),
        limited: processRemarks(settings.customRemarks.limitedUsers),
        disabled: processRemarks(settings.customRemarks.disabledUsers),
        emptyHosts: processRemarks(settings.customRemarks.emptyHosts),
        HWIDMaxDevicesExceeded: processRemarks(settings.customRemarks.HWIDMaxDevicesExceeded),
        HWIDNotSupported: processRemarks(settings.customRemarks.HWIDNotSupported)
    }
}
