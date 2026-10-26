import { ActionIcon, Tooltip } from '@mantine/core'
import { GetNodeCommand } from '@remnawave/backend-contract'
import { memo } from 'react'
import { TbCloudDownload } from 'react-icons/tb'

import { showModal } from '@shared/_modals/show-modal'
import { isGoNodeVersion } from '@shared/utils/misc/node-versions'

interface IProps {
    node: GetNodeCommand.Response['response']
}

const OpenNodeUpdateFeatureComponent = (props: IProps) => {
    const { node } = props

    const isGoNode = isGoNodeVersion(node.versions)

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
