import type { UpdateNodeCommand } from '@remnawave/backend-contract'
import type { TTrafficTrackingStart } from '@shared/api/schemas/node.schema'

export type NodeField =
    | Exclude<keyof UpdateNodeCommand.RequestBody, 'uuid'>
    | 'trafficTrackingStart'
export type NodePatch = Partial<Record<NodeField, unknown>>

export const associationFields = ['providerUuid', 'activePluginUuid', 'tags', 'note'] as const
export const multiplierFields = ['consumptionMultiplier', 'nodeConsumptionMultiplier'] as const

export function fieldText(form: FormData, name: string): string {
    const value = form.get(name)
    return typeof value === 'string' ? value.trim() : ''
}

export function readConfigProfile(form: FormData) {
    return {
        activeConfigProfileUuid: fieldText(form, 'profile'),
        activeInbounds: form.getAll('inbounds').filter((value) => typeof value === 'string')
    }
}

export function readNodePatch(
    form: FormData,
    contains: (key: NodeField) => boolean,
    mode: 'create' | 'edit' | 'bulk'
): NodePatch {
    const patch: NodePatch = {}
    const numberFields = [
        'port',
        'trafficLimitBytes',
        'notifyPercent',
        'trafficResetDay',
        ...multiplierFields
    ] as const
    for (const key of numberFields) {
        if (!contains(key)) continue
        const raw = fieldText(form, key)
        if (raw) patch[key] = Number(raw)
    }
    for (const key of ['name', 'address', 'countryCode'] as const) {
        if (contains(key)) patch[key] = fieldText(form, key)
    }
    for (const key of ['proxyUrl', 'providerUuid', 'activePluginUuid'] as const) {
        if (contains(key)) patch[key] = fieldText(form, key) || null
    }
    if (contains('tags'))
        patch.tags = [
            ...new Set(
                fieldText(form, 'tags')
                    .toUpperCase()
                    .split(/[,\s]+/)
                    .filter(Boolean)
            )
        ]
    if (contains('note')) patch.note = fieldText(form, 'note') || (mode === 'create' ? '' : null)
    if (contains('isTrafficTrackingActive'))
        patch.isTrafficTrackingActive = form.has('isTrafficTrackingActive')
    if (contains('trafficTrackingStart'))
        patch.trafficTrackingStart = fieldText(
            form,
            'trafficTrackingStart'
        ) as TTrafficTrackingStart
    if (contains('configProfile')) patch.configProfile = readConfigProfile(form)
    return patch
}
