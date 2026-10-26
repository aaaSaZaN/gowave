import NiceModal, { useModal } from '@ebay/nice-modal-react'
import { ActionIcon, type ModalProps, TagsInput } from '@mantine/core'
import { TagsSchema } from '@remnawave/backend-contract'
import { useNiceMantineModal } from '@shared/_modals/use-nice-modal'
import {
    QueryKeys,
    useGetConfigProfilesTags,
    useGetExternalSquadsTags,
    useGetInternalSquadsTags,
    useGetNodePluginsTags,
    useGetSubpageConfigsTags,
    useGetSubscriptionTemplatesTags,
    useSetConfigProfilesTags,
    useSetExternalSquadsTags,
    useSetInternalSquadsTags,
    useSetNodePluginsTags,
    useSetSubpageConfigsTags,
    useSetSubscriptionTemplatesTags
} from '@shared/api/hooks'
import { queryClient } from '@shared/api/query-client'
import { optionalProp } from '@shared/lib/optional-prop'
import { runAsync } from '@shared/lib/run-async'
import { CompoundModalShared } from '@shared/ui/compound-modal/compound-modal.shared'
import { BaseOverlayHeader } from '@shared/ui/overlays/base-overlay-header'
import { TagInputPill } from '@shared/ui/tag-input-pill'
import { type ReactNode, useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { TbDeviceFloppy, TbTag, TbTags } from 'react-icons/tb'

type EditTagsType =
    | 'configProfile'
    | 'externalSquad'
    | 'internalSquad'
    | 'nodePlugin'
    | 'subpageConfig'
    | 'template'

interface IProps {
    editTagsFrom: EditTagsType
    tags: string[]
    uuid: string
}

interface IBodyProps {
    modalProps: Omit<ModalProps, 'children' | 'title'>
    onSaved: () => void
    tags: string[]
    uuid: string
}

function TagsBody({
    modalProps,
    tags,
    uuid,
    knownTags,
    isLoadingKnown,
    mutate,
    isPending
}: IBodyProps & {
    knownTags: string[]
    isLoadingKnown: boolean
    isPending: boolean
    mutate: (args: { variables: { tags: string[]; uuid: string } }) => void
}) {
    const { t } = useTranslation()
    const formId = useId()

    const [value, setValue] = useState<string[]>(tags)
    const [error, setError] = useState<null | string>(null)

    const handleChange = (next: string[]) => {
        const normalized = [...new Set(next.map((tag) => tag.trim().toUpperCase()).filter(Boolean))]

        setValue(normalized)

        const result = TagsSchema.safeParse(normalized)
        setError(result.success ? null : (result.error.issues[0]?.message ?? null))
    }

    return (
        <CompoundModalShared
            buttons={
                <ActionIcon
                    color="teal"
                    disabled={!!error}
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
                    IconComponent={TbTags}
                    iconVariant="soft"
                    title={t('common.field.tags')}
                />
            }
        >
            <form
                id={formId}
                onSubmit={(event) => {
                    event.preventDefault()
                    if (TagsSchema.safeParse(value).success) {
                        mutate({ variables: { uuid, tags: value } })
                    }
                }}
            >
                <TagsInput
                    clearable
                    data={knownTags}
                    data-autofocus
                    error={error}
                    leftSection={<TbTag size="1.2rem" />}
                    loading={isLoadingKnown}
                    loadingPosition="left"
                    maxTags={10}
                    onChange={handleChange}
                    placeholder="ENV:PROD"
                    renderPill={({ value: tag, onRemove }) => (
                        <TagInputPill {...optionalProp('onRemove', onRemove)} value={tag} />
                    )}
                    splitChars={[',', ' ', ';']}
                    value={value}
                />
            </form>
        </CompoundModalShared>
    )
}

function ConfigProfileTagsBody(props: IBodyProps) {
    const { data, isLoading } = useGetConfigProfilesTags()
    const { mutate, isPending } = useSetConfigProfilesTags({
        mutationFns: {
            onSuccess: () => {
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.configProfiles.getConfigProfiles.queryKey
                    })
                )
                runAsync(
                    queryClient.invalidateQueries({
                        queryKey: QueryKeys.configProfiles.getConfigProfilesTags.queryKey
                    })
                )
                props.onSaved()
            }
        }
    })
    return (
        <TagsBody
            {...props}
            knownTags={data?.tags ?? []}
            isLoadingKnown={isLoading}
            mutate={mutate}
            isPending={isPending}
        />
    )
}

function ExternalSquadTagsBody(props: IBodyProps) {
    const { data, isLoading } = useGetExternalSquadsTags()
    const { mutate, isPending } = useSetExternalSquadsTags({
        mutationFns: {
            onSuccess: () => {
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.externalSquads.getExternalSquads.queryKey
                    })
                )
                runAsync(
                    queryClient.invalidateQueries({
                        queryKey: QueryKeys.externalSquads.getExternalSquadsTags.queryKey
                    })
                )
                props.onSaved()
            }
        }
    })
    return (
        <TagsBody
            {...props}
            knownTags={data?.tags ?? []}
            isLoadingKnown={isLoading}
            mutate={mutate}
            isPending={isPending}
        />
    )
}

function InternalSquadTagsBody(props: IBodyProps) {
    const { data, isLoading } = useGetInternalSquadsTags()
    const { mutate, isPending } = useSetInternalSquadsTags({
        mutationFns: {
            onSuccess: () => {
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.internalSquads.getInternalSquads.queryKey
                    })
                )
                runAsync(
                    queryClient.invalidateQueries({
                        queryKey: QueryKeys.internalSquads.getInternalSquadsTags.queryKey
                    })
                )
                props.onSaved()
            }
        }
    })
    return (
        <TagsBody
            {...props}
            knownTags={data?.tags ?? []}
            isLoadingKnown={isLoading}
            mutate={mutate}
            isPending={isPending}
        />
    )
}

function NodePluginTagsBody(props: IBodyProps) {
    const { data, isLoading } = useGetNodePluginsTags()
    const { mutate, isPending } = useSetNodePluginsTags({
        mutationFns: {
            onSuccess: () => {
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.nodePlugins.getNodePlugins.queryKey
                    })
                )
                runAsync(
                    queryClient.invalidateQueries({
                        queryKey: QueryKeys.nodePlugins.getNodePluginsTags.queryKey
                    })
                )
                props.onSaved()
            }
        }
    })
    return (
        <TagsBody
            {...props}
            knownTags={data?.tags ?? []}
            isLoadingKnown={isLoading}
            mutate={mutate}
            isPending={isPending}
        />
    )
}

function SubpageConfigTagsBody(props: IBodyProps) {
    const { data, isLoading } = useGetSubpageConfigsTags()
    const { mutate, isPending } = useSetSubpageConfigsTags({
        mutationFns: {
            onSuccess: () => {
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.subpageConfigs.getSubpageConfigs.queryKey
                    })
                )
                runAsync(
                    queryClient.invalidateQueries({
                        queryKey: QueryKeys.subpageConfigs.getSubpageConfigsTags.queryKey
                    })
                )
                props.onSaved()
            }
        }
    })
    return (
        <TagsBody
            {...props}
            knownTags={data?.tags ?? []}
            isLoadingKnown={isLoading}
            mutate={mutate}
            isPending={isPending}
        />
    )
}

function TemplateTagsBody(props: IBodyProps) {
    const { data, isLoading } = useGetSubscriptionTemplatesTags()
    const { mutate, isPending } = useSetSubscriptionTemplatesTags({
        mutationFns: {
            onSuccess: () => {
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.subscriptionTemplate.getSubscriptionTemplates.queryKey
                    })
                )
                runAsync(
                    queryClient.invalidateQueries({
                        queryKey:
                            QueryKeys.subscriptionTemplate.getSubscriptionTemplatesTags.queryKey
                    })
                )
                props.onSaved()
            }
        }
    })
    return (
        <TagsBody
            {...props}
            knownTags={data?.tags ?? []}
            isLoadingKnown={isLoading}
            mutate={mutate}
            isPending={isPending}
        />
    )
}

const BODY_BY_KIND: Record<EditTagsType, (props: IBodyProps) => ReactNode> = {
    configProfile: ConfigProfileTagsBody,
    externalSquad: ExternalSquadTagsBody,
    internalSquad: InternalSquadTagsBody,
    nodePlugin: NodePluginTagsBody,
    subpageConfig: SubpageConfigTagsBody,
    template: TemplateTagsBody
}

export const EditTagsModalShared = NiceModal.create((props: IProps) => {
    const { editTagsFrom, tags, uuid } = props
    const modal = useModal()
    const { modalProps, hide } = useNiceMantineModal({ modal })

    const Body = BODY_BY_KIND[editTagsFrom]

    return <Body modalProps={modalProps} onSaved={hide} tags={tags} uuid={uuid} />
})
