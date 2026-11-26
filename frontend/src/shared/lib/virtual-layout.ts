export type ItemKey = string | number

export interface VirtualItem {
    index: number
    key: ItemKey
    start: number
    end: number
    size: number
}

export interface VirtualLayoutOptions {
    count: number
    estimateSize: (index: number) => number
    getItemKey?: (index: number) => ItemKey
    overscan?: number
    scrollMargin?: number
    pinnedKey?: ItemKey
}

export function fixedVirtualWindow(
    count: number,
    rowHeight: number,
    offset: number,
    height: number,
    overscan = 4
) {
    if (count <= 0 || rowHeight <= 0 || height <= 0) return { from: 0, to: 0 }
    const first = Math.min(count - 1, Math.floor(Math.max(0, offset) / rowHeight))
    const end = Math.max(first + 1, Math.ceil((Math.max(0, offset) + height) / rowHeight))
    return {
        from: Math.max(0, first - overscan),
        to: Math.min(count, end + overscan)
    }
}

export function virtualLayout(
    options: VirtualLayoutOptions,
    viewport: { offset: number; height: number },
    measurements: ReadonlyMap<ItemKey, number>
) {
    const { count, estimateSize, getItemKey, overscan = 1, scrollMargin = 0, pinnedKey } = options
    const items: VirtualItem[] = []
    let pinned: VirtualItem | undefined
    let start = scrollMargin
    for (let index = 0; index < count; index++) {
        const key = getItemKey?.(index) ?? index
        const size = measurements.get(key) ?? estimateSize(index)
        const item = { index, key, start, size, end: start + size }
        items.push(item)
        if (key === pinnedKey) pinned = item
        start += size
    }
    const totalSize = start - scrollMargin
    if (!items.length || viewport.height <= 0) return { totalSize, virtualItems: [] }

    let lower = 0
    let upper = items.length - 1
    while (lower < upper) {
        const middle = Math.floor((lower + upper) / 2)
        const item = items[middle]
        if (item && item.end <= viewport.offset) lower = middle + 1
        else upper = middle
    }
    const first = Math.max(0, lower - overscan)
    let last = lower
    while (
        last + 1 < items.length &&
        (items[last + 1]?.start ?? Infinity) < viewport.offset + viewport.height
    )
        last++
    last = Math.min(items.length - 1, last + overscan)
    const virtualItems = items.slice(first, last + 1)
    if (pinned && pinned.index < first) virtualItems.unshift(pinned)
    if (pinned && pinned.index > last) virtualItems.push(pinned)
    return { totalSize, virtualItems }
}
