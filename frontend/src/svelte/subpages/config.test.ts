import assert from 'node:assert/strict'
import { test } from 'node:test'
import { BASE_TRANSLATION_KEYS } from '@remnawave/subscription-page-types'
import { moveItem } from '@shared/lib/move-selection'

import { localized, mergePageConfig, type PageConfig, readPageConfig } from './config'

function page(): PageConfig {
    return readPageConfig({
        version: '1',
        locales: ['en'],
        brandingSettings: { title: 'Brand', logoUrl: '', supportUrl: 'https://example.test' },
        uiConfig: { subscriptionInfoBlockType: 'expanded', installationGuidesBlockType: 'cards' },
        baseTranslations: Object.fromEntries(
            BASE_TRANSLATION_KEYS.map((key) => [key, { en: key }])
        ),
        svgLibrary: { Old: '<svg xmlns="http://www.w3.org/2000/svg"/>' },
        platforms: { ios: { displayName: { en: 'iOS' }, svgIconKey: 'Old', apps: [] } }
    })
}

test('page config editing retains stored extensions and validates locale and icon references', () => {
    const base = page()
    const saved = readPageConfig({ ...base, pluginExtension: { enabled: true } })
    assert.deepEqual(Reflect.get(saved, 'pluginExtension'), { enabled: true })
    assert.equal(saved.baseSettings.showConnectionKeys, false)
    assert.throws(
        () => readPageConfig({ ...base, locales: ['en', 'fa'] }),
        /Missing required locale/
    )
    assert.throws(() => readPageConfig({ ...base, svgLibrary: {} }), /Unknown svgIconKey/)
})

test('section imports merge icons and replace only the selected parts', () => {
    const current = page()
    const incoming = {
        ...page(),
        brandingSettings: { ...current.brandingSettings, title: 'Imported' },
        svgLibrary: { Old: 'updated', New: 'new icon' },
        platforms: {
            android: { apps: [], displayName: localized(['en'], 'Android'), svgIconKey: 'New' }
        }
    } satisfies PageConfig
    assert.equal(mergePageConfig(current, incoming, 'full'), incoming)
    const icons = mergePageConfig(current, incoming, 'svgLibrary')
    assert.deepEqual(icons.svgLibrary, { Old: 'updated', New: 'new icon' })
    assert.equal(icons.platforms, current.platforms)
    const platforms = mergePageConfig(current, incoming, 'platforms')
    assert.equal(platforms.platforms, incoming.platforms)
    assert.equal(platforms.brandingSettings, current.brandingSettings)
    assert.equal(
        mergePageConfig(current, incoming, 'baseTranslations').svgLibrary,
        current.svgLibrary
    )
    assert.equal(current.svgLibrary['Old'], '<svg xmlns="http://www.w3.org/2000/svg"/>')
})

test('reordering a section preserves its items and ignores out-of-bounds moves', () => {
    const items = ['a', 'b', 'c']
    moveItem(items, 0, -1)
    moveItem(items, 2, -1)
    assert.deepEqual(items, ['a', 'c', 'b'])
})
