import {
    ConnectionsByNodeCommand,
    ConnectionsByNodeResultCommand,
    ConnectionsByUserCommand,
    ConnectionsByUserResultCommand,
    DropConnectionsCommand
} from '@remnawave/backend-contract'

import { callCommand, submitVoidCommand } from '../commands'
import type { SessionNode } from './scan'

type UserJob = ConnectionsByUserResultCommand.Response['response']
export type UserSessionNode = NonNullable<UserJob['result']>['nodes'][number]
export type UserSessionProgress = UserJob['progress']

export type SessionDrop =
    | { kind: 'all' }
    | { kind: 'node'; nodeUuid: string }
    | { kind: 'ip'; nodeUuid: string; ip: string }

function waitForPoll(signal: AbortSignal): Promise<void> {
    signal.throwIfAborted()
    return new Promise((resolve, reject) => {
        const timer = setTimeout(() => {
            signal.removeEventListener('abort', abort)
            resolve()
        }, 1000)

        function abort(): void {
            clearTimeout(timer)
            reject(signal.reason)
        }

        signal.addEventListener('abort', abort, { once: true })
    })
}

async function completedSessionJob<Result extends { success: boolean }>(
    poll: () => Promise<{ isCompleted: boolean; isFailed: boolean; result: Result | null }>,
    signal: AbortSignal
): Promise<Result> {
    while (true) {
        await waitForPoll(signal)
        const response = await poll()
        if (response.isFailed || (response.isCompleted && !response.result?.success)) {
            throw new Error('The session job failed.')
        }
        if (response.isCompleted && response.result) return response.result
    }
}

export async function loadNodeSessions(node: Pick<SessionNode, 'uuid'>, cancel: AbortSignal) {
    const signal = AbortSignal.any([cancel, AbortSignal.timeout(30_000)])
    const create = ConnectionsByNodeCommand
    const { nodeUuid } = create.RequestParamSchema.parse({ nodeUuid: node.uuid })
    const { jobId } = await callCommand({ ...create, TSQ_url: create.url(nodeUuid) }, { signal })
    const poll = ConnectionsByNodeResultCommand
    const params = poll.RequestParamSchema.parse({ jobId })
    const command = { ...poll, TSQ_url: poll.url(params.jobId) }

    const result = await completedSessionJob(() => callCommand(command, { signal }), signal)
    if (result.nodeUuid !== nodeUuid) throw new Error('The session job expired.')
    return result.users
}

export async function loadUserSessions(
    userId: number,
    signal: AbortSignal,
    onProgress: (progress: UserSessionProgress) => void
) {
    const create = ConnectionsByUserCommand
    const params = create.RequestParamSchema.parse({ userId })
    const { jobId } = await callCommand(
        { ...create, TSQ_url: create.url(String(params.userId)) },
        { signal }
    )
    const poll = ConnectionsByUserResultCommand
    const job = poll.RequestParamSchema.parse({ jobId })
    const command = { ...poll, TSQ_url: poll.url(job.jobId) }
    const result = await completedSessionJob(async () => {
        const response = await callCommand(command, { signal })
        signal.throwIfAborted()
        onProgress(response.progress)
        return response
    }, signal)
    if (result.userId !== userId) throw new Error('The session job expired.')
    return result.nodes
}

export function dropUserSessions(userId: number, selection: SessionDrop) {
    return submitVoidCommand(DropConnectionsCommand, {
        dropBy:
            selection.kind === 'ip'
                ? { by: 'ipAddresses', ipAddresses: [selection.ip] }
                : { by: 'userIds', userIds: [userId] },
        targetNodes:
            selection.kind === 'all'
                ? { target: 'allNodes' }
                : { target: 'specificNodes', nodeUuids: [selection.nodeUuid] }
    })
}
