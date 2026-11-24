import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { formatPreviewBody } from './format-preview-body'

describe('host preview display', () => {
    test('indents JSON without rounding large integers', () => {
        assert.equal(
            formatPreviewBody('{"id":9007199254740993}', 'application/vnd.api+json; charset=utf-8'),
            '{\n  "id": 9007199254740993\n}'
        )
    })

    test('preserves malformed JSON, plain text and empty bodies', () => {
        assert.equal(formatPreviewBody('{"a":', 'application/json'), '{"a":')
        assert.equal(formatPreviewBody('{"a":1}', 'text/plain'), '{"a":1}')
        assert.equal(formatPreviewBody('', ''), '')
    })
})
