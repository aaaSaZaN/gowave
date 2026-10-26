/* eslint-disable no-param-reassign */
function formatBytes(value: number, iec: boolean, bits: boolean = false): string {
    if (Number.isNaN(value)) return 'null'
    const base = iec ? 1024 : 1000
    const amount = bits ? value * 8 : value
    const units = iec
        ? ['', 'Ki', 'Mi', 'Gi', 'Ti', 'Pi', 'Ei', 'Zi', 'Yi']
        : ['', 'K', 'M', 'G', 'T', 'P', 'E', 'Z', 'Y']
    const index =
        Number.isFinite(amount) && Math.abs(amount) >= 1
            ? Math.min(Math.floor(Math.log(Math.abs(amount)) / Math.log(base)), units.length - 1)
            : 0
    return `${(amount / base ** index).toFixed(2)} ${units[index]}${bits ? 'b' : 'B'}`
}

export function prettifyBytesUtil(
    bytesInput: null | number | string | undefined,
    returnZero: true
): string
export function prettifyBytesUtil(
    bytesInput: null | number | string | undefined,
    returnZero?: false
): string | undefined
export function prettifyBytesUtil(
    bytesInput: null | number | string | undefined,
    returnZero: boolean = false
): string | undefined {
    if (!bytesInput) {
        return returnZero ? '0' : undefined
    }
    if (typeof bytesInput === 'string') {
        bytesInput = Number(bytesInput)
    }

    return formatBytes(bytesInput, true)
}

export function prettySiBytesUtil(
    bytesInput: number | string | undefined,
    returnZero: boolean = false
): string | undefined {
    if (!bytesInput) {
        return returnZero ? '0 B' : undefined
    }
    if (typeof bytesInput === 'string') {
        bytesInput = Number(bytesInput)
    }

    return formatBytes(bytesInput, false)
}

export function prettySiRealtimeBytesUtil(
    bytesInput: number | string | undefined,
    returnZero: boolean = false,
    withSeconds: boolean = false
): string | undefined {
    if (!bytesInput) {
        return returnZero ? '0 B/s' : undefined
    }
    if (typeof bytesInput === 'string') {
        bytesInput = Number(bytesInput)
    }

    const res = formatBytes(bytesInput, false, true)

    return `${res}${withSeconds ? '/s' : ''}`
}
