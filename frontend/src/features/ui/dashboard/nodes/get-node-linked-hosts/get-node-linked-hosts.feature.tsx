import { ActionIcon, Tooltip } from '@mantine/core'
import { showModal } from '@shared/_modals/show-modal'
import { runAsync } from '@shared/lib/run-async'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'
import { TbServerCog } from 'react-icons/tb'

import type { IProps } from './interfaces'

const GetNodeLinkedHostsFeatureComponent = (props: IProps) => {
    const { nodeUuid } = props
    const { t } = useTranslation()

    return (
        <Tooltip label={t('get-node-linked-hosts.feature.linked-hosts')}>
            <ActionIcon
                color="cyan"
                onClick={() => {
                    runAsync(
                        showModal('nodes_linkedHostsDrawer', {
                            nodeUuid
                        })
                    )
                }}
                size="lg"
                variant="soft"
            >
                <TbServerCog size="22px" />
            </ActionIcon>
        </Tooltip>
    )
}

export const GetNodeLinkedHostsFeature = memo(GetNodeLinkedHostsFeatureComponent)
