export type Move = 'top' | 'up' | 'down' | 'bottom'

export function moveSelection(ids: string[], selected: ReadonlySet<string>, move: Move): string[] {
    if (move === 'top' || move === 'bottom') {
        const moved = ids.filter((id) => selected.has(id))
        const remaining = ids.filter((id) => !selected.has(id))
        return move === 'top' ? [...moved, ...remaining] : [...remaining, ...moved]
    }

    const next = ids.slice()
    const direction = move === 'up' ? 1 : -1
    let index = move === 'up' ? 1 : next.length - 2
    while (index >= 0 && index < next.length) {
        const item = next[index]
        const adjacent = next[index - direction]
        if (
            item !== undefined &&
            adjacent !== undefined &&
            selected.has(item) &&
            !selected.has(adjacent)
        ) {
            next[index] = adjacent
            next[index - direction] = item
        }
        index += direction
    }
    return next
}

export function moveItem<T>(list: T[], index: number, direction: -1 | 1): void {
    const a = list[index]
    const b = list[index + direction]
    if (a === undefined || b === undefined) return
    list[index] = b
    list[index + direction] = a
}
