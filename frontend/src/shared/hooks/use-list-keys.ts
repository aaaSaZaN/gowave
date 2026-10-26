import { useState } from 'react'

interface ListIdentity<T> {
    items: readonly T[]
    keys: number[]
    nextKey: number
}

// Contract arrays have no persistent IDs. Preserve identity through moves and
// immutable edits without adding UI metadata to the serialized configuration.
export function reconcileListKeys<T>(
    previous: ListIdentity<T>,
    items: readonly T[]
): ListIdentity<T> {
    const used = new Set<number>()
    const keys = items.map((item) => {
        const index = previous.items.findIndex(
            (old, position) => old === item && !used.has(position)
        )
        if (index < 0) return undefined
        used.add(index)
        return previous.keys[index]
    })
    let nextKey = previous.nextKey
    const nextKeys = keys.map((key, index) => {
        if (key !== undefined) return key
        const existing = previous.keys[index]
        if (!used.has(index) && existing !== undefined) {
            used.add(index)
            return existing
        }
        return nextKey++
    })
    return { items, keys: nextKeys, nextKey }
}

export function useListKeys<T>(items: readonly T[]): number[] {
    const [identity, setIdentity] = useState<ListIdentity<T>>(() => ({
        items,
        keys: items.map((_, index) => index),
        nextKey: items.length
    }))
    if (identity.items !== items) {
        const next = reconcileListKeys(identity, items)
        setIdentity(next)
        return next.keys
    }
    return identity.keys
}
