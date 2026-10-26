import assert from 'node:assert/strict'
import { test } from 'node:test'
import { reconcileListKeys } from '../hooks/use-list-keys'
import { parseJsonField } from '../utils/misc/parse-json-field'
import { parseFormErrors } from './form-errors'
import { requireValue } from './invariant'
import { optionalProp } from './optional-prop'
import { swapItems } from './swap-items'

void test('optional React props omit undefined and retain meaningful falsy values', () => {
    assert.deepEqual(optionalProp('value', undefined), {})
    assert.deepEqual(optionalProp('value', null), { value: null })
    assert.deepEqual(optionalProp('value', false), { value: false })
    assert.deepEqual(optionalProp('value', 0), { value: 0 })
    assert.deepEqual(optionalProp('value', ''), { value: '' })
})

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

void test('editor row identity follows moves, immutable edits, insertions and deletions', () => {
    const a = { name: 'a' }
    const b = { name: 'b' }
    const initial = { items: [a, b], keys: [0, 1], nextKey: 2 }
    const moved = reconcileListKeys(initial, [b, a])
    assert.deepEqual(moved.keys, [1, 0])
    const edited = reconcileListKeys(moved, [b, { name: 'edited' }])
    assert.deepEqual(edited.keys, [1, 0])
    const inserted = reconcileListKeys(edited, [{ name: 'new' }, ...edited.items])
    assert.deepEqual(inserted.keys, [2, 1, 0])
    const deleted = reconcileListKeys(inserted, inserted.items.slice(1))
    assert.deepEqual(deleted.keys, [1, 0])
    const duplicate = reconcileListKeys(initial, [a, a])
    assert.equal(new Set(duplicate.keys).size, 2)
    assert.deepEqual(reconcileListKeys(initial, []).keys, [])
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
