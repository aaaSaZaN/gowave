// Table accessors use dotted property paths; check a literal key first so
// response headers and other keys containing dots still work.
export default function get(object: unknown, path: PropertyKey): unknown {
    if (object === null || typeof object !== 'object') return undefined
    if (Object.hasOwn(object, path)) return (object as Record<PropertyKey, unknown>)[path]
    let value: unknown = object
    if (typeof path === 'symbol') return undefined
    for (const key of String(path).split('.')) {
        if (value === null || typeof value !== 'object' || !Object.hasOwn(value, key)) {
            return undefined
        }
        value = (value as Record<string, unknown>)[key]
    }
    return value
}
