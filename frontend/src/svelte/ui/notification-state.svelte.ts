type Notice = {
    title: string
    message: string
    color: 'red' | 'teal'
}

type VisibleNotice = Notice & { id: number }

let nextId = 0
const timers = new Map<number, ReturnType<typeof setTimeout>>()
let visible = $state<VisibleNotice[]>([])

export const notifications = {
    get visible() {
        return visible
    },
    show(notice: Notice): void {
        const id = ++nextId
        visible = [...visible.slice(-4), { ...notice, id }]
        timers.set(
            id,
            setTimeout(() => notifications.hide(id), 4000)
        )
    },
    hide(id: number): void {
        clearTimeout(timers.get(id))
        timers.delete(id)
        visible = visible.filter((item) => item.id !== id)
    }
}
