import { Box, getThemeColor, useMantineTheme } from '@mantine/core'
import { sparklinePaths } from '@shared/lib/sparkline'
import { useId } from 'react'

interface SparklineProps {
    curveType?: 'bump' | 'linear'
    data: number[]
    fillOpacity?: number
    h: number
    strokeWidth?: number
    trendColors: { negative: string; neutral: string; positive: string }
    w: number | string
}

export function Sparkline({
    data,
    curveType,
    fillOpacity = 0.6,
    h,
    w,
    strokeWidth = 2,
    trendColors
}: SparklineProps) {
    const theme = useMantineTheme()
    const id = useId()
    const first = data[0] ?? 0
    const last = data.at(-1) ?? 0
    const trend = last > first ? 'positive' : last < first ? 'negative' : 'neutral'
    const color = getThemeColor(trendColors[trend], theme)
    const { line, area } = sparklinePaths(data, curveType === 'bump')
    return (
        <Box h={h} w={w}>
            <svg
                width="100%"
                height="100%"
                viewBox="0 0 1000 100"
                preserveAspectRatio="none"
                role="img"
                aria-label="Usage history"
            >
                <defs>
                    <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor={color} stopOpacity={fillOpacity} />
                        <stop offset="100%" stopColor={color} stopOpacity={0} />
                    </linearGradient>
                </defs>
                <path d={area} fill={`url(#${id})`} />
                <path
                    d={line}
                    fill="none"
                    stroke={color}
                    strokeWidth={strokeWidth}
                    vectorEffect="non-scaling-stroke"
                />
            </svg>
        </Box>
    )
}
