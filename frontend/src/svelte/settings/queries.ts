import {
    GetApiTokenScopesCommand,
    GetApiTokensCommand,
    GetPasskeysCommand,
    GetRemnawaveSettingsCommand
} from '@remnawave/backend-contract'
import { queryOptions } from '@tanstack/svelte-query'

import { callCommand } from '../commands'

export type Settings = GetRemnawaveSettingsCommand.Response['response']
export type ApiToken = GetApiTokensCommand.Response['response']['tokens'][number]

export const panelSettings = queryOptions({
    queryKey: ['gowaveSettings', 'getGowaveSettings'],
    queryFn: ({ signal }) => callCommand(GetRemnawaveSettingsCommand, { signal }),
    gcTime: 0
})

export const apiTokens = queryOptions({
    queryKey: ['apiTokens', 'getAllApiTokens'],
    queryFn: ({ signal }) => callCommand(GetApiTokensCommand, { signal })
})

export const apiScopes = queryOptions({
    queryKey: ['apiTokens', 'getApiTokenScopes'],
    queryFn: ({ signal }) => callCommand(GetApiTokenScopesCommand, { signal }),
    staleTime: 60_000
})

export const passkeys = queryOptions({
    queryKey: ['passkeys', 'getPasskeys'],
    queryFn: ({ signal }) => callCommand(GetPasskeysCommand, { signal })
})
