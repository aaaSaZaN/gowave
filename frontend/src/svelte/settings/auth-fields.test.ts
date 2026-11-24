import assert from 'node:assert/strict'
import { test } from 'node:test'
import { buildScopes, expandScopesToKeys, type ScopeResource } from '@shared/lib/api-token-scopes'

import { readAuthentication } from './auth-fields'

test('authentication forms round-trip OAuth providers and the disabled legacy contract field', () => {
    const form = new FormData()
    form.set('password', 'on')
    form.set('passkey', 'on')
    form.set('rpId', ' example.org ')
    form.set('origin', 'https://example.org')
    form.set('generic.allowedEmails', 'a@example.org; a@example.org, b@example.org')
    form.set('generic.clientSecret', 'sample-secret ')
    form.set('generic.withPkce', 'on')
    form.set('telegram.allowedIds', '987 654')

    const result = readAuthentication(form)
    assert.equal(result.passwordSettings?.enabled, true)
    assert.equal(result.passkeySettings?.rpId, 'example.org')
    assert.equal(result.oauth2Settings?.generic.withPkce, true)
    assert.equal(result.oauth2Settings?.generic.clientSecret, 'sample-secret ')
    assert.deepEqual(result.oauth2Settings?.generic.allowedEmails, [
        'a@example.org',
        'b@example.org'
    ])
    assert.deepEqual(result.oauth2Settings?.telegram.allowedIds, ['987', '654'])
    assert.equal(result.oauth2Settings?.yandex.enabled, false)
})

test('scope imports and selection preserve permission boundaries', () => {
    const resources: ScopeResource[] = [
        {
            resource: 'users',
            resourceScopes: ['users:read', 'users:write', 'users:*'],
            endpoints: [
                {
                    key: 'users:list',
                    kind: 'read',
                    method: 'GET',
                    path: '/api/users',
                    description: 'List users'
                },
                {
                    key: 'users:create',
                    kind: 'write',
                    method: 'POST',
                    path: '/api/users',
                    description: 'Create a user'
                }
            ]
        }
    ]
    const selected = expandScopesToKeys(resources, ['users:read', 'missing:*'])
    assert.deepEqual(selected, ['users:list'])
    assert.deepEqual(buildScopes(resources, new Set(selected)), ['users:read'])
    assert.deepEqual(buildScopes(resources, new Set(expandScopesToKeys(resources, ['*']))), ['*'])
})
