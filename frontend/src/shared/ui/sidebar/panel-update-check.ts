import { instance } from '@shared/api'
import { useQuery } from '@tanstack/react-query'

export type UpdateChannel = 'dev' | 'stable'

export interface IPanelUpdateCheck {
    canAutoUpdate: boolean
    changelog: string
    channel: UpdateChannel
    currentVersion: string
    hasUpdate: boolean
    isDocker: boolean
    latestVersion: string
    targetChannel: UpdateChannel
    updateCommand?: string
}

// The backend knows the build's channel (stable releases or dev builds of main)
// and caches the GitHub answer, so every dashboard can ask.
export function usePanelUpdateCheck(channel?: UpdateChannel) {
    return useQuery({
        queryKey: ['panel-update-check', channel ?? 'current'],
        queryFn: async () => {
            const res = await instance.get<IPanelUpdateCheck>('/api/system/updates/check', {
                params: channel ? { channel } : undefined
            })
            return res.data
        },
        staleTime: 30 * 60 * 1000,
        refetchOnWindowFocus: false,
        retry: false
    })
}
