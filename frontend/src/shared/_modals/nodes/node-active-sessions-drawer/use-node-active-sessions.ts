import type { ConnectionsByNodeResultCommand } from '@remnawave/backend-contract'
import { useConnectionsByNode, useConnectionsByNodeResult } from '@shared/api/hooks'
import { useCallback, useEffect, useState } from 'react'

export type ActiveSessionUser = NonNullable<
    ConnectionsByNodeResultCommand.Response['response']['result']
>['users'][number]

export const useNodeActiveSessions = (nodeUuid: string) => {
    const [jobId, setJobId] = useState<null | string>(null)
    const [isCompleted, setIsCompleted] = useState(false)
    const [isFailed, setIsFailed] = useState(false)

    const { mutate: connectionsByNode } = useConnectionsByNode({
        route: {
            nodeUuid
        },
        mutationFns: {
            onSuccess: (data) => {
                setJobId(data.jobId)
            }
        }
    })

    const shouldPoll = !!jobId && !isCompleted && !isFailed

    const { data: connectionsByNodeResult } = useConnectionsByNodeResult({
        route: {
            jobId: jobId ?? ''
        },
        rQueryParams: {
            enabled: shouldPoll,
            refetchInterval: shouldPoll ? 1000 : false
        }
    })

    const failedResult = Boolean(
        connectionsByNodeResult &&
            (connectionsByNodeResult.isFailed ||
                (connectionsByNodeResult.isCompleted && !connectionsByNodeResult.result?.success))
    )
    const [previousResult, setPreviousResult] = useState(connectionsByNodeResult)
    if (previousResult !== connectionsByNodeResult) {
        setPreviousResult(connectionsByNodeResult)
        if (failedResult) setIsFailed(true)
    }
    useEffect(() => {
        if (!connectionsByNodeResult || failedResult) return undefined
        if (connectionsByNodeResult.isCompleted) {
            const timer = setTimeout(() => setIsCompleted(true), 500)
            return () => clearTimeout(timer)
        }
        return undefined
    }, [connectionsByNodeResult, failedResult])

    const refresh = useCallback(() => {
        setJobId(null)
        setIsCompleted(false)
        setIsFailed(false)
        connectionsByNode({})
    }, [connectionsByNode])

    useEffect(() => {
        connectionsByNode({})
    }, [connectionsByNode])

    return {
        isCompleted,
        isFailed,
        refresh,
        users: connectionsByNodeResult?.result?.users
    }
}
