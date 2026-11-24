import type { GetSnippetsCommand } from '@remnawave/backend-contract'
import consola from '@shared/lib/logger'
import { setupHostJsonFields, setupHostMapperSchema } from '@shared/utils/monaco/host-schemas'
import { registerJsonSchema } from '@shared/utils/monaco/json-schema-registry'
import { setupNodePluginSchema, setupSharedListSchema } from '@shared/utils/monaco/plugin-schemas'
import axios from 'axios'
import { app } from 'src/config'

interface ISchemaNode {
    allOf?: ISchemaNode[]
    anyOf?: ISchemaNode[]
    oneOf?: ISchemaNode[]
    properties?: Record<string, unknown>
}

interface IXraySchema {
    $ref?: unknown
    definitions?: Record<string, ISchemaNode | undefined>
}

function allowLegacyWebsocketAlias(schema: IXraySchema): void {
    const network = schema.definitions?.['StreamSettingsObject']?.properties?.['network'] as
        | { anyOf?: { const?: unknown; title?: string }[] }
        | undefined
    if (!network?.anyOf || network.anyOf.some((value) => value.const === 'ws')) return

    network.anyOf.push({ const: 'ws', title: 'websocket (legacy alias: ws)' })
}

const DEFINITIONS_REF_PREFIX = '#/definitions/'
const PROTECTED_ROOT_KEYS = new Set(['api', 'inbounds', 'metrics', 'snippets', 'stats'])

const CUSTOM_CORE_SCHEMA = {
    title: 'Gowave Custom Core',
    markdownDescription: [
        '**Gowave custom field.** Not part of Xray-Core – it is handled by the Gowave Node.',
        '',
        '> ⚠️ **Beta feature. Use strictly at your own risk.**',
        '>',
        '> It may be changed or removed at any time without prior notice.',
        '',
        'Replaces the Xray-Core binary on every node running this config profile with the one downloaded from `url`.',
        '',
        'The node verifies the `sha256` checksum before installing anything, keeps the bundled core untouched, and rolls back to it as soon as this section is removed.',
        '',
        '```json',
        '{',
        '  "geodata": {',
        '    "core": {',
        '      "url": "https://example.com/xray-linux-amd64",',
        '      "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"',
        '    }',
        '  }',
        '}',
        '```'
    ].join('\n'),
    type: 'object',
    additionalProperties: false,
    required: ['url', 'sha256'],
    properties: {
        url: {
            title: 'Core URL',
            markdownDescription: [
                'Direct link to the Xray-Core binary to install on the node.',
                '',
                '> Only `https` is accepted. The file is downloaded on every node that runs this config profile.'
            ].join('\n'),
            type: 'string',
            pattern: '^https://\\S+$',
            patternErrorMessage: 'URL must start with https:// '
        },
        sha256: {
            title: 'Core SHA-256',
            markdownDescription: [
                'SHA-256 checksum of the binary, 64 hexadecimal characters.',
                '',
                '> The node refuses to install the binary if the checksum does not match, so a wrong value here leaves the currently installed core in place.'
            ].join('\n'),
            type: 'string',
            pattern: '^[A-Fa-f0-9]{64}$',
            patternErrorMessage: 'SHA-256 must be exactly 64 hexadecimal characters'
        }
    }
}

const injectProperty = (
    node: ISchemaNode | undefined,
    propertyName: string,
    propertySchema: object
): number => {
    if (!node || typeof node !== 'object') {
        return 0
    }

    let injected = 0

    if (node.properties) {
        node.properties[propertyName] = propertySchema
        injected += 1
    }

    for (const branch of [...(node.anyOf ?? []), ...(node.oneOf ?? []), ...(node.allOf ?? [])]) {
        injected += injectProperty(branch, propertyName, propertySchema)
    }

    return injected
}

const resolveRootNode = (schema: IXraySchema | undefined): ISchemaNode | undefined => {
    const ref = schema?.$ref

    if (typeof ref !== 'string' || !ref.startsWith(DEFINITIONS_REF_PREFIX)) {
        return undefined
    }

    return schema?.definitions?.[ref.slice(DEFINITIONS_REF_PREFIX.length)]
}

export const MonacoSetupFeature = {
    setup: async (
        currentLanguage: string,
        snippets: GetSnippetsCommand.Response['response']['snippets']
    ) => {
        try {
            const snippetNames = snippets.map((s) => s.name)

            let { jsonSchemaUrl } = app.configEditor
            switch (currentLanguage) {
                case 'zh':
                    jsonSchemaUrl = app.configEditor.jsonSchemaCnUrl
                    break
                default:
                    jsonSchemaUrl = app.configEditor.jsonSchemaUrl
            }

            const response = await axios.get(jsonSchemaUrl)
            const schema = response.data
            allowLegacyWebsocketAlias(schema)

            const snippetDescriptions = snippets.map((snippet) => {
                const snippetJson = JSON.stringify(snippet.snippet, null, 1)

                return ['', '```json', snippetJson.slice(2, -2), '```', '', '---', ''].join('\n')
            })

            const snippetSchema = {
                name: 'snippet',
                title: 'Gowave Snippets',
                markdownDescription:
                    'Create your own snippets to quickly configure your **Outbounds** or **Rules**. \n\n\nReference them here, Gowave will handle the rest.',
                type: 'string',
                enum: snippetNames,
                markdownEnumDescriptions: snippetDescriptions,
                minLength: 2,
                maxLength: 255,
                pattern: '^[A-Za-z0-9_ -]+(/[A-Za-z0-9_ -]+)*$',
                patternErrorMessage:
                    'Snippet name can only contain letters, numbers, spaces, _ and -. Use / to nest, but not at the start or end and never doubled.'
            }

            const rootSnippetsSchema = {
                name: 'snippets',
                title: 'Gowave Snippets',
                markdownDescription: [
                    'Snippets merged into the **root** of this config.',
                    '',
                    'Every object of a listed snippet must hold root-level sections, for example:',
                    '```json',
                    '[{ "log": { "loglevel": "debug" } }]',
                    '```',
                    'Sections already written in this config are kept – a snippet never overwrites them.',
                    '',
                    '`inbounds`, `api`, `stats` and `metrics` are always skipped.'
                ].join('\n'),
                type: 'array',
                items: {
                    ...snippetSchema,
                    title: 'Snippet name'
                },
                minItems: 1
            }

            const notInjected: string[] = (
                ['OutboundObject', 'RuleObject', 'BalancerObject'] as const
            ).filter(
                (definition) =>
                    injectProperty(schema.definitions?.[definition], 'snippet', snippetSchema) === 0
            )

            const rootNode = resolveRootNode(schema)

            if (injectProperty(rootNode, 'snippets', rootSnippetsSchema) === 0) {
                notInjected.push('config root')
            }

            if (notInjected.length > 0) {
                consola.error(
                    `Failed to inject the snippet property into the Xray schema: ${notInjected.join(', ')}.`
                )
            }

            if (
                injectProperty(schema.definitions?.GeodataObject, 'core', CUSTOM_CORE_SCHEMA) ===
                    0 &&
                rootNode?.properties?.['geodata']
            ) {
                consola.error('Failed to inject the custom core property into GeodataObject.')
            }

            registerJsonSchema({
                fileMatch: ['xray-config://*/config.json'],
                schema,
                uri: 'https://xray-config-schema.json'
            })
        } catch (error) {
            consola.error('Failed to load JSON schema:', error)
        }
    }
}

export const MonacoSetupSnippetsFeature = {
    setup: async (currentLanguage: string) => {
        try {
            let { jsonSchemaUrl } = app.configEditor
            switch (currentLanguage) {
                case 'zh':
                    jsonSchemaUrl = app.configEditor.jsonSchemaCnUrl
                    break
                default:
                    jsonSchemaUrl = app.configEditor.jsonSchemaUrl
            }

            const response = await axios.get(jsonSchemaUrl)
            const schema = await response.data

            const rootNode = resolveRootNode(schema)

            injectProperty(schema.definitions?.GeodataObject, 'core', CUSTOM_CORE_SCHEMA)

            const snippetArraySchema = {
                $schema: 'http://json-schema.org/draft-07/schema#',
                title: 'Snippet Array',
                description: 'Array of Root, Outbound, Rule or Balancer objects for snippets',
                type: 'array',
                items: {
                    oneOf: [
                        {
                            ...schema.definitions?.OutboundObject,
                            title: 'Outbound Object',
                            description: 'Outbound configuration (for outbounds[])'
                        },
                        {
                            ...schema.definitions?.RuleObject,
                            title: 'Rule Object',
                            description: 'Routing rule (for routing.rules[])'
                        },
                        {
                            ...schema.definitions?.BalancerObject,
                            title: 'Balancer Object',
                            description: 'Balancer configuration (for routing.balancers[])'
                        },
                        ...(rootNode?.properties
                            ? [
                                  {
                                      ...rootNode,
                                      properties: Object.fromEntries(
                                          Object.entries(rootNode.properties).filter(
                                              ([key]) => !PROTECTED_ROOT_KEYS.has(key)
                                          )
                                      ),
                                      title: 'Root Object',
                                      description:
                                          'Root-level sections (for configs referencing this snippet in the root "snippets" array)',
                                      markdownDescription:
                                          'Root-level sections, merged into the **root** of every config that lists this snippet in its root `snippets` array. \n\n\n`inbounds`, `api`, `stats` and `metrics` cannot be provided by a snippet.'
                                  }
                              ]
                            : [])
                    ]
                },
                minItems: 1,
                definitions: schema.definitions || {}
            }

            registerJsonSchema({
                fileMatch: ['snippet://*/fragment.json'],
                schema: snippetArraySchema,
                uri: 'https://snippet-schema.json'
            })

            return snippetArraySchema
        } catch (error) {
            consola.error('Failed to load snippet JSON schema:', error)
            return null
        }
    }
}

export const MonacoSetupHostMapperEditorFeature = { setup: setupHostMapperSchema }

export const MonacoSetupNodePluginEditorFeature = { setup: setupNodePluginSchema }

export const MonacoSetupSharedListEditorFeature = { setup: setupSharedListSchema }

export const MonacoSetupHostJsonFieldsFeature = { setup: setupHostJsonFields }
