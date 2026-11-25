import type {
    IPanelUpdateStatus,
    IPanelUpdateStatusResponse,
    UpdateChannel
} from '@shared/api/panel-update'

export interface PanelUpdateRequest {
    channel: UpdateChannel
    fromVersion: string
    // Server time of the request; statuses that started earlier belong to older updates.
    requestedAt: string
}

export const POLL_INTERVAL = 1500
export const HELPER_START_TIMEOUT = 30 * 1000
const LEGACY_GRACE = 15 * 1000
const UNREACHABLE_TIMEOUT = 3 * 60 * 1000
export const UPDATE_TIMEOUT = 16 * 60 * 1000
export const RELOAD_DELAY = 3000

export const JOURNAL_COMMAND = 'journalctl -u gowave-update.service -n 100 --no-pager'
export const HELPER_COMMAND = 'systemctl status gowave-update.path gowave-update.service'
const PANEL_COMMAND = 'journalctl -u gowave.service -n 100 --no-pager'

export const STEP_LABELS: Record<string, string> = {
    starting: 'Starting',
    download: 'Downloading',
    verify: 'Verifying checksums',
    frontend: 'Installing frontend',
    binary: 'Installing backend',
    service: 'Updating service',
    restart: 'Restarting panel',
    done: 'Done'
}

export interface PollState {
    lastError: null | string
    now: number
    response: IPanelUpdateStatusResponse | null
    unreachableSince: null | number
}

export type UpdateView =
    | { command?: string; error: string; kind: 'failed' }
    | { kind: 'done'; version: string }
    | { kind: 'helperIdle' }
    | { kind: 'legacy' }
    | { kind: 'restarting' }
    | { kind: 'running'; status: IPanelUpdateStatus }
    | { kind: 'waiting' }

export function belongsToRequest(
    status: IPanelUpdateStatus | null | undefined,
    request: PanelUpdateRequest
): status is IPanelUpdateStatus {
    if (!status || status.channel !== request.channel) return false
    // The root helper writes the status only after it picked up the request.
    return Date.parse(status.startedAt) >= Date.parse(request.requestedAt) - 2000
}

export function deriveUpdateView(
    request: PanelUpdateRequest,
    poll: PollState,
    startedLocally: number
): UpdateView {
    const { response } = poll
    const own = belongsToRequest(response?.status, request) ? response?.status : undefined
    const elapsed = poll.now - startedLocally

    if (own?.state === 'failed') {
        return {
            kind: 'failed',
            error: own.error || own.message || 'The update failed',
            command: JOURNAL_COMMAND
        }
    }
    if (own?.state === 'done') {
        return { kind: 'done', version: own.installedVersion || response?.currentVersion || '' }
    }
    if (response?.channel === request.channel && response.currentVersion !== request.fromVersion) {
        return { kind: 'done', version: response.currentVersion }
    }
    if (poll.unreachableSince !== null) {
        if (poll.now - poll.unreachableSince > UNREACHABLE_TIMEOUT) {
            return {
                kind: 'failed',
                error: `The panel has not answered for 3 minutes (${poll.lastError ?? 'no response'}). It may have failed to start after the update.`,
                command: PANEL_COMMAND
            }
        }
        return { kind: 'restarting' }
    }
    if (elapsed > UPDATE_TIMEOUT) {
        return {
            kind: 'failed',
            error: 'The update did not finish in time and the panel version did not change.',
            command: JOURNAL_COMMAND
        }
    }
    if (own) return { kind: 'running', status: own }
    if (!response || response.requestPending) {
        return elapsed > HELPER_START_TIMEOUT ? { kind: 'helperIdle' } : { kind: 'waiting' }
    }
    // The request was taken, but the installed helper predates progress reports.
    return elapsed > LEGACY_GRACE ? { kind: 'legacy' } : { kind: 'waiting' }
}

export function isFinished(view: UpdateView) {
    return view.kind === 'done' || view.kind === 'failed'
}
