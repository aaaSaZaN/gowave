import assert from 'node:assert/strict'
import { test } from 'node:test'

import { readRemarks } from './settings-fields'

test('custom remarks keep each status row in form order', () => {
    const form = new FormData()
    form.append('expiredUsers', ' first ')
    form.append('expiredUsers', 'second')
    form.append('expiredUsers', '  ')
    form.append('disabledUsers', ' disabled ')

    const remarks = readRemarks(form)
    assert.deepEqual(remarks.expiredUsers, ['first', 'second'])
    assert.deepEqual(remarks.disabledUsers, ['disabled'])
    assert.deepEqual(remarks.limitedUsers, [])
})
