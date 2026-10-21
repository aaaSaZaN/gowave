import { ActionIcon, Tooltip } from '@mantine/core'
import { GetNodeCommand } from '@remnawave/backend-contract'
import { memo } from 'react'
import { TbCloudDownload } from 'react-icons/tb'

import { showModal } from '@shared/_modals/show-modal'

interface IProps {
    node: GetNodeCommand.Response['response']
}

const OpenNodeUpdateFeatureComponent = (props: IProps) => {
    const { node } = props

    const isGoNode =
        (node as any)?.versions?.nodeType === 'go' ||
        (node as any)?.versions?.node?.includes('(Go)')

    if (!isGoNode) {
        return null
    }

    return (
        <Tooltip label="Обновить GoWave Node">
            <ActionIcon
                color="blue"
                onClick={() => {
                    showModal('nodes_updateNodeModal', { node })
                }}
                size="lg"
                variant="soft"
            >
                <TbCloudDownload size={22} />
            </ActionIcon>
        </Tooltip>
    )
}

export const OpenNodeUpdateFeature = memo(OpenNodeUpdateFeatureComponent)
