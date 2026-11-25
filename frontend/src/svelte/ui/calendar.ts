import type { Dayjs } from 'dayjs'

export function monthDays(month: Dayjs): Dayjs[] {
    const first = month.startOf('month')
    const weekday = (first.day() + 6) % 7
    const start = first.subtract(weekday, 'day')
    const length = Math.ceil((weekday + first.daysInMonth()) / 7) * 7
    return Array.from({ length }, (_, index) => start.add(index, 'day'))
}

export function dateWithinRange(value: Dayjs, min?: string, max?: string): boolean {
    const iso = value.format('YYYY-MM-DD')
    return (!min || iso >= min) && (!max || iso <= max)
}
