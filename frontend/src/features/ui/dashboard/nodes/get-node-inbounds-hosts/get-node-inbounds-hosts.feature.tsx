import { ActionIcon, Tooltip } from '@mantine/core'
import { showModal } from '@shared/_modals/show-modal'
import { runAsync } from '@shared/lib/run-async'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'
import { TbSitemap } from 'react-icons/tb'

interface IProps {
    nodeUuid: string
}

const GetNodeInboundsHostsFeatureComponent = (props: IProps) => {
    const { nodeUuid } = props
    const { t } = useTranslation()

    return (
        <Tooltip label={t('common.field.inbounds-hosts')}>
            <ActionIcon
                color="teal"
                onClick={() => {
                    runAsync(
                        showModal('nodes_nodeInboundsHostsDrawer', {
                            nodeUuid
                        })
                    )
                }}
                size="lg"
                variant="soft"
            >
                <TbSitemap size="22px" />
            </ActionIcon>
        </Tooltip>
    )
}

export const GetNodeInboundsHostsFeature = memo(GetNodeInboundsHostsFeatureComponent)
