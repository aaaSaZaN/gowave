interface ColorOptions {
    lightness?: number | number[]
    saturation?: number | number[]
}

function hashString(value: string): number {
    let hash = 2166136261
    for (const char of value) hash = Math.imul(hash ^ char.codePointAt(0)!, 16777619)
    return hash >>> 0
}

function pick(value: number | number[] | undefined, hash: number): number {
    if (typeof value === 'number') return value
    const choices = value?.length ? value : [0.35, 0.5, 0.65]
    return choices[hash % choices.length]
}

export default class ColorHash {
    constructor(private options: ColorOptions = {}) {}

    hex(value: string): string {
        const hash = hashString(value)
        const hue = (hash % 359) / 30
        const saturation = pick(this.options.saturation, hash >>> 8)
        const lightness = pick(this.options.lightness, hash >>> 16)
        const amplitude = saturation * Math.min(lightness, 1 - lightness)
        const channel = (offset: number) => {
            const k = (offset + hue) % 12
            const value = lightness - amplitude * Math.max(-1, Math.min(k - 3, 9 - k, 1))
            return Math.round(255 * value)
                .toString(16)
                .padStart(2, '0')
        }
        return `#${channel(0)}${channel(8)}${channel(4)}`
    }
}

const idColors = new ColorHash({ lightness: 0.65, saturation: 0.6 })
export const colorFromId = (id: number | bigint): string => idColors.hex(String(id))
