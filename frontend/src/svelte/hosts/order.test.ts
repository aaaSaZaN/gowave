import assert from 'node:assert/strict'
import { test } from 'node:test'

import { moveSelection } from '@shared/lib/move-selection'

test('moving selected hosts preserves selected and unselected relative order', () => {
    const order = ['a', 'b', 'c', 'd', 'e', 'f']
    const selected = new Set(['b', 'c', 'e'])
    assert.deepEqual(moveSelection(order, selected, 'up'), ['b', 'c', 'a', 'e', 'd', 'f'])
    assert.deepEqual(moveSelection(order, selected, 'down'), ['a', 'd', 'b', 'c', 'f', 'e'])
    assert.deepEqual(moveSelection(order, selected, 'top'), ['b', 'c', 'e', 'a', 'd', 'f'])
    assert.deepEqual(moveSelection(order, selected, 'bottom'), ['a', 'd', 'f', 'b', 'c', 'e'])
    assert.deepEqual(order, ['a', 'b', 'c', 'd', 'e', 'f'])
    assert.deepEqual(moveSelection([], selected, 'up'), [])
})
