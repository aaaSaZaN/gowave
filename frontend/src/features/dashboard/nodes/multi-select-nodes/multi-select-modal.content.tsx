import { Stack } from '@mantine/core'
import { modals } from '@mantine/modals'
import {
    type GetNodesCommand,
    NODES_BULK_ACTIONS,
    type TNodesBulkActions
} from '@remnawave/backend-contract'
import { QueryKeys, useBulkNodesActions } from '@shared/api/hooks'
import { queryClient } from '@shared/api/query-client'
import { runAsync } from '@shared/lib/run-async'
import { ActionCardShared } from '@shared/ui'
import { useTranslation } from 'react-i18next'
import { PiPulse } from 'react-icons/pi'
import { TbCancel, TbRefresh, TbRocket } from 'react-icons/tb'

type NodeType = GetNodesCommand.Response['response'][number]

interface IProps {
    selectedRecords: NodeType[]
    setSelectedRecords: (records: NodeType[]) => void
}

export const MultiSelectNodesModalContent = (props: IProps) => {
    const { selectedRecords, setSelectedRecords } = props
    const { t } = useTranslation()
    const { mutateAsync: bulkAction, isPending } = useBulkNodesActions()

    const uuids = selectedRecords.map((node) => node.uuid)

    const handleAction = async (action: TNodesBulkActions) => {
        if (isPending || uuids.length === 0) return
        await bulkAction({ variables: { uuids, action } })

        runAsync(queryClient.refetchQueries({ queryKey: QueryKeys.nodes.getAllNodes.queryKey }))
        modals.closeAll()
        setSelectedRecords([])
    }

    return (
        <Stack gap="xs">
            <ActionCardShared
                description={`${uuids.length} node(s)`}
                icon={<TbRocket size={20} />}
                iconColor="teal"
                isLoading={isPending}
                onClick={() => handleAction(NODES_BULK_ACTIONS.RESTART)}
                title={t('restart-node-button.feature.restart')}
                variant="soft"
            />
            <ActionCardShared
                description={`${uuids.length} node(s)`}
                icon={<TbCancel size={20} />}
                iconColor="orange"
                isLoading={isPending}
                onClick={() => handleAction(NODES_BULK_ACTIONS.DISABLE)}
                title={t('common.action.disable')}
                variant="soft"
            />
            <ActionCardShared
                description={`${uuids.length} node(s)`}
                icon={<PiPulse size={20} />}
                iconColor="cyan"
                isLoading={isPending}
                onClick={() => handleAction(NODES_BULK_ACTIONS.ENABLE)}
                title={t('common.action.enable')}
                variant="soft"
            />
            <ActionCardShared
                description={`${uuids.length} node(s)`}
                icon={<TbRefresh size={20} />}
                iconColor="violet"
                isLoading={isPending}
                onClick={() => handleAction(NODES_BULK_ACTIONS.RESET_TRAFFIC)}
                title={t('common.action.reset-traffic')}
                variant="soft"
            />
        </Stack>
    )
}
