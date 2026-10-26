import dayjs from 'dayjs'

export function getNodeResetDaysUtil(targetDay: number): number {
    const today = dayjs()

    const targetThisMonth = today.date(targetDay)
    const targetNextMonth = today.add(1, 'month').date(targetDay)

    const correctedThisMonth = targetThisMonth.isValid() ? targetThisMonth : today.endOf('month')
    const correctedNextMonth = targetNextMonth.isValid()
        ? targetNextMonth
        : today.add(1, 'month').endOf('month')

    const daysToThisMonth = correctedThisMonth.diff(today, 'day')
    const daysToNextMonth = correctedNextMonth.diff(today, 'day')

    const targetDate =
        daysToThisMonth < 0
            ? correctedNextMonth
            : daysToThisMonth < daysToNextMonth
              ? correctedThisMonth
              : correctedNextMonth

    return targetDate.diff(today, 'day')
}

export function getNodeResetPeriodUtil(targetDay: number): string {
    const today = dayjs()

    const targetThisMonth = today.date(targetDay)
    const correctedThisMonth = targetThisMonth.isValid() ? targetThisMonth : today.endOf('month')

    const isPastTargetDay = today.date() > targetDay

    const startDate = isPastTargetDay ? correctedThisMonth : correctedThisMonth.subtract(1, 'month')

    const endDate = isPastTargetDay ? correctedThisMonth.add(1, 'month') : correctedThisMonth

    return `${startDate.format('D MMMM')} – ${endDate.format('D MMMM')}`
}

// Mirrors the backend: the current period began at the latest reset (01:00 UTC
// on the reset day, clamped to the end of short months) at or before now.
export function getNodeTrafficPeriodStartUtil(resetDay: number, now = new Date()): Date {
    const resetAt = (year: number, month: number) => {
        const lastDay = new Date(Date.UTC(year, month + 1, 0)).getUTCDate()
        const day = Math.min(Math.max(resetDay, 1), lastDay)
        return new Date(Date.UTC(year, month, day, 1))
    }

    const start = resetAt(now.getUTCFullYear(), now.getUTCMonth())
    return start > now ? resetAt(now.getUTCFullYear(), now.getUTCMonth() - 1) : start
}
