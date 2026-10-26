import { ActionIcon, Tooltip } from '@mantine/core'
import type { GetNodeCommand } from '@remnawave/backend-contract'
import { showModal } from '@shared/_modals/show-modal'
import { runAsync } from '@shared/lib/run-async'
import { isGoNodeVersion } from '@shared/utils/misc/node-versions'
import { memo } from 'react'
import { TbCloudDownload } from 'react-icons/tb'

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
                    runAsync(showModal('nodes_updateNodeModal', { node }))
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
