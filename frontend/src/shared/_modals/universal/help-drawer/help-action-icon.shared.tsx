import { ActionIcon, type ActionIconProps, Tooltip } from '@mantine/core'
import { showModal } from '@shared/_modals/show-modal'
import { runAsync } from '@shared/lib/run-async'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'
import type { IconBaseProps } from 'react-icons/lib'
import { TbQuestionMark } from 'react-icons/tb'

import type { THelpDrawerAvailableScreen } from './help-drawer.types'

interface IProps {
    actionIconProps?: Omit<ActionIconProps, 'onClick'>
    hidden?: boolean
    iconProps?: IconBaseProps
    screen: THelpDrawerAvailableScreen
}

export const HelpActionIconShared = memo((props: IProps) => {
    const { actionIconProps, hidden, iconProps, screen } = props

    const { t } = useTranslation()

    if (hidden) {
        return null
    }

    const handleOpenHelpDrawer = () => {
        runAsync(
            showModal('helpDrawer', {
                screen
            })
        )
    }

    return (
        <Tooltip label={t('help-action-icon.shared.help-article')}>
            <ActionIcon
                color="lime"
                onClick={handleOpenHelpDrawer}
                size="input-md"
                variant="soft"
                {...actionIconProps}
            >
                <TbQuestionMark size={24} {...iconProps} />
            </ActionIcon>
        </Tooltip>
    )
})
