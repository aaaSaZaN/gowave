import NiceModal, { useModal } from '@ebay/nice-modal-react'
import { Button, CopyButton, Drawer, Group, Modal, Stack, Textarea, TextInput } from '@mantine/core'
import { schemaResolver, useForm } from '@mantine/form'
import { notifications } from '@mantine/notifications'
import {
    CreateHostCommand,
    INTERNAL_SQUADS_MODE,
    SECURITY_LAYERS
} from '@remnawave/backend-contract'
import { useNiceMantineModal } from '@shared/_modals/use-nice-modal'
import { queryClient } from '@shared/api'
import { instance } from '@shared/api/axios'
import {
    QueryKeys,
    useCreateHost,
    useGetConfigProfiles,
    useGetHostTags,
    useGetInternalSquads,
    useGetNodes,
    useGetSubscriptionTemplates
} from '@shared/api/hooks'
import { runAsync } from '@shared/lib/run-async'
import { LoadingScreen } from '@shared/ui'
import { BaseHostForm } from '@shared/ui/forms/hosts/base-host-form'
import { BaseOverlayHeader } from '@shared/ui/overlays/base-overlay-header'
import { parseJsonField } from '@shared/utils/misc'
import { isAxiosError } from 'axios'
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
    const [previewUA, setPreviewUA] = useState('ClashMeta')
    const [previewContent, setPreviewContent] = useState('')
    const [previewLoading, setPreviewLoading] = useState(false)

    const { data: configProfiles } = useGetConfigProfiles()
    const { data: nodes } = useGetNodes()
    const { data: internalSquads } = useGetInternalSquads()
    const { data: templates } = useGetSubscriptionTemplates()
    const { data: hostTags } = useGetHostTags()

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

    const handlePreview = async () => {
        setPreviewLoading(true)
        try {
            const values = form.getValues()
            const res = await instance.post('/api/hosts/preview', {
                host: {
                    remark: values.remark || 'PreviewHost',
                    address: values.address || 'example.com',
                    port: Number(values.port) || 443,
                    path: values.path || '',
                    sni: values.sni || values.address || 'example.com',
                    fingerprint: values.fingerprint || 'chrome',
                    securityLayer: values.securityLayer || 'DEFAULT'
                },
                userAgent: previewUA,
                protocol: 'vless'
            })
            setPreviewContent(res.data.response?.previewContent || '')
        } catch (err: unknown) {
            const responseMessage = isAxiosError<{ message?: string }>(err)
                ? err.response?.data?.message
                : undefined
            notifications.show({
                title: 'Preview Error',
                message:
                    responseMessage || (err instanceof Error ? err.message : 'Failed to preview'),
                color: 'red'
            })
        } finally {
            setPreviewLoading(false)
        }
    }

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
                                setPreviewOpen(true)
                                runAsync(handlePreview())
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

            <Modal
                opened={previewOpen}
                onClose={() => setPreviewOpen(false)}
                title="Host Client Config Preview"
                size="lg"
            >
                <Stack gap="md">
                    <Group align="flex-end">
                        <TextInput
                            label="Client User-Agent"
                            value={previewUA}
                            onChange={(e) => setPreviewUA(e.currentTarget.value)}
                            placeholder="ClashMeta, Sing-Box, v2rayN..."
                            style={{ flex: 1 }}
                        />
                        <Button loading={previewLoading} onClick={handlePreview}>
                            Generate
                        </Button>
                    </Group>
                    <Textarea
                        label="Client Output"
                        value={previewContent}
                        readOnly
                        rows={12}
                        styles={{ input: { fontFamily: 'monospace', fontSize: '12px' } }}
                    />
                    <Group justify="flex-end">
                        <CopyButton value={previewContent}>
                            {({ copied, copy }) => (
                                <Button color={copied ? 'teal' : 'blue'} onClick={copy}>
                                    {copied ? 'Copied' : 'Copy Config'}
                                </Button>
                            )}
                        </CopyButton>
                    </Group>
                </Stack>
            </Modal>
        </>
    )
})
