import { CopyButton, Menu } from '@mantine/core'
import type { GetNodePluginsCommand } from '@remnawave/backend-contract'
import { showModal } from '@shared/_modals/show-modal'
import { ROUTES } from '@shared/constants'
import { WithDndSortable } from '@shared/hocs/with-dnd-sortable'
import { optionalProp } from '@shared/lib/optional-prop'
import { runAsync } from '@shared/lib/run-async'
import { EntityCardShared } from '@shared/ui/entity-card'
import { useTranslation } from 'react-i18next'
import { PiCheck, PiCopy, PiCpu, PiPencil, PiTrashDuotone } from 'react-icons/pi'
import { TbCopyCheck, TbPackage, TbTags } from 'react-icons/tb'
import { generatePath, useNavigate } from 'react-router'

interface IProps {
    disableReordering?: boolean
    handleCloneNodePlugin: (nodePluginUuid: string) => void
    handleDeleteNodePlugin: (nodePluginUuid: string) => void
    handleShowActiveNodes: (nodePluginUuid: string) => void
    isDragOverlay?: boolean
    nodePlugin: GetNodePluginsCommand.Response['response']['nodePlugins'][number]
}

export function NodePluginCardWidget(props: IProps) {
    const {
        disableReordering = false,
        nodePlugin,
        handleDeleteNodePlugin,
        handleCloneNodePlugin,
        handleShowActiveNodes,
        isDragOverlay = false
    } = props

    const { t } = useTranslation()
    const navigate = useNavigate()

    const navigateToNodePlugin = () => {
        runAsync(
            navigate(
                generatePath(ROUTES.DASHBOARD.MANAGEMENT.NODE_PLUGINS.NODE_PLUGIN_BY_UUID, {
                    uuid: nodePlugin.uuid
                })
            )
        )
    }

    return (
        <WithDndSortable
            disableReordering={disableReordering}
            dragHandlePosition="inline-end"
            id={nodePlugin.uuid}
            isDragOverlay={isDragOverlay}
        >
            <EntityCardShared.Root onClick={navigateToNodePlugin}>
                <EntityCardShared.Header>
                    <EntityCardShared.Icon highlight={false}>
                        <TbPackage size={22} />
                    </EntityCardShared.Icon>

                    <EntityCardShared.Content tags={nodePlugin.tags} title={nodePlugin.name} />
                </EntityCardShared.Header>

                <EntityCardShared.Actions>
                    <EntityCardShared.Menu>
                        <CopyButton timeout={2000} value={nodePlugin.uuid}>
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
                            leftSection={<PiCpu size={18} />}
                            onClick={() => handleShowActiveNodes(nodePlugin.uuid)}
                        >
                            {t('node-plugin-card.widget.active-on-nodes')}
                        </Menu.Item>

                        <Menu.Item
                            leftSection={<PiPencil size={18} />}
                            onClick={() => {
                                runAsync(
                                    showModal('renameModal', {
                                        renameFrom: 'nodePlugin',
                                        name: nodePlugin.name,
                                        uuid: nodePlugin.uuid
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
                                        editTagsFrom: 'nodePlugin',

                                        tags: nodePlugin.tags,

                                        uuid: nodePlugin.uuid
                                    })
                                )
                            }}
                        >
                            {t('common.field.tags')}
                        </Menu.Item>

                        <Menu.Item
                            leftSection={<TbCopyCheck size={18} />}
                            onClick={() => handleCloneNodePlugin(nodePlugin.uuid)}
                        >
                            {t('common.action.clone')}
                        </Menu.Item>

                        <Menu.Item
                            color="red"
                            leftSection={<PiTrashDuotone size={18} />}
                            onClick={(e) => {
                                e.stopPropagation()
                                handleDeleteNodePlugin(nodePlugin.uuid)
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
