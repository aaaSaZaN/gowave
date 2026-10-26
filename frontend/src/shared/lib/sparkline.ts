export function sparklinePaths(data: number[], curved: boolean) {
    if (!data.length) return { line: '', area: '' }
    const values = data.map((value) => (Number.isFinite(value) ? value : 0))
    const minimum = Math.min(0, ...values)
    const maximum = Math.max(0, ...values)
    const range = maximum - minimum || 1
    const points = values.map((value, index) => ({
        x: values.length === 1 ? 500 : (index * 1000) / (values.length - 1),
        y: 98 - ((value - minimum) * 96) / range
    }))
    const first = points[0]
    if (!first) return { line: '', area: '' }
    let last = first
    let line = `M${first.x},${first.y}`
    for (const point of points.slice(1)) {
        const previous = last
        const middle = (previous.x + point.x) / 2
        line += curved
            ? ` C${middle},${previous.y} ${middle},${point.y} ${point.x},${point.y}`
            : ` L${point.x},${point.y}`
        last = point
    }
    return { line, area: `${line} L${last.x},100 L${first.x},100 Z` }
}
