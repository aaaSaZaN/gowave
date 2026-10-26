import { ActionIcon, Tooltip } from '@mantine/core'
import type { GetNodeCommand } from '@remnawave/backend-contract'
import { showModal } from '@shared/_modals/show-modal'
import { runAsync } from '@shared/lib/run-async'
import { memo } from 'react'
import { TbTerminal2 } from 'react-icons/tb'

interface IProps {
    node: GetNodeCommand.Response['response']
}

const OpenNodeLogsFeatureComponent = (props: IProps) => {
    const { node } = props

    return (
        <Tooltip label="Логи ноды / Xray">
            <ActionIcon
                color="cyan"
                onClick={() => {
                    runAsync(showModal('nodes_nodeLogsModal', { node }))
                }}
                size="lg"
                variant="soft"
            >
                <TbTerminal2 size={22} />
            </ActionIcon>
        </Tooltip>
    )
}

export const OpenNodeLogsFeature = memo(OpenNodeLogsFeatureComponent)
