import { z } from 'zod'

const sorting = z.object({ id: z.string(), desc: z.boolean() })
const filterMode = z.enum(['contains', 'equals'])
export type FilterMode = z.output<typeof filterMode>
const stateSchema = z.object({
    columnFilter: z.array(z.object({ id: z.string(), value: z.unknown() })).catch([]),
    columnOrder: z.array(z.string()).catch([]),
    columnPinning: z
        .object({ left: z.array(z.string()), right: z.array(z.string()) })
        .catch({ left: [], right: [] }),
    columnSize: z.record(z.string(), z.number().positive()).catch({}),
    columnVisibility: z.record(z.string(), z.boolean()).catch({}),
    showColumnFilters: z.boolean().optional().catch(undefined),
    filterModes: z.record(z.string(), filterMode).optional().catch(undefined),
    compact: z.boolean().optional().catch(undefined),
    paginationState: z
        .object({ pageIndex: z.int().nonnegative(), pageSize: z.int().min(1).max(1000) })
        .catch({ pageIndex: 0, pageSize: 25 }),
    sorting: z.array(sorting).optional().catch(undefined)
})

export type InspectorTableState = Omit<z.output<typeof stateSchema>, 'sorting'> & {
    sorting: z.output<typeof sorting>[]
}
export type InspectorStorage = {
    name: string
    version: number
    defaultSort: string
    showFilters?: boolean
}

export const hwidStorage: InspectorStorage = {
    name: 'x-rmnw-hwid-inspector-table',
    version: 2,
    defaultSort: 'createdAt',
    showFilters: true
}
export const historyStorage: InspectorStorage = {
    name: 'x-rmnw-srh-inspector-table',
    version: 3,
    defaultSort: 'id',
    showFilters: true
}

export const torrentStorage: InspectorStorage = {
    name: 'x-rmnw-tb-reports-table',
    version: 3,
    defaultSort: 'createdAt',
    showFilters: true
}

export function readInspectorState(
    settings: InspectorStorage,
    storage?: Pick<Storage, 'getItem'>
): InspectorTableState {
    let data: unknown
    try {
        const envelope = z
            .object({ version: z.number(), state: z.unknown() })
            .safeParse(
                JSON.parse((storage ?? window.localStorage).getItem(settings.name) ?? 'null')
            )
        if (envelope.success && envelope.data.version === settings.version)
            data = envelope.data.state
    } catch {
        // The table still works when storage is unavailable.
    }
    const parsed = stateSchema.safeParse(data)
    const state = parsed.success ? parsed.data : stateSchema.parse({})
    return {
        ...state,
        showColumnFilters: state.showColumnFilters ?? settings.showFilters ?? false,
        sorting: state.sorting ?? [{ id: settings.defaultSort, desc: true }]
    }
}

export function inspectorRequest(
    state: InspectorTableState,
    filterable: ReadonlySet<string>,
    sortable: ReadonlySet<string>,
    modes: ReadonlyMap<string, FilterMode> = new Map()
) {
    const filters = state.columnFilter.flatMap(({ id, value }) => {
        if (!filterable.has(id)) return []
        if (typeof value === 'string' && value.trim()) return [{ id, value: value.trim() }]
        if (typeof value === 'number') return [{ id, value: String(value) }]
        return []
    })
    return {
        filters,
        sorting: state.sorting.filter(({ id }) => sortable.has(id)),
        filterModes: Object.fromEntries(
            [...filterable].map((id) => [id, modes.get(id) ?? 'contains'])
        ),
        size: state.paginationState.pageSize,
        start: state.paginationState.pageIndex * state.paginationState.pageSize
    }
}
