import type { ConnectionsByNodeResultCommand, GetNodesCommand } from '@remnawave/backend-contract'

type Result = NonNullable<ConnectionsByNodeResultCommand.Response['response']['result']>
export type SessionIp = Result['users'][number]['ips'][number]
export type SessionNode = Pick<
    GetNodesCommand.Response['response'][number],
    'countryCode' | 'name' | 'uuid'
>
type NodeResult = { node: SessionNode; users: Result['users'] }

type UserSessions = {
    userId: number
    totalIps: number
    nodes: Map<SessionNode, SessionIp[]>
    unique: Set<string>
}
export type ScanProgress = {
    active: SessionNode[]
    completed: number
    failed: number
    total: number
}
const maxConcurrent = 10

export function sessionActivity(lastSeen: Date, now: number): 'recent' | 'idle' | 'stale' {
    const minutes = (now - lastSeen.getTime()) / 60_000
    if (minutes <= 5) return 'recent'
    return minutes <= 60 ? 'idle' : 'stale'
}

export function sessionActivityCounts(ips: SessionIp[], now: number) {
    const counts = { recent: 0, idle: 0, stale: 0 }
    for (const { lastSeen } of ips) counts[sessionActivity(lastSeen, now)]++
    return counts
}

export function aggregateSessions(results: NodeResult[]) {
    const users = new Map<number, UserSessions>()
    const globalIps = new Set<string>()
    let totalIps = 0

    for (const { node, users: sessions } of results) {
        for (const { userId, ips } of sessions) {
            const user = users.get(userId) ?? {
                userId,
                nodes: new Map<SessionNode, SessionIp[]>(),
                totalIps: 0,
                unique: new Set<string>()
            }
            const entries = user.nodes.get(node) ?? []
            for (const entry of ips) {
                entries.push(entry)
                globalIps.add(entry.ip)
                user.unique.add(entry.ip)
            }
            user.nodes.set(node, entries)
            user.totalIps += ips.length
            users.set(userId, user)
            totalIps += ips.length
        }
    }

    const ranked = [...users.values()]
        .map(({ userId, nodes, totalIps, unique }) => ({
            userId,
            nodes: [...nodes].map(([node, ips]) => ({ node, ips })),
            totalIps,
            uniqueIps: unique.size
        }))
        .sort((a, b) => b.uniqueIps - a.uniqueIps || a.userId - b.userId)

    return { users: ranked, totalIps, uniqueIps: globalIps.size }
}

export async function scanSessions(
    nodes: SessionNode[],
    load: (node: SessionNode, signal: AbortSignal) => Promise<Result['users']>,
    report: (progress: ScanProgress) => void,
    signal: AbortSignal
) {
    const active = new Set<SessionNode>()
    const results: (NodeResult | undefined)[] = Array.from({ length: nodes.length })
    const errors = new Map<SessionNode, string>()
    let next = 0
    let completed = 0

    function reportProgress(): void {
        if (!signal.aborted) {
            report({
                active: [...active],
                completed,
                failed: errors.size,
                total: nodes.length
            })
        }
    }

    async function worker(): Promise<void> {
        while (!signal.aborted) {
            const index = next++
            const node = nodes[index]
            if (!node) return
            active.add(node)
            reportProgress()
            try {
                const users = await load(node, signal)
                results[index] = { node, users }
                completed++
            } catch (error) {
                if (!signal.aborted) {
                    errors.set(node, error instanceof Error ? error.message : String(error))
                }
            } finally {
                active.delete(node)
                reportProgress()
            }
        }
    }

    await Promise.all(Array.from({ length: Math.min(maxConcurrent, nodes.length) }, () => worker()))
    signal.throwIfAborted()
    return {
        ...aggregateSessions(results.filter((result) => result !== undefined)),
        completed,
        errors
    }
}
