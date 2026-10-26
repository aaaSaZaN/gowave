import {
    GetSubscriptionSettingsCommand,
    ResponseRuleModificationsSchema,
    ResponseRuleSchema,
    TestSrrMatcherCommand,
    UpdateSubscriptionSettingsCommand,
    ResponseRulesConfigSchema as UpstreamResponseRulesConfigSchema
} from '@remnawave/backend-contract'
import { z } from 'zod'

const upstreamModifications = ResponseRuleModificationsSchema.unwrap()
const legacyHeaders = upstreamModifications.shape.headers.unwrap()

const headersSchema = z
    .union([
        z.record(legacyHeaders.element.shape.key, legacyHeaders.element.shape.value),
        legacyHeaders
            .meta({
                deprecated: true,
                description:
                    'Legacy key/value array. Accepted for compatibility; converted on save.'
            })
            .transform((headers) => {
                const entries = new Map<string, [string, string]>()
                for (const { key, value } of headers) {
                    const lower = key.toLowerCase()
                    entries.delete(lower)
                    entries.set(lower, [key, value])
                }
                return Object.fromEntries(entries.values())
            })
    ])
    .meta({
        description: 'Response headers as a JSON object mapping header names to string values.',
        defaultSnippets: [
            { label: 'Add custom header', body: { 'X-Custom-Header': 'CustomValue' } }
        ]
    })

const baseResponseRuleSchema = ResponseRuleSchema.extend({
    responseModifications: upstreamModifications
        .extend({ headers: headersSchema.optional() })
        .optional()
        .meta({
            title: 'Response Modifications',
            description: 'Optional response modifications. Headers use a JSON object.',
            examples: [{ headers: { 'X-Custom-Header': 'CustomValue' } }]
        })
})

const responseRuleSchema = baseResponseRuleSchema.meta({
    title: 'Response Rule',
    description: 'Match request headers and customize the subscription response.',
    defaultSnippets: z
        .array(z.object({ label: z.string(), body: baseResponseRuleSchema }))
        .parse(ResponseRuleSchema.meta()?.['defaultSnippets'])
})

// GoWave uses header objects; all other fields follow the pinned upstream contract.
export const ResponseRulesConfigSchema = UpstreamResponseRulesConfigSchema.extend({
    rules: z.array(responseRuleSchema)
})

export const SubscriptionSettingsResponseSchema =
    GetSubscriptionSettingsCommand.ResponseSchema.extend({
        response: GetSubscriptionSettingsCommand.ResponseSchema.shape.response.extend({
            responseRules: ResponseRulesConfigSchema.nullable()
        })
    })

export const UpdateSubscriptionSettingsBodySchema =
    UpdateSubscriptionSettingsCommand.RequestBodySchema.extend({
        responseRules: ResponseRulesConfigSchema.optional()
    })

export const TestSrrMatcherResponseSchema = TestSrrMatcherCommand.ResponseSchema.extend({
    response: TestSrrMatcherCommand.ResponseSchema.shape.response.extend({
        matchedRule: responseRuleSchema.nullable()
    })
})

export type SubscriptionSettings = z.infer<typeof SubscriptionSettingsResponseSchema>['response']
export type ResponseRules = z.infer<typeof ResponseRulesConfigSchema> | null
