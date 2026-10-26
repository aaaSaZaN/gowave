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
    let line = `M${points[0].x},${points[0].y}`
    for (let index = 1; index < points.length; index++) {
        const point = points[index]
        const previous = points[index - 1]
        const middle = (previous.x + point.x) / 2
        line += curved
            ? ` C${middle},${previous.y} ${middle},${point.y} ${point.x},${point.y}`
            : ` L${point.x},${point.y}`
    }
    return { line, area: `${line} L${points.at(-1)!.x},100 L${points[0].x},100 Z` }
}
