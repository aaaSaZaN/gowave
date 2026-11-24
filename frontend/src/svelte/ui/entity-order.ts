export function entityOrder<T extends { uuid: string }>(
    read: () => readonly T[],
    save: (items: T[]) => void
) {
    let source: string | undefined

    return (uuid: string) => ({
        start: (event: DragEvent) => {
            if (!event.dataTransfer) return
            source = uuid
            event.dataTransfer.effectAllowed = 'move'
            event.dataTransfer.setData('text/plain', uuid)
        },
        over: (event: DragEvent) => {
            if (source !== undefined) event.preventDefault()
        },
        drop: (event: DragEvent) => {
            if (!source) return
            event.preventDefault()
            const items = [...read()]
            const from = items.findIndex((item) => item.uuid === source)
            const target = items.findIndex((item) => item.uuid === uuid)
            source = undefined
            if (from === -1 || target === -1 || from === target) return
            const [item] = items.splice(from, 1)
            if (!item) return
            items.splice(target, 0, item)
            save(items)
        },
        end: () => {
            source = undefined
        }
    })
}
