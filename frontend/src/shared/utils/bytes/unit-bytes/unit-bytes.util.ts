export const IEC_UNITS = ['KiB', 'MiB', 'GiB', 'TiB', 'PiB'] as const

export type TIecUnit = (typeof IEC_UNITS)[number]

const FACTOR: Record<TIecUnit, number> = {
    KiB: 1024,
    MiB: 1024 ** 2,
    GiB: 1024 ** 3,
    TiB: 1024 ** 4,
    PiB: 1024 ** 5
}

export function unitToBytesUtil(
    value: number | string | undefined,
    unit: TIecUnit
): number | undefined {
    if (value === undefined || value === '' || value === null) return undefined

    const num = typeof value === 'string' ? Number(value) : value
    if (Number.isNaN(num)) return undefined

    return Math.round(num * FACTOR[unit])
}

export function bytesToUnitUtil(
    bytes: null | number | undefined,
    unit: TIecUnit,
    fractionDigits = 2
): number | undefined {
    if (bytes === undefined || bytes === null) return undefined

    return Number((bytes / FACTOR[unit]).toFixed(fractionDigits))
}

export function bestFitIecUnitUtil(bytes: null | number | undefined): TIecUnit {
    if (!bytes || bytes <= 0) return 'GiB'

    for (const unit of [...IEC_UNITS].reverse()) {
        if (bytes >= FACTOR[unit]) return unit
    }

    return 'KiB'
}
