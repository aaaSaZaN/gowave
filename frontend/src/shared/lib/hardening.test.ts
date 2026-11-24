import assert from 'node:assert/strict'
import { test } from 'node:test'
import { parseJsonField } from '../utils/misc/parse-json-field'
import { parseFormErrors } from './form-errors'
import { requireValue } from './invariant'
import { swapItems } from './swap-items'

void test('reordering preserves input and ignores missing array positions', () => {
    const first = { uuid: 'first' }
    const second = { uuid: 'second' }
    const items = [first, second]
    assert.deepEqual(swapItems(items, 0, 1), [second, first])
    assert.deepEqual(items, [first, second])
    for (const index of [-1, 2, Number.NaN, 0.5]) {
        assert.deepEqual(swapItems(items, 0, index), items)
    }
    assert.deepEqual(swapItems([], 0, 1), [])
})

void test('required values preserve falsy data and reject missing values', () => {
    assert.equal(requireValue(0, 'missing'), 0)
    assert.equal(requireValue(false, 'missing'), false)
    assert.equal(requireValue('', 'missing'), '')
    assert.throws(() => requireValue(null, 'missing'), /missing/)
    assert.throws(() => requireValue(undefined, 'missing'), /missing/)
})

void test('JSON fields accept objects, arrays and text and reject unexpected scalar values', () => {
    assert.deepEqual(parseJsonField('{"name":"value"}'), { name: 'value' })
    assert.deepEqual(parseJsonField('[1,2]'), [1, 2])
    assert.equal(parseJsonField('"text"'), 'text')
    for (const value of ['', 'null', 'true', '1', undefined, {}, 42]) {
        assert.equal(parseJsonField(value), null)
    }
    assert.equal(parseJsonField('{invalid'), null)
})

void test('form errors validate unknown payloads and preserve nested field paths', () => {
    assert.deepEqual(parseFormErrors(null), { fieldErrors: {}, formErrors: [] })
    assert.deepEqual(
        parseFormErrors({
            issues: [
                { path: ['users', 0, 'name'], message: 'Required' },
                { path: [], message: 'Invalid request' },
                { path: ['ignored'], message: { unsafe: true } }
            ]
        }),
        { fieldErrors: { 'users.0.name': 'Required' }, formErrors: ['Invalid request'] }
    )
    assert.deepEqual(
        parseFormErrors({
            fieldErrors: { name: ['First', 'Second'], invalid: null },
            formErrors: ['Failed', 7]
        }),
        { fieldErrors: { name: 'First,Second' }, formErrors: ['Failed'] }
    )
})
