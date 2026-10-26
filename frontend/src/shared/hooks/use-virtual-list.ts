import { type ItemKey, type VirtualLayoutOptions, virtualLayout } from '@shared/lib/virtual-layout'
import { useCallback, useState, useSyncExternalStore } from 'react'

function createViewportStore() {
    let snapshot = { offset: 0, height: 0 }
    return {
        getSnapshot: () => snapshot,
        subscribe(listener: () => void, target: HTMLElement | Window | null) {
            if (!target) return () => {}
            const element = target instanceof HTMLElement ? target : null
            const update = () => {
                const offset = element ? element.scrollTop : window.scrollY
                const height = element ? element.clientHeight : window.innerHeight
                if (offset === snapshot.offset && height === snapshot.height) return
                snapshot = { offset, height }
                listener()
            }
            target.addEventListener('scroll', update, { passive: true })
            window.addEventListener('resize', update)
            const observer = element ? new ResizeObserver(update) : null
            if (element) observer?.observe(element)
            update()
            return () => {
                target.removeEventListener('scroll', update)
                window.removeEventListener('resize', update)
                observer?.disconnect()
            }
        }
    }
}

function createMeasurementStore() {
    let snapshot: ReadonlyMap<ItemKey, number> = new Map()
    const listeners = new Set<() => void>()
    const elements = new Map<HTMLElement, ItemKey>()
    let observer: ResizeObserver | null = null
    const record = (entries: { key: ItemKey; height: number }[]) => {
        const changed = entries.filter(
            ({ key, height }) => height > 0 && snapshot.get(key) !== height
        )
        if (!changed.length) return
        const next = new Map(snapshot)
        for (const { key, height } of changed) next.set(key, height)
        snapshot = next
        for (const listener of listeners) listener()
    }
    const observe = (element: HTMLElement, key: ItemKey) => {
        if (!observer)
            observer = new ResizeObserver((entries) => {
                record(
                    entries.flatMap((entry) => {
                        const key = elements.get(entry.target as HTMLElement)
                        return key === undefined
                            ? []
                            : [{ key, height: entry.target.getBoundingClientRect().height }]
                    })
                )
            })
        for (const previous of elements.keys()) {
            if (!previous.isConnected) {
                observer.unobserve(previous)
                elements.delete(previous)
            }
        }
        elements.set(element, key)
        observer.observe(element)
        record([{ key, height: element.getBoundingClientRect().height }])
    }
    return {
        getSnapshot: () => snapshot,
        observe,
        subscribe(listener: () => void) {
            listeners.add(listener)
            for (const [element, key] of elements) observe(element, key)
            return () => {
                listeners.delete(listener)
                if (!listeners.size) {
                    observer?.disconnect()
                    observer = null
                }
            }
        }
    }
}

interface VirtualListOptions extends VirtualLayoutOptions {
    getScrollElement?: () => HTMLElement | null
}

export function useVirtualList(options: VirtualListOptions) {
    const { getScrollElement, getItemKey, scrollMargin = 0 } = options
    const [viewportStore] = useState(createViewportStore)
    const [measurementStore] = useState(createMeasurementStore)
    const subscribeViewport = useCallback(
        (listener: () => void) =>
            viewportStore.subscribe(listener, getScrollElement ? getScrollElement() : window),
        [viewportStore, getScrollElement]
    )
    const viewport = useSyncExternalStore(
        subscribeViewport,
        viewportStore.getSnapshot,
        viewportStore.getSnapshot
    )
    const measurements = useSyncExternalStore(
        measurementStore.subscribe,
        measurementStore.getSnapshot,
        measurementStore.getSnapshot
    )
    const measureElement = useCallback(
        (element: HTMLElement | null) => {
            if (!element) return
            const index = Number(element.dataset['index'])
            if (!Number.isInteger(index) || index < 0) return
            measurementStore.observe(element, getItemKey?.(index) ?? index)
        },
        [measurementStore, getItemKey]
    )

    // Rendering consumes immutable snapshots; only DOM subscriptions and ref
    // callbacks update the stores, so memoized children cannot retain stale methods.
    return { ...virtualLayout(options, viewport, measurements), scrollMargin, measureElement }
}
