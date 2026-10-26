import { schemaResolver, useForm } from '@mantine/form'
import { UpdateHostCommand } from '@remnawave/backend-contract'
import { queryClient } from '@shared/api'
import {
    QueryKeys,
    useGetConfigProfiles,
    useGetHostTags,
    useGetInternalSquads,
    useGetNodes,
    useGetSubscriptionTemplates,
    useUpdateHost
} from '@shared/api/hooks'
import { BaseHostForm } from '@shared/ui/forms/hosts/base-host-form'
import { LoaderModalShared } from '@shared/ui/loader-modal'
import { parseJsonField, stringifyJsonField } from '@shared/utils/misc'
import { useEffect } from 'react'

interface IProps {
    host: UpdateHostCommand.Response['response']
    onClose: () => void
}

export const EditHostDrawerContent = (props: IProps) => {
    const { host, onClose } = props

    const { data: configProfiles } = useGetConfigProfiles()
    const { data: nodes } = useGetNodes()
    const { data: templates } = useGetSubscriptionTemplates()
    const { data: internalSquads } = useGetInternalSquads()
    const { data: hostTags } = useGetHostTags()

    const form = useForm<UpdateHostCommand.RequestBody>({
        name: 'edit-host-form',
        mode: 'uncontrolled',
        validateInputOnBlur: true,
        onValuesChange: (values) => {
            if (typeof values.vlessRouteId === 'string' && values.vlessRouteId === '') {
                form.setFieldValue('vlessRouteId', null)
            }
        },
        validate: schemaResolver(UpdateHostCommand.RequestBodySchema.omit({ uuid: true }))
    })

    const { mutate: updateHost, isPending: isUpdateHostPending } = useUpdateHost({
        mutationFns: {
            onSuccess: (data) => {
                queryClient.setQueryData(
                    QueryKeys.hosts.getHost({ uuid: host.uuid }).queryKey,
                    data
                )
                onClose()
            }
        }
    })

    useEffect(() => {
        if (configProfiles) {
            form.initialize({
                uuid: host.uuid,
                remark: host.remark,
                address: host.address,
                port: host.port,
                securityLayer: host.securityLayer,
                isDisabled: host.isDisabled,
                sni: host.sni ?? undefined,
                host: host.host ?? undefined,
                path: host.path ?? undefined,
                alpn: host.alpn ?? undefined,
                fingerprint: host.fingerprint ?? undefined,
                inbound: {
                    configProfileUuid: host.inbound.configProfileUuid ?? '',
                    configProfileInboundUuid: host.inbound.configProfileInboundUuid ?? ''
                },
                serverDescription: host.serverDescription ?? undefined,
                xhttpExtraParams: stringifyJsonField(host.xhttpExtraParams),
                muxParams: stringifyJsonField(host.muxParams),
                sockoptParams: stringifyJsonField(host.sockoptParams),
                finalMask: stringifyJsonField(host.finalMask),
                mapper: host.mapper,
                tags: host.tags ?? undefined,
                isHidden: host.isHidden,
                overrideSniFromAddress: host.overrideSniFromAddress,
                keepSniBlank: host.keepSniBlank,
                vlessRouteId: host.vlessRouteId ?? undefined,
                pinnedPeerCertSha256: host.pinnedPeerCertSha256 ?? undefined,
                verifyPeerCertByName: host.verifyPeerCertByName ?? undefined,
                shuffleHost: host.shuffleHost ?? undefined,
                mihomoX25519: host.mihomoX25519 ?? undefined,
                mihomoIpVersion: host.mihomoIpVersion ?? undefined,
                nodes: host.nodes ?? undefined,
                xrayJsonTemplateUuid: host.xrayJsonTemplateUuid ?? undefined,
                internalSquads: host.internalSquads ?? undefined,
                excludeFromSubscriptionTypes: host.excludeFromSubscriptionTypes ?? undefined
            })
        }
    }, [
        configProfiles,
        host.mihomoIpVersion,
        host.mihomoX25519,
        host.muxParams,
        host.nodes,
        host.overrideSniFromAddress,
        host.path,
        host.pinnedPeerCertSha256,
        host.port,
        host.remark,
        host.securityLayer,
        host.serverDescription,
        host.shuffleHost,
        host.sni,
        host.sockoptParams,
        host.tags,
        host.uuid,
        host.verifyPeerCertByName,
        host.vlessRouteId,
        host.xhttpExtraParams,
        host.mapper,
        host.xrayJsonTemplateUuid,
        host.keepSniBlank,
        host.isHidden,
        host.isDisabled,
        host.internalSquads,
        host.inbound.configProfileUuid,
        host.fingerprint,
        host.inbound.configProfileInboundUuid,
        host.finalMask,
        host.address,
        host.host,
        host.excludeFromSubscriptionTypes,
        host.alpn,
        form.initialize
    ])

    form.watch('inbound.configProfileInboundUuid', ({ value }) => {
        const { inbound } = form.getValues()
        if (!inbound?.configProfileUuid) {
            return
        }

        const configProfile = configProfiles?.configProfiles.find(
            (configProfile) => configProfile.uuid === inbound.configProfileUuid
        )
        if (configProfile) {
            form.setFieldValue(
                'port',
                configProfile.inbounds.find((inbound) => inbound.uuid === value)?.port ?? undefined
            )
        }
    })

    const handleSubmit = form.onSubmit((values) => {
        updateHost({
            variables: {
                ...values,
                uuid: host.uuid,
                xhttpExtraParams: parseJsonField(values.xhttpExtraParams),
                muxParams: parseJsonField(values.muxParams),
                sockoptParams: parseJsonField(values.sockoptParams),
                finalMask: parseJsonField(values.finalMask)
            }
        })
    })

    if (!configProfiles || !nodes || !templates || !internalSquads || !hostTags) {
        return <LoaderModalShared mih="78vh" />
    }

    return (
        <BaseHostForm
            configProfiles={configProfiles.configProfiles}
            form={form}
            handleSubmit={handleSubmit}
            hostTags={hostTags.tags}
            internalSquads={internalSquads.internalSquads}
            isSubmitting={isUpdateHostPending}
            nodes={nodes}
            hostUuid={host.uuid}
            subscriptionTemplates={templates.templates}
        />
    )
}
