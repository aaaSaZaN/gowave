import { z } from 'zod'

const storageKey = 'viewPreferencesStore'
const schema = z.object({
    state: z.object({
        layoutStyle: z.string().optional().catch(undefined),
        hostsViewMode: z.enum(['cards', 'table']).optional().catch(undefined),
        hostsActiveTag: z.string().nullable().optional().catch(undefined),
        nodesViewMode: z.enum(['cards', 'table']).optional().catch(undefined),
        nodesActiveTag: z.string().nullable().optional().catch(undefined),
        experimental: z
            .object({
                legacyLayoutStyle: z.boolean().optional().catch(undefined)
            })
            .optional()
            .catch(undefined),
        sectionActiveTags: z.record(z.string(), z.string().nullable()).optional().catch(undefined)
    }),
    version: z.int().optional()
})
type StoredViews = z.infer<typeof schema>
export type EntityViewMode = NonNullable<StoredViews['state']['hostsViewMode']>

function readViews(storage?: Pick<Storage, 'getItem'>): StoredViews {
    try {
        const item = (storage ?? window.localStorage).getItem(storageKey)
        const result = schema.safeParse(item ? JSON.parse(item) : {})
        if (result.success) return result.data
    } catch {
        // The view still works if storage is unavailable.
    }
    return { state: {}, version: 2 }
}

export function readHostView(storage?: Pick<Storage, 'getItem'>) {
    const { state } = readViews(storage)
    return { mode: state.hostsViewMode ?? 'cards', tag: state.hostsActiveTag ?? '' }
}

export function writeHostView(
    mode: EntityViewMode,
    tag: string,
    storage?: Pick<Storage, 'getItem' | 'setItem'>
): void {
    writeViews({ hostsViewMode: mode, hostsActiveTag: tag || null }, storage)
}

export function readNodeView() {
    const { state } = readViews()
    return { mode: state.nodesViewMode ?? 'cards', tag: state.nodesActiveTag ?? '' }
}

export function writeNodeView(mode: EntityViewMode, tag: string): void {
    writeViews({ nodesViewMode: mode, nodesActiveTag: tag || null })
}

export function readSectionTag(section: string): string {
    return readViews().state.sectionActiveTags?.[section] ?? ''
}

export function writeSectionTag(section: string, tag: string): void {
    const { sectionActiveTags } = readViews().state
    writeViews({ sectionActiveTags: { ...sectionActiveTags, [section]: tag || null } })
}

export function readLegacyLayout(storage?: Pick<Storage, 'getItem'>): boolean {
    const { state, version } = readViews(storage)
    return version !== undefined && version >= 2
        ? (state.experimental?.legacyLayoutStyle ?? false)
        : state['layoutStyle'] === 'sidebar'
}

export function writeLegacyLayout(
    enabled: boolean,
    storage?: Pick<Storage, 'getItem' | 'setItem'>
): void {
    const { state } = readViews(storage)
    writeViews({ experimental: { ...state.experimental, legacyLayoutStyle: enabled } }, storage)
}

function writeViews(
    patch: Partial<StoredViews['state']>,
    storage?: Pick<Storage, 'getItem' | 'setItem'>
): void {
    const { state, version } = readViews(storage)

    if (version !== undefined && version > 2) return
    if (version !== 2) {
        state.experimental = {
            ...state.experimental,
            legacyLayoutStyle: state['layoutStyle'] === 'sidebar'
        }
        delete state['layoutStyle']
    }
    try {
        const target = storage ?? window.localStorage
        target.setItem(storageKey, JSON.stringify({ state: { ...state, ...patch }, version: 2 }))
    } catch {
        // The current page still retains the preference.
    }
}
