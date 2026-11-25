import type { TSubscriptionTemplateType } from '@remnawave/backend-contract'
import { ResponseRulesConfigSchema } from '@shared/api/schemas/subscription-settings.schema'
import consola from '@shared/lib/logger'

import { registerJsonSchema } from './json-schema-registry'

export function setupResponseRulesSchema(
    groupedTemplates: Partial<Record<TSubscriptionTemplateType, string[]>>
): void {
    try {
        const schema = ResponseRulesConfigSchema.toJSONSchema({
            target: 'draft-07',
            io: 'input'
        })

        const templateOptions = {
            BROWSER: [],
            BLOCK: [],
            STATUS_CODE_404: [],
            STATUS_CODE_451: [],
            SOCKET_DROP: [],
            ...groupedTemplates
        }

        const rules = schema.properties?.['rules']
        const rulesItems =
            typeof rules === 'object' &&
            typeof rules.items === 'object' &&
            !Array.isArray(rules.items)
                ? rules.items
                : undefined

        if (rulesItems) {
            rulesItems.allOf ??= []
            const rulesAllOf = rulesItems.allOf

            Object.entries(templateOptions).forEach(([responseType, templates]) => {
                if (templates.length > 0) {
                    rulesAllOf.push({
                        if: {
                            properties: {
                                responseType: { const: responseType }
                            },
                            required: ['responseType']
                        },
                        // biome-ignore lint/suspicious/noThenProperty: JSON Schema conditional keyword, not a Promise handler.
                        then: {
                            properties: {
                                responseModifications: {
                                    properties: {
                                        subscriptionTemplate: {
                                            enum: templates,
                                            markdownDescription: `Available templates for **${responseType}** response type.`,
                                            markdownEnumDescriptions: templates.map(
                                                (t) => `Use ${t} template`
                                            )
                                        }
                                    }
                                }
                            }
                        }
                    })
                } else {
                    rulesAllOf.push({
                        if: {
                            properties: {
                                responseType: { const: responseType }
                            },
                            required: ['responseType']
                        },
                        // biome-ignore lint/suspicious/noThenProperty: JSON Schema conditional keyword, not a Promise handler.
                        then: {
                            properties: {
                                responseModifications: {
                                    properties: {
                                        subscriptionTemplate: {
                                            type: 'null',
                                            not: { type: 'string' },
                                            markdownDescription: `⚠️ No templates available for **${responseType}** response type. This field should not be used.`
                                        }
                                    }
                                }
                            }
                        }
                    })
                }
            })
        }

        registerJsonSchema(
            {
                fileMatch: ['response-rules://*'],
                schema,
                uri: 'https://response-rules-schema.json'
            },
            {
                comments: 'error',
                schemaValidation: 'error',
                trailingCommas: 'error'
            }
        )
    } catch (error) {
        consola.error('Failed to load JSON schema:', error)
    }
}
