import assert from 'node:assert/strict'
import { test } from 'node:test'
import dayjs from 'dayjs'
import { dateWithinRange, monthDays } from './calendar'

test('calendar displays complete Monday-first weeks across month and year boundaries', () => {
    const january = monthDays(dayjs('2025-01-01'))
    assert.equal(january[0]?.format('YYYY-MM-DD'), '2024-12-30')
    assert.equal(january.at(-1)?.format('YYYY-MM-DD'), '2025-02-02')
    assert.equal(january.length, 35)

    const leapYear = monthDays(dayjs('2024-02-01'))
    assert.ok(leapYear.some((day) => day.format('YYYY-MM-DD') === '2024-02-29'))
})

test('calendar allows dates on both limits and rejects dates outside them', () => {
    const min = '2025-05-02'
    const max = '2025-05-31'
    for (const day of [min, max]) assert.ok(dateWithinRange(dayjs(day), min, max))
    for (const day of ['2025-05-01', '2025-06-01']) {
        assert.equal(dateWithinRange(dayjs(day), min, max), false)
    }
})
