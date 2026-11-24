import type { GetApiTokenScopesCommand } from '@remnawave/backend-contract'

export type ScopeResource = GetApiTokenScopesCommand.Response['response']['resources'][number]
export type ScopeEndpoint = ScopeResource['endpoints'][number]
export type KindState = 'none' | 'off' | 'on' | 'partial'

export const humanizeResource = (resource: string): string =>
    resource
        .split('-')
        .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
        .join(' ')

export const getReadKeys = (resource: ScopeResource): string[] =>
    resource.endpoints
        .filter((endpoint) => endpoint.kind === 'read')
        .map((endpoint) => endpoint.key)

export const getWriteKeys = (resource: ScopeResource): string[] =>
    resource.endpoints
        .filter((endpoint) => endpoint.kind === 'write')
        .map((endpoint) => endpoint.key)

export const getAllKeys = (resource: ScopeResource): string[] =>
    resource.endpoints.map((endpoint) => endpoint.key)

export const expandScopesToKeys = (resources: ScopeResource[], scopes: string[]): string[] => {
    const byName = new Map(resources.map((resource) => [resource.resource, resource]))
    const keys = new Set<string>()

    for (const scope of scopes) {
        if (scope === '*') {
            resources.forEach((resource) => {
                resource.endpoints.forEach((endpoint) => {
                    keys.add(endpoint.key)
                })
            })
        } else {
            const separator = scope.indexOf(':')
            const resourceName = separator === -1 ? scope : scope.slice(0, separator)
            const rest = separator === -1 ? '' : scope.slice(separator + 1)
            const resource = byName.get(resourceName)

            if (resource) {
                if (rest === '*')
                    getAllKeys(resource).forEach((key) => {
                        keys.add(key)
                    })
                else if (rest === 'read')
                    getReadKeys(resource).forEach((key) => {
                        keys.add(key)
                    })
                else if (rest === 'write')
                    getWriteKeys(resource).forEach((key) => {
                        keys.add(key)
                    })
                else if (resource.endpoints.some((endpoint) => endpoint.key === scope))
                    keys.add(scope)
            }
        }
    }

    return Array.from(keys)
}

export const getKindState = (keys: string[], selectedEndpoints: Set<string>): KindState => {
    if (keys.length === 0) return 'none'
    const selected = keys.filter((key) => selectedEndpoints.has(key)).length
    if (selected === 0) return 'off'
    if (selected === keys.length) return 'on'
    return 'partial'
}

export const countSelected = (endpoints: ScopeEndpoint[], selectedEndpoints: Set<string>): number =>
    endpoints.reduce((acc, endpoint) => acc + (selectedEndpoints.has(endpoint.key) ? 1 : 0), 0)

const buildResourceScopes = (resource: ScopeResource, selectedEndpoints: Set<string>): string[] => {
    const selected = resource.endpoints.filter((endpoint) => selectedEndpoints.has(endpoint.key))
    if (selected.length === 0) return []

    if (selected.length === resource.endpoints.length) return [`${resource.resource}:*`]

    const reads = resource.endpoints.filter((endpoint) => endpoint.kind === 'read')
    const writes = resource.endpoints.filter((endpoint) => endpoint.kind === 'write')
    const selectedReads = reads.filter((endpoint) => selectedEndpoints.has(endpoint.key))
    const selectedWrites = writes.filter((endpoint) => selectedEndpoints.has(endpoint.key))

    const scopes: string[] = []
    const covered = new Set<string>()

    if (reads.length > 0 && selectedReads.length === reads.length) {
        scopes.push(`${resource.resource}:read`)
        reads.forEach((endpoint) => {
            covered.add(endpoint.key)
        })
    }
    if (writes.length > 0 && selectedWrites.length === writes.length) {
        scopes.push(`${resource.resource}:write`)
        writes.forEach((endpoint) => {
            covered.add(endpoint.key)
        })
    }

    selected.forEach((endpoint) => {
        if (!covered.has(endpoint.key)) scopes.push(endpoint.key)
    })

    return scopes
}

export const buildScopes = (
    resources: ScopeResource[],
    selectedEndpoints: Set<string>
): string[] => {
    const allSelected =
        resources.length > 0 &&
        resources.every((resource) =>
            resource.endpoints.every((endpoint) => selectedEndpoints.has(endpoint.key))
        )
    if (allSelected) return ['*']

    return resources.flatMap((resource) => buildResourceScopes(resource, selectedEndpoints))
}
