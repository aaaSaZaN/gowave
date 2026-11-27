import { writable } from 'svelte/store'

export function applyEntityOrder<T extends { uuid: string }>(
    items: readonly T[],
    order?: readonly string[]
): readonly T[] {
    if (!order) return items
    const positions = new Map(order.map((uuid, index) => [uuid, index]))
    return [...items].sort(
        (a, b) =>
            (positions.get(a.uuid) ?? positions.size) - (positions.get(b.uuid) ?? positions.size)
    )
}

export function entityOrder<T extends { uuid: string }>(
    read: () => readonly T[],
    save: (items: T[]) => Promise<unknown>
) {
    const pendingOrder = writable<readonly string[] | undefined>()
    let saving = false
    const clear = () => {
        pendingOrder.set(undefined)
        saving = false
    }
    const drag = (source: string) => ({
        key: source,
        drop(target: string) {
            if (saving) return
            const items = [...read()]
            const from = items.findIndex((item) => item.uuid === source)
            const to = items.findIndex((item) => item.uuid === target)
            const item = items[from]
            if (!item || to === -1 || from === to) return
            items.splice(from, 1)
            items.splice(to, 0, item)
            saving = true
            pendingOrder.set(items.map(({ uuid }) => uuid))
            void save(items).then(clear, clear)
        }
    })
    return { drag, pendingOrder }
}

type ReorderAxis = 'x' | 'y' | 'grid'

export interface ReorderHandleOptions {
    axis?: ReorderAxis
    drop: (target: string) => void
    onactive?: (active: boolean) => void
}

const itemSelector = '[data-reorder-key]'
const grabbedGroups = new WeakSet<HTMLElement>()

function restingBox(item: HTMLElement): DOMRect {
    const box = item.getBoundingClientRect()
    if (!item.hasAttribute('data-reorder-moving')) return box
    // Transitions can leave the displayed item between its old and new positions.
    const [x, y] = getComputedStyle(item).translate.split(' ').map(Number.parseFloat)
    return new DOMRect(box.x - (x || 0), box.y - (y || 0), box.width, box.height)
}

function scrollContainers(group: HTMLElement): HTMLElement[] {
    const containers: HTMLElement[] = []
    for (let element: HTMLElement | null = group; element; element = element.parentElement) {
        const style = getComputedStyle(element)
        if (element instanceof HTMLDialogElement) {
            containers.push(element)
            return containers
        }
        if (/auto|scroll/.test(style.overflowX + style.overflowY)) containers.push(element)
    }
    const root = document.scrollingElement
    if (root instanceof HTMLElement && !containers.includes(root)) containers.push(root)
    return containers
}

function scrollBounds(element: HTMLElement): DOMRect {
    if (element === document.scrollingElement)
        return new DOMRect(0, 0, window.innerWidth, window.innerHeight)
    const box = element.getBoundingClientRect()
    return new DOMRect(
        box.x + element.clientLeft,
        box.y + element.clientTop,
        element.clientWidth,
        element.clientHeight
    )
}

export function withinReorderDropZone(
    group: HTMLElement,
    { clientX: x, clientY: y }: Pick<PointerEvent, 'clientX' | 'clientY'>,
    axis: ReorderAxis = 'y'
): boolean {
    if (x < 0 || x > window.innerWidth || y < 0 || y > window.innerHeight) return false

    // The pointer can pass the first or last item while staying in the list's lane.
    const list = group.getBoundingClientRect()
    if (axis === 'x' ? y < list.top || y > list.bottom : x < list.left || x > list.right)
        return false

    return scrollContainers(group).every((container) => {
        const bounds = scrollBounds(container)
        if (container instanceof HTMLDialogElement)
            return x >= bounds.left && x <= bounds.right && y >= bounds.top && y <= bounds.bottom
        const clipsX = container.scrollWidth > container.clientWidth
        const clipsY = container.scrollHeight > container.clientHeight
        return (
            (!clipsX || (x >= bounds.left && x <= bounds.right)) &&
            (!clipsY || (y >= bounds.top && y <= bounds.bottom))
        )
    })
}

function distanceToRange(point: number, start: number, end: number): number {
    return Math.max(start - point, point - end, 0)
}

function edgeSpeed(point: number, start: number, end: number): number {
    const edge = Math.min(48, (end - start) / 4)
    const distance = point < start + edge ? point - start - edge : Math.max(0, point - end + edge)
    return Math.max(-18, Math.min(18, distance / 4))
}

export function reorderHandle(handle: HTMLElement, initial?: ReorderHandleOptions) {
    let options: ReorderHandleOptions | undefined
    let stop: (() => void) | undefined
    let suppressClick = false
    let listening = false

    function update(next?: ReorderHandleOptions): void {
        if (next && !listening) {
            handle.addEventListener('pointerdown', start)
            handle.addEventListener('click', click, true)
            listening = true
        }
        if (!next) stop?.()
        options = next
        handle.toggleAttribute('data-reorder-handle', Boolean(options))
    }

    function start(event: PointerEvent): void {
        if (stop || !event.isPrimary || event.button !== 0) return
        suppressClick = false
        if (!options) return
        const sourceItem = handle.closest<HTMLElement>(itemSelector)
        const sourceGroup = sourceItem?.parentElement
        if (!sourceItem || !sourceGroup || grabbedGroups.has(sourceGroup)) return
        const item = sourceItem
        const group = sourceGroup

        const startX = event.clientX
        const startY = event.clientY
        const startBox = item.getBoundingClientRect()
        const anchorX = startX - startBox.left
        const anchorY = startY - startBox.top
        const pointerId = event.pointerId
        const axis = options.axis ?? 'y'
        const containers = scrollContainers(group)
        const listeners = new AbortController()
        const moving = new Set<HTMLElement>()
        const table = axis === 'x' ? item.closest('table') : null
        let frame = 0
        let x = startX
        let y = startY
        let active = false
        let target = item
        const onactive = options.onactive

        function clear(element: HTMLElement): void {
            element.removeAttribute('data-reorder-active')
            element.removeAttribute('data-reorder-moving')
            element.style.removeProperty('--reorder-x')
            element.style.removeProperty('--reorder-y')
        }

        function translate(element: HTMLElement, dx: number, dy: number): void {
            moving.add(element)
            element.toggleAttribute('data-reorder-moving', true)
            element.style.setProperty('--reorder-x', `${dx}px`)
            element.style.setProperty('--reorder-y', `${dy}px`)
        }

        function moveColumn(header: HTMLElement, dx: number): void {
            translate(header, dx, 0)
            if (!(header instanceof HTMLTableCellElement) || !table) return
            for (const body of table.tBodies) {
                for (const row of body.rows) {
                    const cell = row.cells[header.cellIndex]
                    if (cell?.colSpan !== 1) continue
                    if (header === item) cell.toggleAttribute('data-reorder-active', true)
                    translate(cell, dx, 0)
                }
            }
        }

        function layout(): void {
            const entries = Array.from(group.children).flatMap((element) =>
                element instanceof HTMLElement && element.matches(itemSelector)
                    ? [{ element, box: restingBox(element) }]
                    : []
            )
            const from = entries.findIndex((entry) => entry.element === item)
            const source = entries[from]
            if (!source) return
            const { box } = source
            const groupBox = group.getBoundingClientRect()
            const left = Math.max(groupBox.left, Math.min(groupBox.right - box.width, x - anchorX))
            const top = Math.max(groupBox.top, Math.min(groupBox.bottom - box.height, y - anchorY))
            let best = Infinity
            let to = from
            for (const [index, entry] of entries.entries()) {
                const dx = axis === 'y' ? 0 : distanceToRange(x, entry.box.left, entry.box.right)
                const dy = axis === 'x' ? 0 : distanceToRange(y, entry.box.top, entry.box.bottom)
                const distance = Math.hypot(dx, dy)
                if (distance < best) {
                    best = distance
                    to = index
                }
            }
            target = entries[to]?.element ?? item
            const direction = Math.sign(to - from)
            const low = Math.min(from, to)
            const high = Math.max(from, to)
            const gap =
                Number.parseFloat(
                    axis === 'x'
                        ? getComputedStyle(group).columnGap
                        : getComputedStyle(group).rowGap
                ) || 0

            for (const [index, entry] of entries.entries()) {
                if (index === from) continue
                const shift = index >= low && index <= high
                const slot = shift ? entries[index - direction]?.box : undefined
                if (slot && axis === 'grid')
                    translate(entry.element, slot.left - entry.box.left, slot.top - entry.box.top)
                else if (axis === 'x' && (shift || moving.has(entry.element)))
                    moveColumn(
                        entry.element,
                        shift
                            ? -Math.sign((entries[to]?.box.left ?? box.left) - box.left) *
                                  (box.width + gap)
                            : 0
                    )
                else if (shift) translate(entry.element, 0, -direction * (box.height + gap))
                else if (moving.has(entry.element)) translate(entry.element, 0, 0)
            }
            if (axis === 'x') moveColumn(item, left - box.left)
            else translate(item, axis === 'y' ? 0 : left - box.left, top - box.top)
        }

        function tick(): void {
            frame = 0
            for (const container of containers) {
                const bounds = scrollBounds(container)
                const dx = axis === 'y' ? 0 : edgeSpeed(x, bounds.left, bounds.right)
                const dy = axis === 'x' ? 0 : edgeSpeed(y, bounds.top, bounds.bottom)
                const previousX = container.scrollLeft
                const previousY = container.scrollTop
                if (dx || dy) container.scrollBy({ left: dx, top: dy, behavior: 'instant' })
                if (container.scrollLeft !== previousX || container.scrollTop !== previousY) {
                    frame = requestAnimationFrame(tick)
                    break
                }
            }
            layout()
        }

        function finish(): void {
            stop = undefined
            grabbedGroups.delete(group)
            listeners.abort()
            if (frame) cancelAnimationFrame(frame)
            item.removeAttribute('data-reorder-active')
            for (const element of moving) clear(element)
            if (handle.hasPointerCapture(pointerId)) handle.releasePointerCapture(pointerId)
            if (active) onactive?.(false)
        }

        function move(moveEvent: PointerEvent): void {
            if (moveEvent.pointerId !== pointerId) return
            x = moveEvent.clientX
            y = moveEvent.clientY
            if (!active) {
                const dx = axis === 'y' ? 0 : x - startX
                const dy = axis === 'x' ? 0 : y - startY
                if (Math.hypot(dx, dy) < 5) return
                active = true
                suppressClick = true
                item.toggleAttribute('data-reorder-active', true)
                onactive?.(true)
            }
            if (!frame) frame = requestAnimationFrame(tick)
        }

        function drop(up: PointerEvent): void {
            if (up.pointerId !== pointerId) return
            x = up.clientX
            y = up.clientY
            if (active) layout()
            const valid = active && target !== item && withinReorderDropZone(group, up, axis)
            const key = target.getAttribute('data-reorder-key')
            finish()
            if (valid && key !== null) options?.drop(key)
        }

        handle.setPointerCapture(pointerId)
        grabbedGroups.add(group)
        stop = finish
        const { signal } = listeners
        // Keep listening outside the handle so releasing outside the list cancels the move.
        window.addEventListener('pointermove', move, { signal })
        window.addEventListener('pointerup', drop, { signal })
        window.addEventListener(
            'pointercancel',
            (cancel) => {
                if (cancel.pointerId === pointerId) finish()
            },
            { signal }
        )
        handle.addEventListener('lostpointercapture', finish, { signal })
        window.addEventListener('blur', finish, { signal })
        window.addEventListener(
            'keydown',
            (keyEvent) => {
                if (keyEvent.key !== 'Escape' || !active) return
                keyEvent.preventDefault()
                keyEvent.stopPropagation()
                finish()
            },
            { capture: true, signal }
        )
    }

    function click(event: MouseEvent): void {
        if (!suppressClick || event.detail === 0) return
        suppressClick = false
        event.preventDefault()
        event.stopImmediatePropagation()
    }

    update(initial)
    return {
        update,
        destroy() {
            update()
            handle.removeEventListener('pointerdown', start)
            handle.removeEventListener('click', click, true)
        }
    }
}
