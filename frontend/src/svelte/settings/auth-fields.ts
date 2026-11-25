import { UpdateRemnawaveSettingsCommand } from '@remnawave/backend-contract'

import type { Settings } from './queries'

export const providers = [
    {
        key: 'telegram',
        title: 'Telegram',
        fields: ['clientId', 'clientSecret', 'frontendDomain', 'allowedIds']
    },
    { key: 'github', title: 'GitHub', fields: ['clientId', 'clientSecret', 'allowedEmails'] },
    {
        key: 'pocketid',
        title: 'PocketID',
        fields: ['clientId', 'clientSecret', 'plainDomain', 'frontendDomain', 'allowedEmails']
    },
    {
        key: 'keycloak',
        title: 'Keycloak',
        fields: [
            'clientId',
            'clientSecret',
            'realm',
            'frontendDomain',
            'keycloakDomain',
            'allowedEmails'
        ]
    },
    {
        key: 'generic',
        title: 'Generic OAuth2',
        fields: [
            'clientId',
            'clientSecret',
            'frontendDomain',
            'authorizationUrl',
            'tokenUrl',
            'allowedEmails',
            'withPkce'
        ]
    }
] as const

export type ProviderField = (typeof providers)[number]['fields'][number]

export function optionalText(values: FormData, name: string): string | null {
    const value = values.get(name)
    return typeof value === 'string' ? value.trim() || null : null
}

export function readAuthentication(values: FormData) {
    return UpdateRemnawaveSettingsCommand.RequestBodySchema.parse({
        passwordSettings: { enabled: values.has('password') },
        passkeySettings: {
            enabled: values.has('passkey'),
            rpId: optionalText(values, 'rpId'),
            origin: optionalText(values, 'origin')
        },
        oauth2Settings: {
            yandex: { enabled: false, clientId: null, clientSecret: null, allowedEmails: [] },
            ...Object.fromEntries(
                providers.map(({ key, fields }) => [
                    key,
                    {
                        enabled: values.has(`${key}.enabled`),
                        ...Object.fromEntries(
                            fields.map((field) => {
                                const name = `${key}.${field}`
                                if (field === 'withPkce') return [field, values.has(name)]
                                if (field === 'clientSecret') {
                                    const secret = values.get(name)
                                    return [
                                        field,
                                        typeof secret === 'string' ? secret || null : null
                                    ]
                                }
                                const text = optionalText(values, name)
                                if (field === 'allowedEmails' || field === 'allowedIds') {
                                    return [
                                        field,
                                        [...new Set(text?.split(/[\s,;]+/).filter(Boolean) ?? [])]
                                    ]
                                }
                                return [field, text]
                            })
                        )
                    }
                ])
            )
        }
    })
}

export function oauthField(
    provider: NonNullable<Settings['oauth2Settings']>[(typeof providers)[number]['key']],
    field: ProviderField
): string {
    const fields: Partial<Record<ProviderField, string | string[] | null | boolean>> = provider
    const value = fields[field]
    return Array.isArray(value) ? value.join(', ') : typeof value === 'string' ? value : ''
}
