import { create } from 'zustand'
import { createJSONStorage, devtools, persist } from 'zustand/middleware'

import {
    HOSTS_VIEW_MODE,
    type IActions,
    type IExperimentalFeatures,
    type IState,
    NODES_VIEW_MODE
} from './interfaces'

const LEGACY_LAYOUT_STYLE = 'sidebar'

const initialExperimental: IExperimentalFeatures = {
    legacyLayoutStyle: false
}

const initialState: IState = {
    experimental: initialExperimental,
    nodesViewMode: NODES_VIEW_MODE.CARDS,
    nodesActiveTag: null,
    hostsViewMode: HOSTS_VIEW_MODE.CARDS,
    hostsActiveTag: null,
    sectionActiveTags: {}
}

type PersistedState = IState & { layoutStyle?: string }

const migrateState = (persistedState: unknown, version: number): IState => {
    const { layoutStyle, ...state } = (persistedState ?? {}) as Partial<PersistedState>

    if (version >= 2) {
        return { ...initialState, ...state }
    }

    return {
        ...initialState,
        ...state,
        experimental: {
            legacyLayoutStyle: layoutStyle === LEGACY_LAYOUT_STYLE
        }
    }
}

export const useViewPreferencesStore = create<IActions & IState>()(
    persist(
        devtools(
            (set) => ({
                ...initialState,
                actions: {
                    setNodesViewMode: (mode) => set({ nodesViewMode: mode }),
                    setSectionActiveTag: (section, tag) =>
                        set((state) => ({
                            sectionActiveTags: { ...state.sectionActiveTags, [section]: tag }
                        })),
                    setNodesActiveTag: (tag) => set({ nodesActiveTag: tag }),
                    setHostsViewMode: (mode) => set({ hostsViewMode: mode }),
                    setHostsActiveTag: (tag) => set({ hostsActiveTag: tag }),
                    setExperimentalFeature: (feature, enabled) =>
                        set((state) => ({
                            experimental: { ...state.experimental, [feature]: enabled }
                        })),
                    resetState: () => set({ ...initialState })
                }
            }),
            { name: 'viewPreferencesStore', anonymousActionType: 'viewPreferencesStore' }
        ),
        {
            name: 'viewPreferencesStore',
            version: 2,
            storage: createJSONStorage(() => localStorage),
            partialize: (state) => ({
                experimental: state.experimental,
                nodesViewMode: state.nodesViewMode,
                nodesActiveTag: state.nodesActiveTag,
                hostsViewMode: state.hostsViewMode,
                hostsActiveTag: state.hostsActiveTag,
                sectionActiveTags: state.sectionActiveTags
            }),
            migrate: migrateState,
            merge: (persistedState, currentState) => {
                const {
                    experimental,
                    nodesViewMode = currentState.nodesViewMode,
                    nodesActiveTag = currentState.nodesActiveTag,
                    hostsViewMode = currentState.hostsViewMode,
                    hostsActiveTag = currentState.hostsActiveTag,
                    sectionActiveTags = currentState.sectionActiveTags
                } = (persistedState ?? {}) as Partial<IState>

                return {
                    ...currentState,
                    experimental: {
                        legacyLayoutStyle:
                            experimental?.legacyLayoutStyle ??
                            currentState.experimental.legacyLayoutStyle
                    },
                    nodesViewMode,
                    nodesActiveTag,
                    hostsViewMode,
                    hostsActiveTag,
                    sectionActiveTags
                }
            }
        }
    )
)

export const useNodesViewMode = () => useViewPreferencesStore((state) => state.nodesViewMode)
export const useNodesActiveTag = () => useViewPreferencesStore((state) => state.nodesActiveTag)
export const useViewPreferencesStoreActions = () =>
    useViewPreferencesStore((state) => state.actions)
export const useHostsViewMode = () => useViewPreferencesStore((state) => state.hostsViewMode)
export const useHostsActiveTag = () => useViewPreferencesStore((state) => state.hostsActiveTag)

export const useSectionActiveTag = (section: string) =>
    useViewPreferencesStore((state) => state.sectionActiveTags[section] ?? null)
export const useExperimentalFeature = (feature: keyof IExperimentalFeatures) =>
    useViewPreferencesStore((state) => state.experimental[feature])
