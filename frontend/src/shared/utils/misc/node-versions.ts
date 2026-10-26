interface NodeVersions {
    node?: string | null
    xray?: string | null
    nodeType?: string
    isCustom?: boolean
    isCustomXray?: boolean
}

export function isGoNodeVersion(versions: NodeVersions | null | undefined): boolean {
    return versions?.nodeType === 'go' || Boolean(versions?.node?.includes('(Go)'))
}

export function isCustomXrayVersion(versions: NodeVersions | null | undefined): boolean {
    const version = versions?.xray?.toLowerCase()
    return Boolean(
        versions?.isCustom ||
            versions?.isCustomXray ||
            version?.includes('custom') ||
            version?.includes('mod')
    )
}
