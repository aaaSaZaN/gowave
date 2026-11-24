type Notice = {
    title: string
    message: string
    color: 'red' | 'teal'
}

type VisibleNotice = Notice & { id: number }

const limit = 5
const autoClose = 4000
let nextId = 0
let paused = false
const timers = new Map<number, ReturnType<typeof setTimeout>>()
let pending = $state<VisibleNotice[]>([])
const visible = $derived(pending.slice(0, limit))

function cancelTimer(id: number): void {
    clearTimeout(timers.get(id))
    timers.delete(id)
}

function scheduleTimers(): void {
    if (paused) return
    for (const { id } of visible) {
        if (!timers.has(id))
            timers.set(
                id,
                setTimeout(() => notifications.hide(id), autoClose)
            )
    }
}

export const notifications = {
    get visible() {
        return visible
    },
    show(notice: Notice): void {
        pending = [...pending, { ...notice, id: ++nextId }]
        scheduleTimers()
    },
    hide(id: number): void {
        cancelTimer(id)
        pending = pending.filter((item) => item.id !== id)
        scheduleTimers()
    },
    pause(value: boolean): void {
        if (value === paused) return
        paused = value
        if (paused) {
            for (const id of timers.keys()) cancelTimer(id)
        } else scheduleTimers()
    }
}
