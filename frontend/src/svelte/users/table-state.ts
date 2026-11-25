import {
    USERS_TABLE_DEFAULTS,
    USERS_TABLE_STORAGE_KEY,
    USERS_TABLE_VERSION
} from '@entities/dashboard/users/users-table-store/defaults'
import {
    USERS_TEMPLATES_STORAGE_KEY,
    USERS_TEMPLATES_VERSION
} from '@entities/dashboard/users/users-table-templates-store/storage-key'
import { GetUsersCommand } from '@remnawave/backend-contract'
import { z } from 'zod'

const columnFilter = z.object({ id: z.string(), value: z.unknown() })
const sorting = z.object({ id: z.string(), desc: z.boolean() })
const visibility = z.record(z.string(), z.boolean())
const columnOrder = z.array(z.string())
const columnPinning = z.object({ left: columnOrder.default([]), right: columnOrder.default([]) })

const tableSchema = z.object({
    columnFilter: z.array(columnFilter).catch([]),
    columnOrder: columnOrder.catch([]),
    columnPinning: columnPinning.catch({ left: [], right: [] }),
    columnSize: z.record(z.string(), z.number().positive()).catch(USERS_TABLE_DEFAULTS.columnSize),
    columnVisibility: visibility.catch(USERS_TABLE_DEFAULTS.columnVisibility),
    paginationState: z
        .object({
            pageIndex: z.int().nonnegative(),
            pageSize: z.int().positive()
        })
        .catch({ pageIndex: 0, pageSize: 25 }),
    showColumnFilters: z.boolean().catch(true),
    sorting: z.array(sorting).catch(USERS_TABLE_DEFAULTS.sorting)
})

const templateSchema = z.object({
    id: z.string(),
    name: z.string(),
    color: z.string(),
    snapshot: z.object({
        columnFilterFns: z.record(z.string(), z.string()).catch({}),
        columnFilters: z.array(columnFilter),
        columnOrder,
        columnPinning,
        columnVisibility: visibility,
        showColumnFilters: z.boolean(),
        sorting: z.array(sorting)
    })
})

type ReadStorage = Pick<Storage, 'getItem'>
type WriteStorage = Pick<Storage, 'setItem'>
export type UserTableState = z.infer<typeof tableSchema>
export type UserTableTemplate = z.infer<typeof templateSchema>
export const tableDensities = ['xxs', 'xl', 'md', 'xs'] as const
export type TableDensity = (typeof tableDensities)[number]

function read(storage: ReadStorage | undefined, key: string, version: number): unknown {
    try {
        const raw: unknown = JSON.parse(storage?.getItem(key) ?? 'null')
        const result = z
            .object({ state: z.unknown(), version: z.number().optional() })
            .safeParse(raw)
        if (
            result.success &&
            (result.data.version === undefined || result.data.version === version)
        ) {
            return result.data.state
        }
    } catch {
        // The dashboard can operate without browser storage.
    }
    return undefined
}

export function readUserTable(storage?: ReadStorage): UserTableState {
    const result = tableSchema.safeParse(
        read(storage, USERS_TABLE_STORAGE_KEY, USERS_TABLE_VERSION)
    )
    return result.success ? result.data : tableSchema.parse({})
}

export function writeUserTable(storage: WriteStorage, state: UserTableState): void {
    storage.setItem(
        USERS_TABLE_STORAGE_KEY,
        JSON.stringify({ state, version: USERS_TABLE_VERSION })
    )
}

export function readUserTemplates(storage?: ReadStorage): UserTableTemplate[] {
    const value = read(storage, USERS_TEMPLATES_STORAGE_KEY, USERS_TEMPLATES_VERSION)
    const result = z.object({ templates: z.array(z.unknown()) }).safeParse(value)
    if (!result.success) return []
    return result.data.templates.flatMap((item) => {
        const template = templateSchema.safeParse(item)
        return template.success ? [template.data] : []
    })
}

export function writeUserTemplates(storage: WriteStorage, templates: UserTableTemplate[]): void {
    storage.setItem(
        USERS_TEMPLATES_STORAGE_KEY,
        JSON.stringify({
            state: { templates },
            version: USERS_TEMPLATES_VERSION
        })
    )
}

export function captureUserTable(
    state: UserTableState,
    columnFilterFns: Record<string, string>
): UserTableTemplate['snapshot'] {
    return templateSchema.shape.snapshot.parse({
        ...state,
        columnFilters: state.columnFilter,
        columnFilterFns
    })
}

export function applyUserTable(
    state: UserTableState,
    snapshot: UserTableTemplate['snapshot']
): UserTableState {
    const { columnFilters, columnFilterFns: _modes, ...layout } = snapshot
    return {
        ...state,
        ...layout,
        columnFilter: columnFilters,
        paginationState: { ...state.paginationState, pageIndex: 0 }
    }
}

function userTableFilters(filters: UserTableState['columnFilter']) {
    function present(value: unknown): boolean {
        return value !== undefined && value !== null && value !== ''
    }
    return filters.filter(({ value }) =>
        Array.isArray(value) ? value.some(present) : present(value)
    )
}

export function usersRequest(state: UserTableState, modes: Record<string, string>) {
    const { pageIndex, pageSize } = state.paginationState
    return GetUsersCommand.RequestQuerySchema.parse({
        start: pageIndex * pageSize,
        size: pageSize,
        filters: userTableFilters(state.columnFilter),
        sorting: state.sorting,
        filterModes: modes
    })
}
