import NiceModal, { useModal } from '@ebay/nice-modal-react'
import { Modal } from '@mantine/core'
import type { TSubscriptionTemplateType } from '@remnawave/backend-contract'
import { useNiceMantineModal } from '@shared/_modals/use-nice-modal'
import { requireValue } from '@shared/lib/invariant'
import { BaseOverlayHeader } from '@shared/ui/overlays/base-overlay-header'
import { useTranslation } from 'react-i18next'
import { TbNewSection } from 'react-icons/tb'
import type { NavigateFunction } from 'react-router'

import { CreateExternalSquadContent } from './create-external-squad.content'
import { CreateInternalSquadContent } from './create-internal-squad.content'
import { CreateNodePluginContent } from './create-node-plugin.content'
import { CreateSubpageConfigContent } from './create-subpage-config.content'
import { CreateTemplateContent } from './create-template.content'
import { CreateConfigProfileContent } from './create-сonfig-profile.content'

type CreateFrom =
    | 'template'
    | 'externalSquad'
    | 'configProfile'
    | 'internalSquad'
    | 'nodePlugin'
    | 'subpageConfig'

interface IProps {
    createFrom: CreateFrom
    contentOptions: {
        navigate?: NavigateFunction
        templateType?: TSubscriptionTemplateType
    }
}

export const CreateModal = NiceModal.create((props: IProps) => {
    const { createFrom, contentOptions } = props
    const modal = useModal()
    const { modalProps, hide } = useNiceMantineModal({ modal })

    const { t } = useTranslation()

    return (
        <Modal
            {...modalProps}
            title={
                <BaseOverlayHeader
                    iconColor="teal"
                    IconComponent={TbNewSection}
                    iconVariant="soft"
                    title={t('common.field.creation')}
                />
            }
        >
            {createFrom === 'template' && contentOptions.templateType && (
                <CreateTemplateContent
                    templateType={contentOptions.templateType}
                    onClose={hide}
                    navigate={requireValue(
                        contentOptions.navigate,
                        'Navigation is required for this modal'
                    )}
                />
            )}
            {createFrom === 'internalSquad' && <CreateInternalSquadContent onClose={hide} />}
            {createFrom === 'subpageConfig' && (
                <CreateSubpageConfigContent
                    onClose={hide}
                    navigate={requireValue(
                        contentOptions.navigate,
                        'Navigation is required for this modal'
                    )}
                />
            )}
            {createFrom === 'externalSquad' && <CreateExternalSquadContent onClose={hide} />}
            {createFrom === 'configProfile' && (
                <CreateConfigProfileContent
                    onClose={hide}
                    navigate={requireValue(
                        contentOptions.navigate,
                        'Navigation is required for this modal'
                    )}
                />
            )}
            {createFrom === 'nodePlugin' && (
                <CreateNodePluginContent
                    onClose={hide}
                    navigate={requireValue(
                        contentOptions.navigate,
                        'Navigation is required for this modal'
                    )}
                />
            )}
        </Modal>
    )
})
