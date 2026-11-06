import NiceModal, { useModal } from '@ebay/nice-modal-react'
import { Button, Drawer, Group } from '@mantine/core'
import { useNiceMantineModal } from '@shared/_modals/use-nice-modal'
import { queryClient } from '@shared/api'
import { QueryKeys, useGetConfigProfiles, useGetHost } from '@shared/api/hooks'
import { OPEN_ENTITY } from '@shared/constants'
import { runAsync } from '@shared/lib/run-async'
import { HostClientConfigPreviewModal } from '@shared/ui'
import { LoaderModalShared } from '@shared/ui/loader-modal'
import { BaseOverlayHeader } from '@shared/ui/overlays/base-overlay-header'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { PiEye, PiListChecks } from 'react-icons/pi'

import { EditHostDrawerContent } from './edit-host.modal.content'

interface IProps {
    hostUuid: string
}

export const EditHostDrawer = NiceModal.create((props: IProps) => {
    const { hostUuid } = props
    const { t } = useTranslation()

    const modal = useModal()
    const { modalProps, hide } = useNiceMantineModal({
        modal,
        drawer: true,
        onClose() {
            runAsync(
                queryClient.refetchQueries({
                    queryKey: QueryKeys.hosts.getAllTags.queryKey
                })
            )
            runAsync(
                queryClient.refetchQueries({
                    queryKey: QueryKeys.hosts.getAllHosts.queryKey
                })
            )
        }
    })

    const [previewOpen, setPreviewOpen] = useState(false)

    const { data: host, isLoading: isGetHostLoading } = useGetHost({ route: { uuid: hostUuid } })
    const { data: configProfiles } = useGetConfigProfiles()

    const previewPayload = useMemo(() => ({ hostUuid: hostUuid }), [hostUuid])
    const previewInboundType = useMemo(() => {
        const profile = configProfiles?.configProfiles.find(
            (item) => item.uuid === host?.inbound.configProfileUuid
        )
        const inbound = profile?.inbounds.find(
            (inbound) => inbound.uuid === host?.inbound.configProfileInboundUuid
        )
        return inbound?.type ?? null
    }, [configProfiles, host])

    return (
        <>
            <Drawer
                {...modalProps}
                padding="lg"
                position="right"
                size="700px"
                title={
                    <Group justify="space-between" style={{ width: '100%', paddingRight: '25px' }}>
                        <BaseOverlayHeader
                            iconColor="teal"
                            openEntity={{ entity: OPEN_ENTITY.HOST, id: hostUuid }}
                            IconComponent={PiListChecks}
                            iconVariant="soft"
                            subtitle={hostUuid}
                            title={host?.remark ?? t('edit-host-modal.widget.edit-host')}
                            withCopy={true}
                        />
                        <Button
                            color="cyan"
                            disabled={isGetHostLoading || !host}
                            leftSection={<PiEye size={16} />}
                            onClick={() => {
                                setPreviewOpen(true)
                            }}
                            size="xs"
                            variant="light"
                        >
                            Preview Config
                        </Button>
                    </Group>
                }
            >
                {isGetHostLoading || !host ? (
                    <LoaderModalShared mih="78vh" />
                ) : (
                    <EditHostDrawerContent host={host} onClose={hide} />
                )}
            </Drawer>

            <HostClientConfigPreviewModal
                inboundType={previewInboundType}
                onClose={() => setPreviewOpen(false)}
                opened={previewOpen}
                payload={previewPayload}
            />
        </>
    )
})
