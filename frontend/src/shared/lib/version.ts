interface Version {
    numbers: [number, number, number]
    prerelease: string[]
    version: string
}

function parse(input: unknown): Version | null {
    if (typeof input !== 'string' || input.length > 256) return null
    const match = input
        .trim()
        .match(
            /^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([\da-zA-Z-]+(?:\.[\da-zA-Z-]+)*))?(?:\+([\da-zA-Z-]+(?:\.[\da-zA-Z-]+)*))?$/
        )
    if (!match) return null
    const numbers: [number, number, number] = [Number(match[1]), Number(match[2]), Number(match[3])]
    const prerelease = match[4]?.split('.') ?? []
    if (numbers.some((value) => !Number.isSafeInteger(value))) return null
    if (prerelease.some((value) => /^\d+$/.test(value) && value.length > 1 && value[0] === '0')) {
        return null
    }
    return {
        numbers,
        prerelease,
        version: `${numbers.join('.')}${match[4] ? `-${match[4]}` : ''}`
    }
}

export const valid = (input: unknown): string | null => parse(input)?.version ?? null
export const clean = (input: string): string | null => valid(input.trim().replace(/^[=v]+/, ''))

export function coerce(input: unknown): string | null {
    if (typeof input !== 'string' && typeof input !== 'number') return null
    const match = String(input).match(
        /(?:^|[^\d])(\d{1,16})(?:\.(\d{1,16}))?(?:\.(\d{1,16}))?(?:$|[^\d])/
    )
    return match ? valid(`${match[1]}.${match[2] ?? 0}.${match[3] ?? 0}`) : null
}

function compare(left: string, right: string): number {
    const a = parse(left)
    const b = parse(right)
    if (!a || !b) throw new Error('Invalid version')
    for (const i of [0, 1, 2] as const) {
        if (a.numbers[i] !== b.numbers[i]) return a.numbers[i] > b.numbers[i] ? 1 : -1
    }
    if (!a.prerelease.length || !b.prerelease.length) {
        return Math.sign(b.prerelease.length) - Math.sign(a.prerelease.length)
    }
    for (let i = 0; i < Math.max(a.prerelease.length, b.prerelease.length); i++) {
        const x = a.prerelease[i]
        const y = b.prerelease[i]
        if (x === y) continue
        if (x === undefined) return -1
        if (y === undefined) return 1
        const xNumeric = /^\d+$/.test(x)
        const yNumeric = /^\d+$/.test(y)
        if (xNumeric && yNumeric) return BigInt(x) > BigInt(y) ? 1 : -1
        if (xNumeric !== yNumeric) return xNumeric ? -1 : 1
        return x > y ? 1 : -1
    }
    return 0
}

export default {
    valid,
    clean,
    coerce,
    gt: (left: string, right: string) => compare(left, right) > 0,
    gte: (left: string, right: string) => compare(left, right) >= 0
}
