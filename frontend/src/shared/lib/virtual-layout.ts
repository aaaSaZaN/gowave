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
}

export function virtualLayout(
    options: VirtualLayoutOptions,
    viewport: { offset: number; height: number },
    measurements: ReadonlyMap<ItemKey, number>
) {
    const { count, estimateSize, getItemKey, overscan = 1, scrollMargin = 0 } = options
    const items: VirtualItem[] = []
    let start = scrollMargin
    for (let index = 0; index < count; index++) {
        const key = getItemKey?.(index) ?? index
        const size = measurements.get(key) ?? estimateSize(index)
        items.push({ index, key, start, size, end: start + size })
        start += size
    }
    const totalSize = start - scrollMargin
    if (!items.length || viewport.height <= 0) return { totalSize, virtualItems: [] }

    let lower = 0
    let upper = items.length - 1
    while (lower < upper) {
        const middle = Math.floor((lower + upper) / 2)
        if (items[middle].end <= viewport.offset) lower = middle + 1
        else upper = middle
    }
    const first = Math.max(0, lower - overscan)
    let last = lower
    while (last + 1 < items.length && items[last + 1].start < viewport.offset + viewport.height)
        last++
    last = Math.min(items.length - 1, last + overscan)
    return { totalSize, virtualItems: items.slice(first, last + 1) }
}
