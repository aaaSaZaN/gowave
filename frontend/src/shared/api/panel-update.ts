import { isAxiosError } from 'axios'

import { instance } from './axios'

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

export async function fetchUpdateCheck(
    channel?: UpdateChannel,
    force = false,
    signal?: AbortSignal
) {
    const params: Record<string, string> = {}
    if (channel) params['channel'] = channel
    if (force) params['force'] = '1'
    const res = await instance.get<IPanelUpdateCheck>('/api/system/updates/check', {
        params,
        ...(signal ? { signal } : {})
    })
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

export async function requestPanelUpdate(channel: UpdateChannel) {
    const res = await instance.post<IPanelUpdateRequest>('/api/system/updates/apply', { channel })
    return res.data
}

export async function fetchPanelUpdateStatus(signal?: AbortSignal) {
    const res = await instance.get<IPanelUpdateStatusResponse>('/api/system/updates/status', {
        timeout: 5000,
        ...(signal ? { signal } : {})
    })
    return res.data
}
