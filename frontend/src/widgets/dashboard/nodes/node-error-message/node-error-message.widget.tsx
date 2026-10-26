import { ErrorMessageBlock } from '@shared/ui/error-message-block'
import { BaseOverlayHeader } from '@shared/ui/overlays/base-overlay-header'
import { SectionCard } from '@shared/ui/section-card'
import { isNodeConfigRejected } from '@shared/utils/misc'
import { useTranslation } from 'react-i18next'
import { TbAlertTriangle } from 'react-icons/tb'

import type { IProps } from './interfaces'

export const NodeErrorMessageWidget = (props: IProps) => {
    const { t } = useTranslation()

    const { node } = props

    if (!node?.lastStatusMessage) {
        return null
    }

    const isConfigRejected = isNodeConfigRejected(node)

    return (
        <SectionCard.Root>
            <SectionCard.Section>
                <BaseOverlayHeader
                    iconColor={isConfigRejected ? 'orange' : 'red'}
                    IconComponent={TbAlertTriangle}
                    iconVariant="soft"
                    subtitle={
                        isConfigRejected
                            ? t('node-error-message.config-not-applied-description')
                            : undefined
                    }
                    title={
                        isConfigRejected
                            ? t('node-error-message.config-not-applied')
                            : t('common.message.last-error-message')
                    }
                    titleOrder={5}
                />
            </SectionCard.Section>

            <SectionCard.Section>
                <ErrorMessageBlock message={node.lastStatusMessage} />
            </SectionCard.Section>
        </SectionCard.Root>
    )
}
