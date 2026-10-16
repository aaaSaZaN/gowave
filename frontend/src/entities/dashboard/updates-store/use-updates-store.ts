import axios from 'axios'
import { create } from 'zustand'
import { createJSONStorage, devtools, persist } from 'zustand/middleware'

import { sToMs } from '@shared/utils/time-utils'

const CACHE_TIME = sToMs(24 * 60 * 60)

export interface IRemnawaveInfo {
    latestVersion: string
    starsCount: number
}

interface IState {
    isLoading: boolean
    lastUpdateTimestamp: number
    remnawaveInfo: IRemnawaveInfo
}

interface IActions {
    actions: {
        getRemnawaveInfo: () => Promise<void>
        resetState: () => void
        setRemnawaveInfo: (info: IRemnawaveInfo) => void
    }
}

const initialState: IState = {
    isLoading: false,
    lastUpdateTimestamp: 0,
    remnawaveInfo: {
        latestVersion: '',
        starsCount: 0
    }
}

export const useUpdatesStore = create<IActions & IState>()(
    persist(
        devtools(
            (set, get) => ({
                ...initialState,
                actions: {
                    getRemnawaveInfo: async () => {
                        const { lastUpdateTimestamp, remnawaveInfo } = get()
                        const now = Date.now()

                        if (
                            lastUpdateTimestamp &&
                            now - lastUpdateTimestamp < CACHE_TIME &&
                            remnawaveInfo.latestVersion &&
                            remnawaveInfo.starsCount > 0
                        ) {
                            return
                        }

                        try {
                            set({ isLoading: true })

                            const starsResponse = await axios.get<{
                                totalStars: number
                            }>('https://ungh.cc/stars/aaaSaZaN/gowave')

                            const versionResponse = await axios.get<{
                                release: {
                                    tag: string
                                }
                            }>('https://ungh.cc/repos/aaaSaZaN/gowave/releases/latest')

                            set({
                                remnawaveInfo: {
                                    latestVersion: versionResponse.data.release.tag,
                                    starsCount: starsResponse.data.totalStars
                                },
                                lastUpdateTimestamp: now
                            })
                        } catch {
                            // silent error
                        } finally {
                            set({ isLoading: false })
                        }
                    },

                    setRemnawaveInfo: (info: IRemnawaveInfo) => {
                        set({ remnawaveInfo: info, lastUpdateTimestamp: Date.now() })
                    },
                    resetState: () => {
                        set({ ...initialState })
                    }
                }
            }),
            { name: 'updatesStore', anonymousActionType: 'updatesStore' }
        ),
        {
            name: 'updatesStore',
            storage: createJSONStorage(() => localStorage),
            version: 2,
            migrate: () => ({ ...initialState }),
            partialize: (state) => ({
                lastUpdateTimestamp: state.lastUpdateTimestamp,
                remnawaveInfo: state.remnawaveInfo
            })
        }
    )
)

export const useRemnawaveInfo = () => useUpdatesStore((state) => state.remnawaveInfo)
export const useLastUpdateTimestamp = () => useUpdatesStore((state) => state.lastUpdateTimestamp)
export const useIsLoadingRemnawaveUpdates = () => useUpdatesStore((state) => state.isLoading)
export const useUpdatesStoreActions = () => useUpdatesStore((state) => state.actions)
