import NiceModal, { useModal } from '@ebay/nice-modal-react'
import { Button, Drawer, Group } from '@mantine/core'
import { schemaResolver, useForm } from '@mantine/form'
import { notifications } from '@mantine/notifications'
import {
    CreateHostCommand,
    INTERNAL_SQUADS_MODE,
    SECURITY_LAYERS
} from '@remnawave/backend-contract'
import { useNiceMantineModal } from '@shared/_modals/use-nice-modal'
import { queryClient } from '@shared/api'
import {
    QueryKeys,
    useCreateHost,
    useGetConfigProfiles,
    useGetHostTags,
    useGetInternalSquads,
    useGetNodes,
    useGetSubscriptionTemplates
} from '@shared/api/hooks'
import { HostClientConfigPreviewModal, LoadingScreen } from '@shared/ui'
import { BaseHostForm } from '@shared/ui/forms/hosts/base-host-form'
import type { HostPreviewPayload } from '@shared/ui/host-client-config-preview/host-client-config-preview'
import { BaseOverlayHeader } from '@shared/ui/overlays/base-overlay-header'
import { parseJsonField } from '@shared/utils/misc'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { PiEye, PiListChecks } from 'react-icons/pi'

export const CreateHostDrawer = NiceModal.create(() => {
    const { t } = useTranslation()

    const modal = useModal()
    const { modalProps, hide } = useNiceMantineModal({
        modal,
        drawer: true
    })

    const [previewOpen, setPreviewOpen] = useState(false)
    const [previewPayload, setPreviewPayload] = useState<HostPreviewPayload>({ host: {} })
    const [previewInboundType, setPreviewInboundType] = useState<string | null>(null)

    const { data: configProfiles } = useGetConfigProfiles()
    const { data: nodes } = useGetNodes()
    const { data: internalSquads } = useGetInternalSquads()
    const { data: templates } = useGetSubscriptionTemplates()
    const { data: hostTags } = useGetHostTags()

    const openPreview = () => {
        const values = form.getValues()

        const selectedProfile = configProfiles?.configProfiles.find(
            (item) => item.uuid === values.inbound.configProfileUuid
        )
        const selectedInbound = selectedProfile?.inbounds.find(
            (item) => item.uuid === values.inbound.configProfileInboundUuid
        )

        setPreviewPayload({
            host: {
                remark: values.remark || 'PreviewHost',
                address: values.address || 'example.com',
                port: Number(values.port) || 443,
                path: values.path || '',
                sni: values.sni || values.address || 'example.com',
                fingerprint: values.fingerprint || 'chrome',
                securityLayer: values.securityLayer || 'DEFAULT',
                configProfileUuid: values.inbound.configProfileUuid,
                configProfileInboundUuid: values.inbound.configProfileInboundUuid
            }
        })
        setPreviewInboundType(selectedInbound?.type ?? null)
        setPreviewOpen(true)
    }

    const form = useForm<CreateHostCommand.RequestBody>({
        mode: 'uncontrolled',
        name: 'create-host-form',
        validateInputOnBlur: true,
        onValuesChange: (values) => {
            if (typeof values.vlessRouteId === 'string' && values.vlessRouteId === '') {
                form.setFieldValue('vlessRouteId', null)
            }
        },
        validate: schemaResolver(CreateHostCommand.RequestBodySchema),

        initialValues: {
            isDisabled: true,
            securityLayer: SECURITY_LAYERS.DEFAULT,
            port: 0,
            remark: '',
            address: '',
            inbound: {
                configProfileUuid: '',
                configProfileInboundUuid: ''
            },
            internalSquads: {
                mode: INTERNAL_SQUADS_MODE.EXCLUDE,
                squads: []
            }
        }
    })

    const { mutate: createHost, isPending: isCreateHostPending } = useCreateHost({
        mutationFns: {
            onSuccess: async () => {
                hide()

                await queryClient.refetchQueries({
                    queryKey: QueryKeys.hosts.getAllTags.queryKey
                })

                await queryClient.refetchQueries({
                    queryKey: QueryKeys.hosts.getAllHosts.queryKey
                })
            }
        }
    })

    const handleSubmit = form.onSubmit((values) => {
        if (!values.inbound.configProfileInboundUuid || !values.inbound.configProfileUuid) {
            notifications.show({
                title: t('common.message.error'),
                message: t('create-host-modal.widget.please-select-the-config-profile-and-inbound'),
                color: 'red'
            })

            return
        }

        createHost({
            variables: {
                ...values,
                sockoptParams: parseJsonField(values.sockoptParams),
                muxParams: parseJsonField(values.muxParams),
                xhttpExtraParams: parseJsonField(values.xhttpExtraParams),
                finalMask: parseJsonField(values.finalMask),
                inbound: {
                    configProfileInboundUuid: values.inbound.configProfileInboundUuid,
                    configProfileUuid: values.inbound.configProfileUuid
                }
            }
        })

        return
    })

    form.watch('inbound.configProfileInboundUuid', ({ value }) => {
        const { configProfileUuid } = form.getValues().inbound
        if (!configProfileUuid) {
            return
        }

        const configProfile = configProfiles?.configProfiles.find(
            (configProfile) => configProfile.uuid === configProfileUuid
        )
        if (configProfile) {
            form.setFieldValue(
                'port',
                configProfile.inbounds.find((inbound) => inbound.uuid === value)?.port ?? 0
            )
        }
    })

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
                            IconComponent={PiListChecks}
                            iconVariant="soft"
                            title={t('create-host-modal.widget.new-host')}
                        />
                        <Button
                            size="xs"
                            variant="light"
                            color="cyan"
                            leftSection={<PiEye size={16} />}
                            onClick={() => {
                                openPreview()
                            }}
                        >
                            Preview Config
                        </Button>
                    </Group>
                }
            >
                {!configProfiles || !nodes || !templates || !internalSquads || !hostTags ? (
                    <LoadingScreen />
                ) : (
                    <BaseHostForm
                        configProfiles={configProfiles.configProfiles}
                        form={form}
                        handleSubmit={handleSubmit}
                        hostTags={hostTags.tags}
                        internalSquads={internalSquads.internalSquads}
                        isSubmitting={isCreateHostPending}
                        nodes={nodes}
                        subscriptionTemplates={templates.templates}
                    />
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
