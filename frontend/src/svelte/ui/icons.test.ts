import assert from 'node:assert/strict'
import { test } from 'node:test'

import { getIcon } from './icons'

test('sign-in icons render immediately and dashboard icons share the loaded module', async () => {
    const login = getIcon('PiSignInDuotone')
    assert.ok(!(login instanceof Promise))
    assert.match(login.content, /<path/)

    const [chart, node] = await Promise.all([getIcon('PiChartBar'), getIcon('TbServer')])
    assert.equal(getIcon('PiChartBar'), chart)
    assert.equal(getIcon('TbServer'), node)
    assert.notEqual(chart.content, node.content)
})
