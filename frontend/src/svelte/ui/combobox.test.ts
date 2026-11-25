import assert from 'node:assert/strict'
import { test } from 'node:test'
import { nextComboboxOption } from './combobox'

const sizes = ['20', '25', '30', '50'].map((value) => ({ value }))

test('keyboard navigation starts from the current choice and wraps at the ends', () => {
    const next = nextComboboxOption('ArrowDown', '25', sizes)
    assert.equal(next, '30')
    assert.equal(nextComboboxOption('ArrowDown', next, sizes), '50')
    assert.equal(nextComboboxOption('ArrowDown', '50', sizes), '20')
    assert.equal(nextComboboxOption('ArrowUp', '20', sizes), '50')
})

test('filtered lists can have no current option or no options', () => {
    assert.equal(nextComboboxOption('ArrowDown', '100', sizes), '20')
    assert.equal(nextComboboxOption('ArrowUp', undefined, sizes), '50')
    assert.equal(nextComboboxOption('ArrowDown', '25', []), undefined)
})
