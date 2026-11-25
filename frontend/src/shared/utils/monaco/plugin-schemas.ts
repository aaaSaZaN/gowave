import type { GetSharedListsCommand } from '@remnawave/backend-contract'
import { NodePluginEditorSchema, SharedListConfigSchema } from '@remnawave/node-plugins'
import consola from '@shared/lib/logger'

import { registerJsonSchema } from './json-schema-registry'

type TSharedLists = GetSharedListsCommand.Response['response']['sharedLists']

const buildSharedListDescription = (sharedList: TSharedLists[number]): string => {
    const { type, itemsCount } = sharedList

    return `**${type}** · ${itemsCount} item${itemsCount === 1 ? '' : 's'}`
}

const injectSharedListNames = (node: unknown, sharedLists: TSharedLists): void => {
    if (!node || typeof node !== 'object') return

    if (Array.isArray(node)) {
        node.forEach((item) => {
            injectSharedListNames(item, sharedLists)
        })
        return
    }

    const schemaNode = node as Record<string, unknown>

    if (
        schemaNode['type'] === 'string' &&
        String(schemaNode['pattern'] ?? '').startsWith('^ext:')
    ) {
        delete schemaNode['pattern']

        schemaNode['enum'] = sharedLists.map((sharedList) => `ext:${sharedList.name}`)
        schemaNode['markdownEnumDescriptions'] = sharedLists.map(buildSharedListDescription)
        schemaNode['title'] = 'Shared List'
        schemaNode['markdownDescription'] =
            sharedLists.length > 0
                ? 'Reference to a shared list. Manage lists in **Shared Lists**.'
                : 'No shared lists created yet. Create one in **Shared Lists** first.'
        return
    }

    Object.values(schemaNode).forEach((value) => {
        injectSharedListNames(value, sharedLists)
    })
}

export function setupNodePluginSchema(sharedLists: TSharedLists = []): void {
    try {
        const schema = NodePluginEditorSchema.toJSONSchema()
        if (schema && typeof schema === 'object') {
            schema.additionalProperties = true
            if (schema.properties) {
                schema.properties['sharedLists'] = {
                    type: 'array',
                    items: { type: 'object' },
                    description: 'Optional shared lists definition'
                }
            }
        }

        injectSharedListNames(schema, sharedLists)

        registerJsonSchema(
            {
                fileMatch: ['node-plugin://*'],
                schema,
                uri: 'https://node-plugin-schema.json'
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

export function setupSharedListSchema(): void {
    try {
        const schema = SharedListConfigSchema.toJSONSchema()
        registerJsonSchema(
            {
                fileMatch: ['shared-list://*'],
                schema,
                uri: 'https://shared-list-schema.json'
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
