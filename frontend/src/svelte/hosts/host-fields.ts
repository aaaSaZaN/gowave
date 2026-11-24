import {
    type CreateHostCommand,
    INTERNAL_SQUADS_MODE,
    SECURITY_LAYERS
} from '@remnawave/backend-contract'

export const textOptions = {
    path: 'base-host-form.path',
    sni: 'base-host-form.sni',
    host: 'base-host-form.host',
    fingerprint: 'base-host-form.fingerprint',
    serverDescription: 'base-host-form.server-description-header',
    pinnedPeerCertSha256: 'base-host-form.pinned-peer-cert-sha256',
    verifyPeerCertByName: 'base-host-form.verify-peer-cert-by-name'
} as const

export const switches = {
    isDisabled: 'use-hosts-table-widget.disabled',
    isHidden: 'base-host-form.hide-host',
    shuffleHost: 'base-host-form.shuffle-host',
    overrideSniFromAddress: 'base-host-form.override-sni-from-address',
    keepSniBlank: 'base-host-form.keep-sni-blank',
    mihomoX25519: 'base-host-form.enable-x25519mlkem768'
} as const

export const jsonOptions = {
    xhttpExtraParams: 'base-host-form.extra-xhttp',
    muxParams: 'base-host-form.mux-params',
    sockoptParams: 'base-host-form.sockopt-params',
    finalMask: 'base-host-form.final-mask-params',
    mapper: 'base-host-form.mapper'
} as const

export type HostField = keyof CreateHostCommand.RequestBody

export function formText(data: FormData, key: string): string {
    const value = data.get(key)
    return typeof value === 'string' ? value : ''
}

function textList(input: string, separator: RegExp): string[] {
    return [
        ...new Set(
            input
                .split(separator)
                .map((word) => word.trim())
                .filter(Boolean)
        )
    ]
}

export function parseHostJson(input: string, mapper = false): unknown {
    if (!input.trim()) return mapper ? {} : null

    const value: unknown = JSON.parse(input)
    if (mapper) return value
    if (value === null || typeof value === 'object' || typeof value === 'string') return value
    throw new Error('Expected a JSON object, array, string, or null')
}

export function hostPatch(data: FormData, includes: (key: HostField) => boolean) {
    const patch: Partial<Record<HostField, unknown>> = {}
    const nullable = (key: HostField) => formText(data, key).trim() || null
    const readSelected = (key: string) =>
        data.getAll(key).filter((value) => typeof value === 'string')

    for (const key of ['remark', 'address'] as const) {
        if (includes(key)) {
            const value = formText(data, key).trim()
            if (!value) throw new Error(`Enter a host ${key}`)
            patch[key] = value
        }
    }
    if (includes('port')) {
        const port = Number(formText(data, 'port'))
        if (!Number.isInteger(port) || port < 1 || port > 65535)
            throw new Error('Port must be between 1 and 65535')
        patch.port = port
    }
    if (includes('inbound')) {
        const [configProfileUuid, configProfileInboundUuid] = formText(data, 'inbound').split('/')
        patch.inbound = { configProfileUuid, configProfileInboundUuid }
    }
    for (const key of [
        ...(Object.keys(textOptions) as Array<keyof typeof textOptions>),
        'alpn',
        'mihomoIpVersion',
        'xrayJsonTemplateUuid'
    ] as const) {
        if (includes(key)) patch[key] = nullable(key)
    }
    for (const key of Object.keys(switches) as Array<keyof typeof switches>) {
        if (includes(key)) patch[key] = data.has(key)
    }
    if (includes('securityLayer'))
        patch.securityLayer = formText(data, 'securityLayer') || SECURITY_LAYERS.DEFAULT
    if (includes('vlessRouteId')) {
        const value = nullable('vlessRouteId')
        patch.vlessRouteId = value === null ? null : Number(value)
    }
    if (includes('tags')) patch.tags = textList(formText(data, 'tags').toUpperCase(), /[,\s]+/)
    for (const key of ['nodes', 'excludeFromSubscriptionTypes', 'includeInSrrRules'] as const) {
        if (includes(key)) patch[key] = readSelected(key)
    }
    if (includes('internalSquads')) {
        patch.internalSquads = {
            mode: formText(data, 'internalSquads:mode') || INTERNAL_SQUADS_MODE.EXCLUDE,
            squads: readSelected('internalSquads:squads')
        }
    }
    for (const key of Object.keys(jsonOptions) as Array<keyof typeof jsonOptions>) {
        if (!includes(key)) continue
        try {
            patch[key] = parseHostJson(formText(data, key), key === 'mapper')
        } catch (error) {
            throw new Error(`Invalid JSON in ${key}`, { cause: error })
        }
    }
    return patch
}
