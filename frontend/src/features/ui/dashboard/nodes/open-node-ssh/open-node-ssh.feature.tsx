import { useExperimentalFeature } from '@entities/dashboard/view-preferences-store'
import { ActionIcon, Tooltip } from '@mantine/core'
import type { GetNodeCommand } from '@remnawave/backend-contract'
import { showModal } from '@shared/_modals/show-modal'
import { useIsMobile } from '@shared/hooks'
import { runAsync } from '@shared/lib/run-async'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'
import { TbTerminal2 } from 'react-icons/tb'

interface IProps {
    node: GetNodeCommand.Response['response']
}

const OpenNodeSshFeatureComponent = (props: IProps) => {
    const { node } = props
    const { t } = useTranslation()

    const isMobile = useIsMobile()
    const isSshTerminalEnabled = useExperimentalFeature('sshTerminal')

    if (isMobile || !isSshTerminalEnabled) {
        return null
    }

    return (
        <Tooltip label={t('node-ssh.title')}>
            <ActionIcon
                color="cyan"
                onClick={() => {
                    runAsync(showModal('nodes_nodeSshTerminal', { node }))
                }}
                size="lg"
                variant="soft"
            >
                <TbTerminal2 size="22px" />
            </ActionIcon>
        </Tooltip>
    )
}

export const OpenNodeSshFeature = memo(OpenNodeSshFeatureComponent)
