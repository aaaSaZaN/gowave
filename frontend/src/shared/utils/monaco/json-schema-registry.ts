import type { jsonDefaults } from '@shared/utils/setup-monaco/setup-monaco'

type DiagnosticsOptions = Parameters<typeof jsonDefaults.setDiagnosticsOptions>[0]
type JsonSchema = NonNullable<DiagnosticsOptions['schemas']>[number]
type SeverityOptions = Omit<DiagnosticsOptions, 'schemas'>

const DEFAULT_DIAGNOSTICS: SeverityOptions = {
    allowComments: false,
    enableSchemaRequest: true,
    schemaRequest: 'warning',
    validate: true
}

const registry = new Map<string, JsonSchema>()

export const registerJsonSchema = (schema: JsonSchema, diagnostics?: SeverityOptions) => {
    registry.set(schema.uri, schema)

    // Callers run once Monaco is mounted, so this resolves the already loaded module
    // without pulling Monaco into the main bundle.
    import('@shared/utils/setup-monaco/setup-monaco').then(({ jsonDefaults: defaults }) => {
        defaults.setDiagnosticsOptions({
            ...DEFAULT_DIAGNOSTICS,
            ...diagnostics,
            schemas: [...registry.values()]
        })
    })
}
