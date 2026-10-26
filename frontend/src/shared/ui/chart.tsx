import Highcharts from 'highcharts'
import { useEffect, useRef } from 'react'

interface ChartProps {
    options: Highcharts.Options
    title?: string
}

export function Chart({ options, title }: ChartProps) {
    const container = useRef<HTMLDivElement>(null)
    const chart = useRef<Highcharts.Chart | null>(null)

    useEffect(() => {
        if (!container.current) return
        chart.current = Highcharts.chart(container.current, {})
        return () => {
            chart.current?.destroy()
            chart.current = null
        }
    }, [])

    useEffect(() => {
        chart.current?.update(
            { ...options, title: title === undefined ? options.title : { text: title } },
            true,
            true
        )
    }, [options, title])

    return <div ref={container} />
}
