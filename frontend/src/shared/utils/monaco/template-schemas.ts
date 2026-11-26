import {
    type GetHostsCommand,
    SUBSCRIPTION_TEMPLATE_TYPE,
    type TSubscriptionTemplateType
} from '@remnawave/backend-contract'
import consola from '@shared/lib/logger'
import { registerJsonSchema } from '@shared/utils/monaco/json-schema-registry'
import axios from 'axios'
import type { MonacoYaml, MonacoYamlOptions, SchemasSettings } from 'monaco-yaml'
import { app } from 'src/config'

type Host = GetHostsCommand.Response['response'][number]
type Monaco = typeof import('@shared/utils/setup-monaco/setup-monaco').monaco

interface IJsonSchemaDocument {
    [key: string]: unknown
    properties?: Record<string, unknown>
}

export const getTemplateModelPath = (templateType: TSubscriptionTemplateType, modelId: string) =>
    `subscription-template://${templateType.toLowerCase()}/${modelId}`

const YAML_OPTIONS: MonacoYamlOptions = {
    validate: true,
    enableSchemaRequest: true,
    hover: true,
    completion: true,
    format: {
        printWidth: 160,
        enable: true
    }
}

const COMMON_YAML_PORT = {
    type: 'integer',
    minimum: 0,
    maximum: 65535
} as const

let monacoYaml: Promise<MonacoYaml> | undefined

// monaco-yaml depends on monaco-editor, so it is imported lazily with the editor.
const configureYaml = async (monaco: Monaco) => {
    if (monacoYaml !== undefined) {
        await monacoYaml
        return
    }
    const schemas: SchemasSettings[] = [
        {
            fileMatch: [getTemplateModelPath(SUBSCRIPTION_TEMPLATE_TYPE.MIHOMO, '*')],
            uri: new URL(app.templateEditor.mihomoYamlSchemaUrl, window.location.origin).href
        },
        {
            fileMatch: [
                getTemplateModelPath(SUBSCRIPTION_TEMPLATE_TYPE.CLASH, '*'),
                getTemplateModelPath(SUBSCRIPTION_TEMPLATE_TYPE.STASH, '*')
            ],
            uri: 'gowave://schemas/clash-compatible-template.json',
            // Both clients extend the format independently. Validate the shared fields only.
            schema: {
                type: 'object',
                properties: {
                    port: COMMON_YAML_PORT,
                    'socks-port': COMMON_YAML_PORT,
                    'mixed-port': COMMON_YAML_PORT,
                    'redir-port': COMMON_YAML_PORT,
                    'tproxy-port': COMMON_YAML_PORT,
                    'allow-lan': { type: 'boolean' },
                    dns: { type: 'object' },
                    proxies: { type: ['array', 'null'] },
                    'proxy-groups': { type: ['array', 'null'] },
                    rules: { type: ['array', 'null'], items: { type: 'string' } }
                }
            }
        }
    ]
    const options: MonacoYamlOptions = {
        ...YAML_OPTIONS,
        schemas
    }

    monacoYaml = import('monaco-yaml')
        .then(({ configureMonacoYaml }) => configureMonacoYaml(monaco, options))
        .catch((error: unknown) => {
            monacoYaml = undefined
            throw error
        })
    await monacoYaml
}

const DOCS_URL = 'https://docs.rw/docs/learn/xray-json-advanced'
const DOCS_LINK = `\n\n[📖 Documentation](${DOCS_URL})`

function getHostStatus(host: Host): { icon: string; label: string } {
    if (host.isDisabled) return { icon: '⛔', label: 'Disabled' }
    if (host.isHidden) return { icon: '👁', label: 'Hidden' }
    return { icon: '✅', label: 'Active' }
}

function buildMarkdownDescription(host: Host): string {
    const { icon, label } = getHostStatus(host)

    const rows: string[] = [
        '',
        '| | |',
        '|:--|:--|',
        `| **Remark** | **${host.remark}** |`,
        `| **Address** | \`${host.address}:${host.port}\` |`,
        `| **Status** | ${icon} ${label} |`
    ]

    if (host.tags.length > 0) rows.push(`| **Tags** | \`${host.tags.join(', ')}\` |`)
    if (host.sni) rows.push(`| **SNI** | \`${host.sni}\` |`)
    if (host.serverDescription) rows.push(`| **Description** | ${host.serverDescription} |`)
    if (host.inbound.configProfileUuid) {
        rows.push(`| **Profile UUID** | \`${host.inbound.configProfileUuid}\` |`)
    }
    if (host.inbound.configProfileInboundUuid) {
        rows.push(`| **Inbound UUID** | \`${host.inbound.configProfileInboundUuid}\` |`)
    }

    return rows.join('\n')
}

export const configureMonaco = async (
    monaco: Monaco,
    language: 'json' | 'yaml',
    hosts: GetHostsCommand.Response['response'],
    templateType: TSubscriptionTemplateType
) => {
    try {
        if (language === 'yaml') {
            await configureYaml(monaco)
        }

        if (language === 'json') {
            const hostUuids = hosts.map((h) => h.uuid)
            const hostDescriptions = hosts.map(buildMarkdownDescription)

            const schema = {
                type: 'object',
                properties: {
                    remnawave: {
                        type: 'object',
                        properties: {
                            injectHosts: {
                                type: 'array',
                                items: {
                                    type: 'object',
                                    properties: {
                                        selector: {
                                            type: 'object',
                                            properties: {
                                                type: {
                                                    type: 'string',
                                                    enum: [
                                                        'uuids',
                                                        'remarkRegex',
                                                        'tagRegex',
                                                        'sameTagAsRecipient'
                                                    ]
                                                }
                                            },
                                            required: ['type'],
                                            allOf: [
                                                {
                                                    if: {
                                                        properties: {
                                                            type: { const: 'uuids' }
                                                        },
                                                        required: ['type']
                                                    },
                                                    // biome-ignore lint/suspicious/noThenProperty: JSON Schema conditional keyword, not a Promise handler.
                                                    then: {
                                                        properties: {
                                                            type: true,
                                                            values: {
                                                                type: 'array',
                                                                items: {
                                                                    type: 'string',
                                                                    format: 'uuid',
                                                                    enum: hostUuids,
                                                                    markdownEnumDescriptions:
                                                                        hostDescriptions,
                                                                    errorMessage:
                                                                        'No host found with this UUID'
                                                                },
                                                                minItems: 1
                                                            }
                                                        },
                                                        required: ['values'],
                                                        additionalProperties: false
                                                    }
                                                },
                                                {
                                                    if: {
                                                        properties: {
                                                            type: { const: 'remarkRegex' }
                                                        },
                                                        required: ['type']
                                                    },
                                                    // biome-ignore lint/suspicious/noThenProperty: JSON Schema conditional keyword, not a Promise handler.
                                                    then: {
                                                        properties: {
                                                            type: true,
                                                            pattern: {
                                                                type: 'string',
                                                                minLength: 1
                                                            }
                                                        },
                                                        required: ['pattern'],
                                                        additionalProperties: false
                                                    }
                                                },
                                                {
                                                    if: {
                                                        properties: {
                                                            type: { const: 'tagRegex' }
                                                        },
                                                        required: ['type']
                                                    },
                                                    // biome-ignore lint/suspicious/noThenProperty: JSON Schema conditional keyword, not a Promise handler.
                                                    then: {
                                                        properties: {
                                                            type: true,
                                                            pattern: {
                                                                type: 'string',
                                                                minLength: 1
                                                            }
                                                        },
                                                        required: ['pattern'],
                                                        additionalProperties: false
                                                    }
                                                },
                                                {
                                                    if: {
                                                        properties: {
                                                            type: {
                                                                const: 'sameTagAsRecipient'
                                                            }
                                                        },
                                                        required: ['type']
                                                    },
                                                    // biome-ignore lint/suspicious/noThenProperty: JSON Schema conditional keyword, not a Promise handler.
                                                    then: {
                                                        properties: { type: true },
                                                        additionalProperties: false
                                                    }
                                                }
                                            ]
                                        },
                                        selectFrom: {
                                            type: 'string',
                                            enum: ['ALL', 'HIDDEN', 'NOT_HIDDEN'],
                                            default: 'HIDDEN',
                                            markdownDescription: `Filter hosts by visibility. Defaults to HIDDEN if not specified. ${DOCS_LINK}`
                                        },
                                        tagPrefix: {
                                            type: 'string',
                                            minLength: 1
                                        },
                                        useHostRemarkAsTag: {
                                            type: 'boolean',
                                            markdownDescription: `Use host remark as tag. ${DOCS_LINK}`
                                        },
                                        useHostTagAsTag: {
                                            type: 'boolean',
                                            markdownDescription: `Use host tag as tag. ${DOCS_LINK}`
                                        }
                                    },
                                    required: ['selector'],
                                    anyOf: [
                                        { required: ['tagPrefix'] },
                                        {
                                            properties: {
                                                useHostRemarkAsTag: {
                                                    const: true
                                                }
                                            },
                                            required: ['useHostRemarkAsTag']
                                        },
                                        {
                                            properties: {
                                                useHostTagAsTag: {
                                                    const: true
                                                }
                                            },
                                            required: ['useHostTagAsTag']
                                        }
                                    ],
                                    additionalProperties: false
                                },
                                markdownDescription: `Inject hosts into the subscription template. ${DOCS_LINK}`
                            },
                            addVirtualHostAsOutbound: {
                                type: 'boolean',
                                markdownDescription: `When \`true\`, the recipient host (the host that owns this template) is added as an outbound with tag \`proxy\`. This allows routing rules that reference \`outboundTag: "proxy"\` to work correctly alongside injected hosts. Defaults to \`false\`. ${DOCS_LINK}`
                            }
                        },
                        additionalProperties: false
                    }
                }
            }

            registerJsonSchema({
                fileMatch: [
                    getTemplateModelPath(SUBSCRIPTION_TEMPLATE_TYPE.SINGBOX, '*'),
                    getTemplateModelPath(SUBSCRIPTION_TEMPLATE_TYPE.XRAY_JSON, '*')
                ],
                schema,
                uri: 'https://subscription-template-schema.json'
            })

            if (templateType === SUBSCRIPTION_TEMPLATE_TYPE.SINGBOX) {
                const response = await axios.get<IJsonSchemaDocument>(
                    app.templateEditor.singboxJsonSchemaUrl
                )
                const singboxSchema = response.data

                registerJsonSchema({
                    fileMatch: [getTemplateModelPath(templateType, '*')],
                    schema: {
                        ...singboxSchema,
                        properties: { ...singboxSchema.properties, remnawave: true }
                    },
                    uri: 'https://singbox-schema.json'
                })
            }
        }
    } catch (error) {
        consola.error(`Failed to configure Monaco ${language.toUpperCase()}:`, error)
        throw error
    }
}
