type ClassValue =
    | string
    | number
    | boolean
    | null
    | undefined
    | ClassValue[]
    | Record<string, unknown>

export default function clsx(...values: ClassValue[]): string {
    const classes: string[] = []
    for (const value of values) {
        if (!value) continue
        if (typeof value === 'string' || typeof value === 'number') {
            classes.push(String(value))
        } else if (Array.isArray(value)) {
            const nested = clsx(...value)
            if (nested) classes.push(nested)
        } else if (typeof value === 'object') {
            classes.push(...Object.keys(value).filter((key) => value[key]))
        }
    }
    return classes.join(' ')
}
