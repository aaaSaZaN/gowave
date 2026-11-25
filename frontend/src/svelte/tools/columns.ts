export type Column<Row> = (
    | { key: keyof Row & string }
    | { key: string; value: (row: Row) => unknown }
) & {
    label: string
    filter?: 'number' | 'text' | 'select'
    options?: { value: string; label: string; flag?: string }[]
    filterMode?: 'contains' | 'equals' | 'choose'
    sortable?: boolean
    width?: number
    align?: 'center'
    prefix?: (row: Row) => string
    monospace?: boolean
}

export type InspectorRow = { userId: number }
