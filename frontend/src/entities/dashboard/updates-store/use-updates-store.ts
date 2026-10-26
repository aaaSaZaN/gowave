import { sToMs } from '@shared/utils/time-utils'
import axios from 'axios'
import { create } from 'zustand'
import { createJSONStorage, devtools, persist } from 'zustand/middleware'

const CACHE_TIME = sToMs(24 * 60 * 60)

export interface IGowaveInfo {
    latestVersion: string
    starsCount: number
}

interface IState {
    isLoading: boolean
    lastUpdateTimestamp: number
    gowaveInfo: IGowaveInfo
}

interface IActions {
    actions: {
        getGowaveInfo: () => Promise<void>
        resetState: () => void
        setGowaveInfo: (info: IGowaveInfo) => void
    }
}

const initialState: IState = {
    isLoading: false,
    lastUpdateTimestamp: 0,
    gowaveInfo: {
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
                    getGowaveInfo: async () => {
                        const { lastUpdateTimestamp, gowaveInfo } = get()
                        const now = Date.now()

                        if (
                            lastUpdateTimestamp &&
                            now - lastUpdateTimestamp < CACHE_TIME &&
                            gowaveInfo.latestVersion &&
                            gowaveInfo.starsCount > 0
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
                                gowaveInfo: {
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

                    setGowaveInfo: (info: IGowaveInfo) => {
                        set({ gowaveInfo: info, lastUpdateTimestamp: Date.now() })
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
            version: 3,
            migrate: () => ({ ...initialState }),
            partialize: (state) => ({
                lastUpdateTimestamp: state.lastUpdateTimestamp,
                gowaveInfo: state.gowaveInfo
            })
        }
    )
)

export const useGowaveInfo = () => useUpdatesStore((state) => state.gowaveInfo)
export const useLastUpdateTimestamp = () => useUpdatesStore((state) => state.lastUpdateTimestamp)
export const useIsLoadingGowaveUpdates = () => useUpdatesStore((state) => state.isLoading)
export const useUpdatesStoreActions = () => useUpdatesStore((state) => state.actions)
