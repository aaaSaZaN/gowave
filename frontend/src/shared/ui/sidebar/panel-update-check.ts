import { instance } from '@shared/api'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { isAxiosError } from 'axios'
import { useCallback, useState } from 'react'

export type UpdateChannel = 'dev' | 'stable'

export interface IPanelUpdateCheck {
    canAutoUpdate: boolean
    changelog: string
    channel: UpdateChannel
    checkedAt?: string
    currentVersion: string
    hasUpdate: boolean
    isDocker: boolean
    latestVersion: string
    targetChannel: UpdateChannel
    updateCommand?: string
}

export interface IPanelUpdateLogLine {
    level: 'error' | 'info'
    message: string
    time: string
}

export type PanelUpdateState = 'done' | 'failed' | 'running'

export interface IPanelUpdateStatus {
    channel: UpdateChannel
    downloaded?: number
    error?: string
    file?: string
    finishedAt?: string
    fromVersion: string
    installedVersion?: string
    log: IPanelUpdateLogLine[]
    message: string
    progress: number
    startedAt: string
    state: PanelUpdateState
    step: string
    total?: number
    updatedAt: string
}

export interface IPanelUpdateStatusResponse {
    channel: UpdateChannel
    currentVersion: string
    helperInstalled: boolean
    requestPending: boolean
    serverTime: string
    status: IPanelUpdateStatus | null
    statusError?: string
}

export interface IPanelUpdateRequest {
    channel: UpdateChannel
    requestedAt: string
    success: boolean
}

// While the dashboard is open the panel asks again every 10 minutes; the backend
// keeps GitHub's answer for the same time, so open tabs share one request.
const UPDATE_CHECK_INTERVAL = 10 * 60 * 1000

const updateCheckKey = (channel?: UpdateChannel) => ['panel-update-check', channel ?? 'current']

async function fetchUpdateCheck(channel: UpdateChannel | undefined, force: boolean) {
    const params: Record<string, string> = {}
    if (channel) params['channel'] = channel
    if (force) params['force'] = '1'
    const res = await instance.get<IPanelUpdateCheck>('/api/system/updates/check', { params })
    return res.data
}

export function getUpdateErrorMessage(error: unknown, fallback: string): string {
    if (isAxiosError<unknown>(error)) {
        const data = error.response?.data
        if (data && typeof data === 'object' && 'message' in data) {
            const { message } = data as { message?: unknown }
            if (typeof message === 'string' && message) return message
        }
        if (typeof data === 'string' && data.trim()) return data.trim()
        if (!error.response) return `${fallback}: the panel is not reachable`
        return `${fallback}: HTTP ${error.response.status}`
    }
    return error instanceof Error && error.message ? error.message : fallback
}

// The backend knows the build's channel (stable releases or dev builds of main)
// and caches the GitHub answer, so every dashboard can ask.
export function usePanelUpdateCheck(channel?: UpdateChannel) {
    return useQuery({
        queryKey: updateCheckKey(channel),
        queryFn: () => fetchUpdateCheck(channel, false),
        staleTime: UPDATE_CHECK_INTERVAL / 2,
        refetchInterval: UPDATE_CHECK_INTERVAL,
        refetchIntervalInBackground: false,
        refetchOnWindowFocus: true,
        retry: false
    })
}

// useForceUpdateCheck asks GitHub right away instead of waiting for the cache,
// for an admin who knows a new build was just published.
export function useForceUpdateCheck(channel?: UpdateChannel) {
    const queryClient = useQueryClient()
    const [isChecking, setIsChecking] = useState(false)
    const [error, setError] = useState<string | null>(null)

    const check = useCallback(async () => {
        setIsChecking(true)
        setError(null)
        try {
            const data = await fetchUpdateCheck(channel, true)
            queryClient.setQueryData(updateCheckKey(channel), data)
            if (!channel || channel === data.channel) {
                queryClient.setQueryData(updateCheckKey(), data)
            }
        } catch (err) {
            setError(getUpdateErrorMessage(err, 'Could not check for updates'))
        } finally {
            setIsChecking(false)
        }
    }, [channel, queryClient])

    return { check, error, isChecking }
}

export async function requestPanelUpdate(channel: UpdateChannel) {
    const res = await instance.post<IPanelUpdateRequest>('/api/system/updates/apply', { channel })
    return res.data
}

export async function fetchPanelUpdateStatus() {
    const res = await instance.get<IPanelUpdateStatusResponse>('/api/system/updates/status', {
        timeout: 5000
    })
    return res.data
}
