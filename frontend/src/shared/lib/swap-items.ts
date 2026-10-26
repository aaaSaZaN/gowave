export function swapItems<T extends object>(
    items: readonly T[],
    first: number,
    second: number
): T[] {
    const result = [...items]
    const left = result[first]
    const right = result[second]
    if (!left || !right) return result
    result[first] = right
    result[second] = left
    return result
}
