import { useSyncExternalStore } from 'react'

import classes from './navigation-progress.module.css'

const listeners = new Set<() => void>()
let loading = false

function notify(value: boolean) {
    loading = value
    for (const listener of listeners) listener()
}

export const nprogress = {
    start: () => notify(true),
    complete: () => notify(false)
}

export function NavigationProgress() {
    const active = useSyncExternalStore(
        subscribe,
        () => loading,
        () => false
    )
    return active ? <div className={classes.bar} role="progressbar" aria-label="Loading" /> : null
}

function subscribe(listener: () => void) {
    listeners.add(listener)
    return () => {
        listeners.delete(listener)
    }
}
