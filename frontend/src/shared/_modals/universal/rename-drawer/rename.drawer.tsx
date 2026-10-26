import NiceModal, { useModal } from '@ebay/nice-modal-react'
import { ActionIcon, type ModalProps, TextInput } from '@mantine/core'
import { useField } from '@mantine/form'
import {
    UpdateConfigProfileCommand,
    UpdateExternalSquadCommand,
    UpdateInternalSquadCommand,
    UpdateNodePluginCommand,
    UpdatePasskeyCommand,
    UpdateSubpageConfigCommand,
    UpdateSubscriptionTemplateCommand
} from '@remnawave/backend-contract'
import { useNiceMantineModal } from '@shared/_modals/use-nice-modal'
import {
    QueryKeys,
    useUpdateConfigProfile,
    useUpdateExternalSquad,
    useUpdateInternalSquad,
    useUpdateNodePlugin,
    useUpdatePasskey,
    useUpdateSubpageConfig,
    useUpdateSubscriptionTemplate
} from '@shared/api/hooks'
import { queryClient } from '@shared/api/query-client'
import { runAsync } from '@shared/lib/run-async'
import { CompoundModalShared } from '@shared/ui/compound-modal/compound-modal.shared'
import { BaseOverlayHeader } from '@shared/ui/overlays/base-overlay-header'
import { type ReactNode, useId } from 'react'
import { useTranslation } from 'react-i18next'
import { TbDeviceFloppy, TbPencil } from 'react-icons/tb'

type RenameType =
    | 'configProfile'
    | 'externalSquad'
    | 'internalSquad'
    | 'nodePlugin'
    | 'passkey'
    | 'subpageConfig'
    | 'template'

interface IProps {
    name: string
    renameFrom: RenameType
    uuid: string
}

interface IBodyProps {
    modalProps: Omit<ModalProps, 'children' | 'title'>
    name: string
    onSaved: () => void
    uuid: string
}

const validateWith =
    (schema: {
        safeParse: (value: unknown) => {
            error?: { issues: { message: string }[] }
            success: boolean
        }
    }) =>
    (value: string) => {
        const result = schema.safeParse({ name: value })

        return result.success ? null : (result.error?.issues[0]?.message ?? null)
    }

function RenameBody<V>({
    modalProps,
    name,
    uuid,
    mutate,
    isPending,
    buildVariables,
    validate
}: IBodyProps & {
    isPending: boolean
    mutate: (args: { variables: V }) => void
    buildVariables: (uuid: string, name: string) => V
    validate: (value: string) => string | null
}) {
    const { t } = useTranslation()
    const formId = useId()

    const nameField = useField<string>({
        mode: 'controlled',
        initialValue: '',
        validate
    })

    const handleSubmit = async () => {
        if (await nameField.validate()) return

        mutate({ variables: buildVariables(uuid, nameField.getValue()) })
    }

    return (
        <CompoundModalShared
            buttons={
                <ActionIcon
                    color="teal"
                    disabled={!!nameField.error || !nameField.getValue()}
                    form={formId}
                    loading={isPending}
                    type="submit"
                    size="lg"
                    variant="soft"
                >
                    <TbDeviceFloppy size="20px" />
                </ActionIcon>
            }
            modalProps={modalProps}
            title={
                <BaseOverlayHeader
                    iconColor="teal"
                    IconComponent={TbPencil}
                    iconVariant="soft"
                    title={t('common.action.rename')}
                />
            }
        >
            <form
                id={formId}
                onSubmit={(event) => {
                    event.preventDefault()
                    runAsync(handleSubmit())
                }}
            >
                <TextInput
                    data-autofocus
                    key={nameField.key}
                    placeholder={name}
                    {...nameField.getInputProps()}
                    required
                />
            </form>
        </CompoundModalShared>
    )
}

const validateConfigProfileRenameBody = validateWith(
    UpdateConfigProfileCommand.RequestBodySchema.omit({ uuid: true })
)
function ConfigProfileRenameBody(props: IBodyProps) {
    const { mutate, isPending } = useUpdateConfigProfile({
        mutationFns: {
            onSuccess: () => {
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.configProfiles.getConfigProfiles.queryKey
                    })
                )
                props.onSaved()
            }
        }
    })
    return (
        <RenameBody
            {...props}
            mutate={mutate}
            isPending={isPending}
            buildVariables={(uuid, name) => ({ uuid, name })}
            validate={validateConfigProfileRenameBody}
        />
    )
}

const validateExternalSquadRenameBody = validateWith(
    UpdateExternalSquadCommand.RequestBodySchema.omit({ uuid: true })
)
function ExternalSquadRenameBody(props: IBodyProps) {
    const { mutate, isPending } = useUpdateExternalSquad({
        mutationFns: {
            onSuccess: () => {
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.externalSquads.getExternalSquads.queryKey
                    })
                )
                props.onSaved()
            }
        }
    })
    return (
        <RenameBody
            {...props}
            mutate={mutate}
            isPending={isPending}
            buildVariables={(uuid, name) => ({ uuid, name })}
            validate={validateExternalSquadRenameBody}
        />
    )
}

const validateInternalSquadRenameBody = validateWith(
    UpdateInternalSquadCommand.RequestBodySchema.omit({ uuid: true })
)
function InternalSquadRenameBody(props: IBodyProps) {
    const { mutate, isPending } = useUpdateInternalSquad({
        mutationFns: {
            onSuccess: () => {
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.internalSquads.getInternalSquads.queryKey
                    })
                )
                props.onSaved()
            }
        }
    })
    return (
        <RenameBody
            {...props}
            mutate={mutate}
            isPending={isPending}
            buildVariables={(uuid, name) => ({ uuid, name })}
            validate={validateInternalSquadRenameBody}
        />
    )
}

const validateNodePluginRenameBody = validateWith(
    UpdateNodePluginCommand.RequestBodySchema.omit({ uuid: true })
)
function NodePluginRenameBody(props: IBodyProps) {
    const { mutate, isPending } = useUpdateNodePlugin({
        mutationFns: {
            onSuccess: () => {
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.nodePlugins.getNodePlugins.queryKey
                    })
                )
                props.onSaved()
            }
        }
    })
    return (
        <RenameBody
            {...props}
            mutate={mutate}
            isPending={isPending}
            buildVariables={(uuid, name) => ({ uuid, name })}
            validate={validateNodePluginRenameBody}
        />
    )
}

const validatePasskeyRenameBody = validateWith(
    UpdatePasskeyCommand.RequestBodySchema.omit({ id: true })
)
function PasskeyRenameBody(props: IBodyProps) {
    const { mutate, isPending } = useUpdatePasskey({
        mutationFns: {
            onSuccess: () => {
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.passkeys.getPasskeys.queryKey
                    })
                )
                props.onSaved()
            }
        }
    })
    return (
        <RenameBody
            {...props}
            mutate={mutate}
            isPending={isPending}
            buildVariables={(uuid, name) => ({ id: uuid, name })}
            validate={validatePasskeyRenameBody}
        />
    )
}

const validateSubpageConfigRenameBody = validateWith(
    UpdateSubpageConfigCommand.RequestBodySchema.omit({ uuid: true })
)
function SubpageConfigRenameBody(props: IBodyProps) {
    const { mutate, isPending } = useUpdateSubpageConfig({
        mutationFns: {
            onSuccess: () => {
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.subpageConfigs.getSubpageConfigs.queryKey
                    })
                )
                props.onSaved()
            }
        }
    })
    return (
        <RenameBody
            {...props}
            mutate={mutate}
            isPending={isPending}
            buildVariables={(uuid, name) => ({ uuid, name })}
            validate={validateSubpageConfigRenameBody}
        />
    )
}

const validateTemplateRenameBody = validateWith(
    UpdateSubscriptionTemplateCommand.RequestBodySchema.omit({ uuid: true })
)
function TemplateRenameBody(props: IBodyProps) {
    const { mutate, isPending } = useUpdateSubscriptionTemplate({
        mutationFns: {
            onSuccess: () => {
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.subscriptionTemplate.getSubscriptionTemplates.queryKey
                    })
                )
                props.onSaved()
            }
        }
    })
    return (
        <RenameBody
            {...props}
            mutate={mutate}
            isPending={isPending}
            buildVariables={(uuid, name) => ({ uuid, name })}
            validate={validateTemplateRenameBody}
        />
    )
}

const BODY_BY_KIND: Record<RenameType, (props: IBodyProps) => ReactNode> = {
    configProfile: ConfigProfileRenameBody,
    externalSquad: ExternalSquadRenameBody,
    internalSquad: InternalSquadRenameBody,
    nodePlugin: NodePluginRenameBody,
    passkey: PasskeyRenameBody,
    subpageConfig: SubpageConfigRenameBody,
    template: TemplateRenameBody
}

export const RenameModalShared = NiceModal.create((props: IProps) => {
    const { name, renameFrom, uuid } = props
    const modal = useModal()
    const { modalProps, hide } = useNiceMantineModal({ modal })

    const Body = BODY_BY_KIND[renameFrom]

    return <Body modalProps={modalProps} name={name} onSaved={hide} uuid={uuid} />
})
