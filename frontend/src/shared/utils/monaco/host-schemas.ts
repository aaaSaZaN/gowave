import { HostMapperSchema } from '@remnawave/backend-contract'
import logger from '@shared/lib/logger'
import axios from 'axios'
import { app } from 'src/config'

import { registerJsonSchema } from './json-schema-registry'

interface ISchemaNode {
    oneOf?: ISchemaNode[]
    properties?: Record<string, unknown>
}

interface IXraySchema {
    definitions?: Record<string, ISchemaNode | undefined>
}

const MAX_INBOUND_PATHS = 400
const MAX_INBOUND_DEPTH = 10
const MAX_INBOUND_VALUE_PREVIEW = 400

const collectInboundPaths = (
    value: unknown,
    prefix: string,
    depth: number,
    collected: Map<string, unknown>
) => {
    if (collected.size >= MAX_INBOUND_PATHS || depth > MAX_INBOUND_DEPTH) return
    if (value === null || typeof value !== 'object') return

    const entries = Array.isArray(value)
        ? value.map((item, index) => [String(index), item] as const)
        : Object.entries(value as Record<string, unknown>)

    for (const [key, child] of entries) {
        if (collected.size >= MAX_INBOUND_PATHS) return

        const path = prefix ? `${prefix}.${key}` : key

        collected.set(path, child)
        collectInboundPaths(child, path, depth + 1, collected)
    }
}

const injectInboundPaths = (schema: unknown, rawInbound: unknown) => {
    const collected = new Map<string, unknown>()
    collectInboundPaths(rawInbound, '', 0, collected)

    if (collected.size === 0) return

    const snippets = [...collected.entries()].map(([path, value]) => {
        const json = JSON.stringify(value, null, 2) ?? 'undefined'
        const preview =
            json.length > MAX_INBOUND_VALUE_PREVIEW
                ? `${json.slice(0, MAX_INBOUND_VALUE_PREVIEW)}\n…`
                : json

        return {
            label: path,
            body: path,
            markdownDescription: [
                '',
                'Current value in the inbound:',
                '',
                '```json',
                preview,
                '```'
            ].join('\n')
        }
    })

    const properties = (schema as { properties?: Record<string, { items?: ISchemaNode }> })
        .properties

    for (const client of ['xrayJson', 'mihomo', 'base64']) {
        for (const branch of properties?.[client]?.items?.oneOf ?? []) {
            const from = branch.properties?.['from'] as Record<string, unknown> | undefined

            if (!from) continue

            from['defaultSnippets'] = snippets
        }
    }
}

export function setupHostMapperSchema(rawInbound?: unknown): void {
    try {
        const schema = HostMapperSchema.toJSONSchema({ target: 'draft-07' })

        injectInboundPaths(schema, rawInbound)

        registerJsonSchema(
            {
                fileMatch: ['host-mapper://*'],
                schema,
                uri: 'https://host-mapper-schema.json'
            },
            {
                comments: 'error',
                schemaValidation: 'error',
                trailingCommas: 'error'
            }
        )
    } catch (error) {
        logger.error('Failed to load JSON schema:', error)
    }
}

const HOST_JSON_FIELD_SCHEMAS = [
    { definition: 'MuxObject', fileMatch: 'host-mux://*', uri: 'https://host-mux-schema.json' },
    {
        definition: 'SockoptObject',
        fileMatch: 'host-sockopt://*',
        uri: 'https://host-sockopt-schema.json'
    },
    {
        definition: 'FinalMaskObject',
        fileMatch: 'host-final-mask://*',
        uri: 'https://host-final-mask-schema.json'
    }
]

const buildFinalMaskProperties = (definitions: Record<string, ISchemaNode | undefined>) => {
    if (!definitions['TCPMask'] || !definitions['UDPMask']) {
        return undefined
    }

    return {
        quicParams: definitions['quicParams']
            ? { $ref: '#/definitions/quicParams' }
            : { type: 'object' },
        tcp: { type: 'array', items: { $ref: '#/definitions/TCPMask' } },
        udp: { type: 'array', items: { $ref: '#/definitions/UDPMask' } }
    }
}

export async function setupHostJsonFields(currentLanguage: string): Promise<void> {
    try {
        let { jsonSchemaUrl } = app.configEditor
        switch (currentLanguage) {
            case 'zh':
                jsonSchemaUrl = app.configEditor.jsonSchemaCnUrl
                break
            default:
                jsonSchemaUrl = app.configEditor.jsonSchemaUrl
        }

        const response = await axios.get<IXraySchema>(jsonSchemaUrl)
        const { definitions = {} } = response.data

        HOST_JSON_FIELD_SCHEMAS.forEach(({ definition, fileMatch, uri }) => {
            const node = definitions[definition]

            if (!node) {
                return
            }

            const properties =
                definition === 'FinalMaskObject' ? buildFinalMaskProperties(definitions) : undefined

            registerJsonSchema({
                fileMatch: [fileMatch],
                schema: {
                    ...node,
                    ...(properties ? { type: 'object', properties } : {}),
                    definitions
                },
                uri
            })
        })
    } catch (error) {
        logger.error('Failed to load JSON schema:', error)
    }
}
