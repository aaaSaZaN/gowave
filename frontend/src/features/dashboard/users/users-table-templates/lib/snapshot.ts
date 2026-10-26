import type { UsersTableTemplateSnapshot } from '@entities/dashboard/users/users-table-templates-store'
import type { MRT_TableInstance } from '@kastov/mantine-react-table-open'
import type { GetUsersCommand } from '@remnawave/backend-contract'

type UserRow = GetUsersCommand.Response['response']['users'][number]
type UsersTable = MRT_TableInstance<UserRow>

export const captureSnapshot = (table: UsersTable): UsersTableTemplateSnapshot => {
    const state = table.getState()

    return {
        columnFilterFns: { ...state.columnFilterFns },
        columnFilters: [...state.columnFilters],
        columnOrder: [...state.columnOrder],
        columnPinning: {
            left: [...(state.columnPinning.left ?? [])],
            right: [...(state.columnPinning.right ?? [])]
        },
        columnVisibility: { ...state.columnVisibility },
        showColumnFilters: state.showColumnFilters,
        sorting: [...state.sorting]
    }
}

export const applySnapshot = (table: UsersTable, snapshot: UsersTableTemplateSnapshot) => {
    table.setColumnVisibility(snapshot.columnVisibility)
    table.setColumnOrder(snapshot.columnOrder)
    table.setColumnPinning(snapshot.columnPinning)
    table.setColumnFilterFns(snapshot.columnFilterFns)
    table.setColumnFilters(snapshot.columnFilters)
    table.setSorting(snapshot.sorting)
    table.setPageIndex(0)
}
