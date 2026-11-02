import { Group, Loader, Stack, Text } from '@mantine/core'
import { useGetNodes } from '@shared/api/hooks'
import { EditorStatusBar } from '@shared/ui/code-editor'
import { ErrorMessageBlock } from '@shared/ui/error-message-block'
import { isNodeConfigRejected } from '@shared/utils/misc'
import { sToMs } from '@shared/utils/time-utils'
import { useTranslation } from 'react-i18next'

interface IProps {
    configProfileUuid: string
    savedAt: null | number
}

// A node takes up to 15 seconds to test a config and 40 to restart Xray.
const POLLING_WINDOW_MS = sToMs(60)
// The queue picks a node start up about a second after the save; until then the
// node is not marked as connecting yet.
const QUEUE_DELAY_MS = sToMs(3)

// Saving a profile only queues node restarts, so a rejection arrives after the
// save has already succeeded. Polling lives here to keep the editor from
// rendering on every refetch.
export function ConfigRejectionStatusBarWidget(props: IProps) {
    const { configProfileUuid, savedAt } = props
    const { t } = useTranslation()

    const { data: nodes, dataUpdatedAt } = useGetNodes({
        rQueryParams: {
            refetchInterval: () =>
                savedAt !== null && Date.now() - savedAt < POLLING_WINDOW_MS ? sToMs(2) : false
        }
    })

    const profileNodes = (nodes ?? []).filter(
        (node) =>
            node.configProfile.activeConfigProfileUuid === configProfileUuid && !node.isDisabled
    )

    const sinceSave = savedAt === null ? null : dataUpdatedAt - savedAt

    if (sinceSave !== null && sinceSave < POLLING_WINDOW_MS) {
        const checkingNodes =
            sinceSave < QUEUE_DELAY_MS
                ? profileNodes
                : profileNodes.filter((node) => node.isConnecting)

        if (checkingNodes.length > 0) {
            return (
                <EditorStatusBar key="checking" status="warning">
                    <Group gap="xs" wrap="nowrap">
                        <Loader color="orange" size="xs" />
                        <Text c="orange" size="sm">
                            {t('node-error-message.config-checking', {
                                nodes: checkingNodes.map((node) => node.name).join(', ')
                            })}
                        </Text>
                    </Group>
                </EditorStatusBar>
            )
        }
    }

    const nodeNamesByMessage = new Map<string, string[]>()

    for (const node of profileNodes) {
        if (!isNodeConfigRejected(node) || !node.lastStatusMessage) continue

        const names = nodeNamesByMessage.get(node.lastStatusMessage) ?? []
        nodeNamesByMessage.set(node.lastStatusMessage, [...names, node.name])
    }

    if (nodeNamesByMessage.size === 0) return null

    return (
        // Its own key, or the bar keeps the scroll position of the shorter one.
        <EditorStatusBar key="rejected" status="warning">
            <Stack gap="xs">
                <div>
                    <Text c="orange" fw={600} size="sm">
                        {t('node-error-message.config-not-applied')}
                    </Text>
                    <Text c="dimmed" size="xs">
                        {t('node-error-message.config-not-applied-description')}
                    </Text>
                </div>

                {[...nodeNamesByMessage].map(([message, names]) => (
                    <Stack gap={4} key={message}>
                        <Text fw={500} size="xs">
                            {names.join(', ')}
                        </Text>
                        <ErrorMessageBlock message={message} />
                    </Stack>
                ))}
            </Stack>
        </EditorStatusBar>
    )
}
