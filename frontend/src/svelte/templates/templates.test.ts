import assert from 'node:assert/strict'
import { test } from 'node:test'
import { UpdateSubscriptionTemplateCommand } from '@remnawave/backend-contract'

import { type Template, templateContent, templateText, templateType } from './templates'

const defaults: Template = {
    uuid: 'bfbb53cf-c55e-482c-ae53-a7b33eec4ebc',
    name: 'Test',
    tags: [],
    viewPosition: 0,
    templateType: 'MIHOMO',
    templateJson: null,
    encodedTemplateYaml: null
}

test('YAML templates preserve UTF-8 content and line endings across the API envelope', () => {
    const draft = '# 東京\r\nmode: rule\r\n'
    const body = UpdateSubscriptionTemplateCommand.RequestBodySchema.parse({
        uuid: defaults.uuid,
        ...templateContent(draft, 'MIHOMO')
    })
    const saved = { ...defaults, encodedTemplateYaml: body.encodedTemplateYaml ?? null }

    assert.equal(templateText(saved, 'MIHOMO'), draft)
    assert.ok(!Object.hasOwn(body, 'templateJson'))
})

test('JSON template drafts must be objects and allowed URL types are explicit', () => {
    assert.deepEqual(templateContent('{"log":{"disabled":true}}', 'XRAY_JSON'), {
        templateJson: { log: { disabled: true } }
    })
    assert.throws(() => templateContent('{', 'SINGBOX'), SyntaxError)
    assert.equal(
        UpdateSubscriptionTemplateCommand.RequestBodySchema.safeParse({
            uuid: defaults.uuid,
            ...templateContent('[]', 'XRAY_JSON')
        }).success,
        false
    )
    assert.equal(templateType('XRAY_JSON'), 'XRAY_JSON')
    assert.equal(templateType('constructor'), undefined)
    assert.equal(templateType('XRAY_BASE64'), undefined)
})
