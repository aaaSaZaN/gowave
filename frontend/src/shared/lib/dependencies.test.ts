import assert from 'node:assert/strict'
import { test } from 'node:test'

import { createQueryKeys, mergeQueryKeys } from '../api/query-keys'
import { sortRecords } from '../ui/datatable/sort'
import {
    prettifyBytesUtil,
    prettySiBytesUtil,
    prettySiRealtimeBytesUtil
} from '../utils/bytes/pretty-bytes/pretty-bytes.util'
import { decodeBase64, encodeBase64 } from '../utils/misc/base64'
import { faviconResolver } from '../utils/misc/favicon-resolver'
import { decode, decodeURLSafe, encode, encodeURLSafe } from './base64'
import clsx from './classes'
import ColorHash, { colorFromId } from './color'
import invariant from './invariant'
import get from './object-path'
import { sparklinePaths } from './sparkline'
import semver from './version'

test('class names preserve conditional, nested, numeric and object values', () => {
    assert.equal(
        clsx('card', null, undefined, false, 0, true, 7, [
            'active',
            ['', { hidden: false, open: true }]
        ]),
        'card 7 active open'
    )
    assert.equal(clsx(), '')
})

test('base64 matches RFC vectors, binary data and large UTF-8 templates', () => {
    for (const [text, encoded] of [
        ['', ''],
        ['f', 'Zg=='],
        ['fo', 'Zm8='],
        ['foo', 'Zm9v'],
        ['foobar', 'Zm9vYmFy']
    ]) {
        assert.equal(encodeBase64(text), encoded)
        assert.equal(decodeBase64(encoded), text)
    }
    const text = 'Привет, 世界 👋'.repeat(20000)
    assert.equal(decodeBase64(encodeBase64(text)), text)
    const binary = Uint8Array.from({ length: 256 }, (_, index) => index)
    assert.deepEqual(decode(encode(binary)), binary)
    assert.deepEqual(decodeURLSafe(encodeURLSafe(binary)), binary)
    assert.equal(decodeBase64('  Zg==\n'), 'f')
    assert.equal(decodeBase64('_w=='), null) // Binary 0xff is invalid UTF-8.
    assert.equal(decodeBase64(encodeURLSafe(new TextEncoder().encode('🌍'))), '🌍')
})

test('base64 rejects malformed vault data and mixed alphabets', () => {
    for (const value of ['Zg', 'Zg=', 'Zg===', '=Zg=', 'Z g==', 'Zg==\n', '!!!!', 'a===']) {
        assert.throws(() => decode(value), /base64|character/i)
    }
    assert.throws(() => decodeURLSafe('+/8='), /base64url/)
    assert.equal(decodeBase64('not base64'), null)
})

test('query keys retain parameter values and invalidation prefixes', () => {
    const users = createQueryKeys('users', {
        all: { queryKey: null },
        detail: (id: string, filter?: { enabled: boolean }) => ({ queryKey: [id, filter] })
    })
    const nodes = createQueryKeys('nodes', { all: { queryKey: null } })
    assert.deepEqual(users._def, ['users'])
    assert.deepEqual(users.all.queryKey, ['users', 'all'])
    assert.deepEqual(users.all._def, ['users', 'all'])
    assert.deepEqual(users.detail._def, ['users', 'detail'])
    assert.deepEqual(users.detail('a', { enabled: false }).queryKey, [
        'users',
        'detail',
        'a',
        { enabled: false }
    ])
    assert.deepEqual(users.detail('b').queryKey, ['users', 'detail', 'b', undefined])
    const keys = mergeQueryKeys(users, nodes)
    assert.equal(keys.users, users)
    assert.equal(keys.nodes, nodes)
})

test('table paths support literal dotted keys, arrays and missing properties', () => {
    const object = { user: { name: 'A' }, 'user.name': 'literal', nodes: [{ id: 3 }] }
    assert.equal(get(object, 'user.name'), 'literal')
    assert.equal(get(object, 'nodes.0.id'), 3)
    assert.equal(get(object, 'user.missing'), undefined)
    assert.equal(get(object, '__proto__.constructor'), undefined)
    assert.equal(get(null, 'a'), undefined)
    const key = Symbol('key')
    assert.equal(get({ [key]: 12 }, key), 12)
})

test('table sorting is stable, case insensitive and keeps missing values last', () => {
    const records = [
        { id: 1, value: 'b' },
        { id: 2, value: null },
        { id: 3, value: 'A' },
        { id: 4, value: 'a' }
    ]
    assert.deepEqual(
        sortRecords(records, { columnAccessor: 'value', direction: 'asc' }).map((row) => row.id),
        [3, 4, 1, 2]
    )
    assert.deepEqual(
        sortRecords(records, { columnAccessor: 'value', direction: 'desc' }).map((row) => row.id),
        [1, 3, 4, 2]
    )
    assert.deepEqual(
        records.map((row) => row.id),
        [1, 2, 3, 4]
    )
    assert.deepEqual(
        sortRecords([{ n: 20 }, { n: 3 }, { n: 100 }], { columnAccessor: 'n', direction: 'asc' }),
        [{ n: 3 }, { n: 20 }, { n: 100 }]
    )
})

test('version checks follow SemVer prerelease precedence and ignore build metadata', () => {
    const ordered = [
        '1.0.0-alpha',
        '1.0.0-alpha.1',
        '1.0.0-alpha.beta',
        '1.0.0-beta',
        '1.0.0-beta.2',
        '1.0.0-beta.11',
        '1.0.0-rc.1',
        '1.0.0'
    ]
    for (let index = 1; index < ordered.length; index++)
        assert.ok(semver.gt(ordered[index], ordered[index - 1]))
    assert.equal(semver.gt('1.2.3+build2', '1.2.3+build1'), false)
    assert.equal(semver.clean(' =v3.4.15 '), '3.4.15')
    assert.equal(semver.valid('1.02.3'), null)
    assert.equal(semver.valid('1.0.0-01'), null)
    assert.equal(semver.valid('9007199254740992.0.0'), null)
    assert.equal(semver.coerce('node v3.3.0 (build)'), '3.3.0')
    assert.equal(semver.coerce('3.3'), '3.3.0')
    assert.equal(semver.coerce(undefined), null)
    assert.equal(semver.coerce('unknown'), null)
    assert.equal(semver.gte('3.3.0', '3.3.0'), true)
    assert.equal(semver.gte('3.2.9', '3.3.0'), false)
})

test('byte formatting retains IEC, SI, bit rate and zero behavior', () => {
    assert.equal(prettifyBytesUtil(0), undefined)
    assert.equal(prettifyBytesUtil(0, true), '0')
    assert.equal(prettifyBytesUtil('1024'), '1.00 KiB')
    assert.equal(prettifyBytesUtil(-1500), '-1.46 KiB')
    assert.equal(prettySiBytesUtil(1500), '1.50 KB')
    assert.equal(prettySiBytesUtil(undefined, true), '0 B')
    assert.equal(prettySiRealtimeBytesUtil(1500, false, true), '12.00 Kb/s')
    assert.equal(prettySiRealtimeBytesUtil(1500), '12.00 Kb')
    assert.equal(prettifyBytesUtil(0.5), '0.50 B')
})

test('favicon resolver normalizes web hosts and rejects invalid addresses', () => {
    assert.equal(
        faviconResolver('example.com/path'),
        'https://www.google.com/s2/favicons?sz=64&domain_url=https://example.com'
    )
    assert.equal(faviconResolver('//example.com/a'), faviconResolver('https://example.com/a'))
    assert.equal(
        faviconResolver('http://example.com:8080'),
        'https://www.google.com/s2/favicons?sz=64&domain_url=http://example.com:8080'
    )
    assert.equal(faviconResolver('file:///tmp/a'), null)
    assert.equal(faviconResolver('https://'), null)
    assert.equal(faviconResolver(null), null)
})

test('colors remain deterministic valid CSS hex values', () => {
    const colors = new ColorHash({ lightness: [0.65], saturation: 0.6 })
    assert.match(colors.hex('tag'), /^#[0-9a-f]{6}$/)
    assert.equal(colors.hex('tag'), colors.hex('tag'))
    assert.notEqual(colors.hex('tag'), colors.hex('other'))
    assert.equal(colorFromId(42), colorFromId(42n))
    assert.match(new ColorHash().hex('世界'), /^#[0-9a-f]{6}$/)
})

test('sparkline geometry handles empty, single, constant and invalid values', () => {
    assert.deepEqual(sparklinePaths([], false), { line: '', area: '' })
    for (const values of [[0], [4], [1, 1, 1], [1, 2, 3], [NaN, Infinity, -2]]) {
        const { line, area } = sparklinePaths(values, true)
        assert.ok(!/NaN|Infinity/.test(line + area))
        assert.match(area, /Z$/)
    }
    assert.match(sparklinePaths([1, 2], true).line, / C/)
    assert.match(sparklinePaths([1, 2], false).line, / L/)
})

test('assertion helper narrows truthy values and throws meaningful errors', () => {
    assert.doesNotThrow(() => invariant({ value: true }, 'missing'))
    assert.throws(() => invariant(null, 'missing provider'), /missing provider/)
})
