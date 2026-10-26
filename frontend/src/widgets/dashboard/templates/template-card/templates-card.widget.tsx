import { CopyButton, Menu } from '@mantine/core'
import type { GetSubscriptionTemplatesCommand } from '@remnawave/backend-contract'
import { showModal } from '@shared/_modals/show-modal'
import { ROUTES } from '@shared/constants'
import { WithDndSortable } from '@shared/hocs/with-dnd-sortable'
import { optionalProp } from '@shared/lib/optional-prop'
import { runAsync } from '@shared/lib/run-async'
import { EntityCardShared } from '@shared/ui/entity-card'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { PiCheck, PiCopy, PiPencil, PiTrashDuotone } from 'react-icons/pi'
import { TbTags } from 'react-icons/tb'
import { generatePath, useNavigate } from 'react-router'

interface IProps {
    disableReordering?: boolean
    handleDeleteTemplate: (templateUuid: string) => void
    isDragOverlay?: boolean
    template: GetSubscriptionTemplatesCommand.Response['response']['templates'][number]
    themeLogo: ReactNode
}

export function TemplatesCardWidget(props: IProps) {
    const {
        disableReordering = false,
        template,
        themeLogo,
        handleDeleteTemplate,
        isDragOverlay = false
    } = props

    const { t } = useTranslation()

    const navigate = useNavigate()

    const openTemplateEditor = () =>
        navigate(
            generatePath(ROUTES.DASHBOARD.TEMPLATES.TEMPLATE_EDITOR, {
                type: template.templateType,
                uuid: template.uuid
            })
        )

    return (
        <WithDndSortable
            disableReordering={disableReordering}
            dragHandlePosition="inline-end"
            id={template.uuid}
            isDragOverlay={isDragOverlay}
        >
            <EntityCardShared.Root
                isActive={template.name === 'Default'}
                onClick={openTemplateEditor}
            >
                <EntityCardShared.Header>
                    <EntityCardShared.Icon highlight={template.name === 'Default'}>
                        {themeLogo}
                    </EntityCardShared.Icon>
                    <EntityCardShared.Content tags={template.tags} title={template.name} />
                </EntityCardShared.Header>

                <EntityCardShared.Actions>
                    <EntityCardShared.Menu>
                        <CopyButton timeout={2000} value={template.uuid}>
                            {({ copied, copy }) => (
                                <Menu.Item
                                    {...optionalProp('color', copied ? 'teal' : undefined)}
                                    leftSection={
                                        copied ? <PiCheck size={18} /> : <PiCopy size={18} />
                                    }
                                    onClick={copy}
                                >
                                    {t('common.action.copy-uuid')}
                                </Menu.Item>
                            )}
                        </CopyButton>

                        <Menu.Item
                            disabled={template.name === 'Default'}
                            leftSection={<PiPencil size={18} />}
                            onClick={() => {
                                runAsync(
                                    showModal('renameModal', {
                                        renameFrom: 'template',
                                        name: template.name,
                                        uuid: template.uuid
                                    })
                                )
                            }}
                        >
                            {t('common.action.rename')}
                        </Menu.Item>

                        <Menu.Item
                            leftSection={<TbTags size={18} />}
                            onClick={() => {
                                runAsync(
                                    showModal('editTagsModal', {
                                        editTagsFrom: 'template',

                                        tags: template.tags,

                                        uuid: template.uuid
                                    })
                                )
                            }}
                        >
                            {t('common.field.tags')}
                        </Menu.Item>

                        <Menu.Item
                            color="red"
                            disabled={template.name === 'Default'}
                            leftSection={<PiTrashDuotone size={18} />}
                            onClick={(e) => {
                                e.stopPropagation()
                                handleDeleteTemplate(template.uuid)
                            }}
                        >
                            {t('common.action.delete')}
                        </Menu.Item>
                    </EntityCardShared.Menu>
                </EntityCardShared.Actions>
            </EntityCardShared.Root>
        </WithDndSortable>
    )
}
