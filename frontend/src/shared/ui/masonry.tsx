import { Children, type ReactNode, useLayoutEffect, useRef } from 'react'

interface MasonryProps {
    children: ReactNode
    columns: number | Record<number, number>
    gap?: number
}

export default function Masonry({ children, columns, gap = 0 }: MasonryProps) {
    const root = useRef<HTMLDivElement>(null)

    useLayoutEffect(() => {
        const container = root.current
        if (!container) return
        const setColumns = () => {
            const breakpoints = typeof columns === 'number' ? [] : Object.entries(columns)
            const count =
                typeof columns === 'number'
                    ? columns
                    : breakpoints
                          .sort(([a], [b]) => Number(a) - Number(b))
                          .reduce(
                              (count, [width, value]) =>
                                  window.innerWidth >= Number(width) ? value : count,
                              1
                          )
            container.style.gridTemplateColumns = `repeat(${Math.max(1, count)}, minmax(0, 1fr))`
        }
        setColumns()
        window.addEventListener('resize', setColumns)
        return () => window.removeEventListener('resize', setColumns)
    }, [columns])

    useLayoutEffect(() => {
        const container = root.current
        if (!container) return
        const measure = (content: Element) => {
            const item = (content as HTMLElement).parentElement
            if (item)
                item.style.gridRowEnd = `span ${Math.max(1, Math.ceil(content.getBoundingClientRect().height))}`
        }
        const observer = new ResizeObserver((entries) =>
            entries.forEach((entry) => {
                measure(entry.target)
            })
        )
        const observed = new Set<Element>()
        const observeChildren = () => {
            for (const content of observed) {
                if (!container.contains(content)) {
                    observer.unobserve(content)
                    observed.delete(content)
                }
            }
            for (const item of container.children) {
                if (item.firstElementChild) {
                    measure(item.firstElementChild)
                    observer.observe(item.firstElementChild)
                    observed.add(item.firstElementChild)
                }
            }
        }
        observeChildren()
        const mutations = new MutationObserver(observeChildren)
        mutations.observe(container, { childList: true })
        return () => {
            observer.disconnect()
            mutations.disconnect()
        }
    }, [])

    return (
        <div ref={root} style={{ display: 'grid', gridAutoRows: '1px', columnGap: gap }}>
            {Children.toArray(children).map((child, index) => (
                <div
                    key={
                        typeof child === 'object' && child !== null && 'key' in child
                            ? child.key
                            : index
                    }
                    style={{ minWidth: 0 }}
                >
                    <div style={{ paddingBottom: gap, display: 'flow-root' }}>{child}</div>
                </div>
            ))}
        </div>
    )
}
