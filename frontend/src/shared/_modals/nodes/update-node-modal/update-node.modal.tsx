import NiceModal, { useModal } from '@ebay/nice-modal-react'
import {
    Alert,
    Badge,
    Button,
    Checkbox,
    Divider,
    Group,
    Loader,
    Modal,
    Select,
    Stack,
    Tabs,
    Text,
    TextInput
} from '@mantine/core'
import { GetNodeCommand } from '@remnawave/backend-contract'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { TbAlertTriangle, TbArrowRight, TbCheck, TbCloudDownload, TbCpu } from 'react-icons/tb'

import { useNiceMantineModal } from '@shared/_modals/use-nice-modal'
import { instance } from '@shared/api/axios'
import { BaseOverlayHeader } from '@shared/ui/overlays/base-overlay-header'

interface IProps {
    node: GetNodeCommand.Response['response']
    initialTab?: 'node' | 'core'
}

interface IReleaseAsset {
    name: string
    browser_download_url: string
    size: number
}

interface IReleaseInfo {
    tag_name: string
    name: string
    prerelease: boolean
    published_at: string
    assets: IReleaseAsset[]
}

interface IUpdatesCheckResponse {
    currentVersion: string
    latestRelease?: IReleaseInfo
    latestPrerelease?: IReleaseInfo
    allReleases: IReleaseInfo[]
    architecture: string
    os: string
    currentCoreVersion: string
    error?: string
}

type ReleaseChannel = 'stable' | 'prerelease' | 'version'

const NODE_REPOSITORY = 'aaaSaZaN/gowave-node'
const XRAY_REPOSITORY = 'XTLS/Xray-core'

const archAliases: Record<string, string[]> = {
    amd64: ['amd64', 'x86_64', 'linux-64', '64bit'],
    arm64: ['arm64-v8a', 'arm64v8', 'arm64', 'aarch64'],
    arm: ['arm32-v7a', 'armv7', 'arm32-v6', 'armv6', 'armhf', 'arm'],
    mipsle: ['mips32le', 'mipsle', 'mipsel'],
    mips: ['mips32', 'mips'],
    mips64le: ['mips64le'],
    mips64: ['mips64'],
    '386': ['linux-32', '386', 'i386', '32bit', 'x86']
}

function getCompatibleAsset(
    release: IReleaseInfo,
    os: string,
    architecture: string,
    binaryName: 'gowave-node' | 'rw-core'
) {
    const aliases = archAliases[architecture] || [architecture]
    const assets = release.assets || []
    const matches = (asset: IReleaseAsset, alias: string, requireOS: boolean) => {
        const name = asset.name.toLowerCase()
        if (
            ['.dgst', '.sha256', '.sha512', '.sha1', '.sig', '.asc', '.txt', '.json'].some(
                (suffix) => name.endsWith(suffix)
            )
        ) {
            return false
        }
        if (requireOS && !name.includes(os.toLowerCase())) return false
        if (
            (architecture === 'mips' || architecture === 'mips64') &&
            (name.includes('le') || name.includes('el'))
        ) {
            return false
        }
        return (
            name.includes(alias.toLowerCase()) &&
            (binaryName === 'rw-core' || name.includes(binaryName))
        )
    }

    for (const alias of aliases) {
        const asset = assets.find((candidate) => matches(candidate, alias, true))
        if (asset) return asset
    }
    for (const alias of aliases) {
        const asset = assets.find((candidate) => matches(candidate, alias, false))
        if (asset) return asset
    }
    return undefined
}

function getErrorMessage(error: unknown) {
    const value = error as {
        message?: string
        response?: { data?: { message?: string; error?: string } }
    }
    return (
        value?.response?.data?.message ||
        value?.response?.data?.error ||
        value?.message ||
        'Не удалось выполнить обновление.'
    )
}

export const UpdateNodeModal = NiceModal.create((props: IProps) => {
    const { node, initialTab = 'node' } = props
    const modal = useModal()
    const { modalProps, hide } = useNiceMantineModal({ modal })

    const [activeTab, setActiveTab] = useState<'node' | 'core'>(initialTab)
    const [nodeChannel, setNodeChannel] = useState<ReleaseChannel>('stable')
    const [selectedNodeVersion, setSelectedNodeVersion] = useState<string | null>(null)
    const [coreChannel, setCoreChannel] = useState<ReleaseChannel>('stable')
    const [selectedCoreVersion, setSelectedCoreVersion] = useState<string | null>(null)
    const [coreDownloadUrl, setCoreDownloadUrl] = useState('')
    const [confirmed, setConfirmed] = useState(false)

    useEffect(() => {
        if (modal.visible) {
            setActiveTab(initialTab)
            setConfirmed(false)
        }
    }, [initialTab, modal.visible])

    const nodeUpdatesQuery = useQuery({
        queryKey: ['node-updates-check', node.uuid, NODE_REPOSITORY],
        queryFn: async () => {
            const res = await instance.get<{ response: IUpdatesCheckResponse }>(
                `/api/nodes/${node.uuid}/updates` as any,
                { params: { repo: NODE_REPOSITORY } }
            )
            return res.data.response
        },
        enabled: activeTab === 'node',
        refetchOnWindowFocus: false
    })

    const coreUpdatesQuery = useQuery({
        queryKey: ['node-updates-check', node.uuid, XRAY_REPOSITORY],
        queryFn: async () => {
            const res = await instance.get<{ response: IUpdatesCheckResponse }>(
                `/api/nodes/${node.uuid}/updates` as any,
                { params: { repo: XRAY_REPOSITORY } }
            )
            return res.data.response
        },
        enabled: activeTab === 'core',
        refetchOnWindowFocus: false
    })

    const nodeReleases = nodeUpdatesQuery.data?.allReleases || []
    const coreReleases = coreUpdatesQuery.data?.allReleases || []
    const nodeOptions = useMemo(
        () =>
            nodeReleases
                .filter((release) =>
                    getCompatibleAsset(
                        release,
                        nodeUpdatesQuery.data?.os || 'linux',
                        nodeUpdatesQuery.data?.architecture || '',
                        'gowave-node'
                    )
                )
                .map((release) => ({
                    value: release.tag_name,
                    label: `${release.tag_name} · ${release.prerelease ? 'Pre-release' : 'Release'}`
                })),
        [nodeReleases, nodeUpdatesQuery.data?.architecture, nodeUpdatesQuery.data?.os]
    )
    const coreOptions = useMemo(
        () =>
            coreReleases
                .filter((release) =>
                    getCompatibleAsset(
                        release,
                        coreUpdatesQuery.data?.os || 'linux',
                        coreUpdatesQuery.data?.architecture || '',
                        'rw-core'
                    )
                )
                .map((release) => ({
                    value: release.tag_name,
                    label: `${release.tag_name} · ${release.prerelease ? 'Pre-release' : 'Release'}`
                })),
        [coreReleases, coreUpdatesQuery.data?.architecture, coreUpdatesQuery.data?.os]
    )

    const latestNodeStable = nodeReleases.find(
        (release) =>
            !release.prerelease &&
            getCompatibleAsset(
                release,
                nodeUpdatesQuery.data?.os || 'linux',
                nodeUpdatesQuery.data?.architecture || '',
                'gowave-node'
            )
    )
    const latestNodePrerelease = nodeReleases.find(
        (release) =>
            release.prerelease &&
            getCompatibleAsset(
                release,
                nodeUpdatesQuery.data?.os || 'linux',
                nodeUpdatesQuery.data?.architecture || '',
                'gowave-node'
            )
    )
    const latestCoreStable = coreReleases.find(
        (release) =>
            !release.prerelease &&
            getCompatibleAsset(
                release,
                coreUpdatesQuery.data?.os || 'linux',
                coreUpdatesQuery.data?.architecture || '',
                'rw-core'
            )
    )
    const latestCorePrerelease = coreReleases.find(
        (release) =>
            release.prerelease &&
            getCompatibleAsset(
                release,
                coreUpdatesQuery.data?.os || 'linux',
                coreUpdatesQuery.data?.architecture || '',
                'rw-core'
            )
    )

    const nodeTargetVersion =
        nodeChannel === 'version'
            ? selectedNodeVersion || ''
            : (nodeChannel === 'stable'
                  ? latestNodeStable?.tag_name
                  : latestNodePrerelease?.tag_name) || ''
    const coreTargetVersion =
        coreChannel === 'version'
            ? selectedCoreVersion || ''
            : (coreChannel === 'stable'
                  ? latestCoreStable?.tag_name
                  : latestCorePrerelease?.tag_name) || ''

    const applyMutation = useMutation({
        mutationFn: async () => {
            if (activeTab === 'node') {
                if (!nodeTargetVersion)
                    throw new Error('Нет подходящего релиза GoWave Node для этой архитектуры.')
                const res = await instance.post(`/api/nodes/${node.uuid}/updates/apply` as any, {
                    type: 'node',
                    repo: NODE_REPOSITORY,
                    version: nodeTargetVersion
                })
                return res.data
            }

            const downloadUrl = coreDownloadUrl.trim()
            if (downloadUrl) {
                if (!downloadUrl.startsWith('https://')) {
                    throw new Error('Ссылка должна начинаться с https://')
                }
                const res = await instance.post(`/api/nodes/${node.uuid}/updates/apply` as any, {
                    type: 'core',
                    downloadUrl
                })
                return res.data
            }

            if (!coreTargetVersion)
                throw new Error('Нет подходящего релиза Xray для этой архитектуры.')
            const res = await instance.post(`/api/nodes/${node.uuid}/updates/apply` as any, {
                type: 'core',
                repo: XRAY_REPOSITORY,
                version: coreTargetVersion
            })
            return res.data
        }
    })

    const isLoading = activeTab === 'node' ? nodeUpdatesQuery.isLoading : coreUpdatesQuery.isLoading
    const updateInfo = activeTab === 'node' ? nodeUpdatesQuery.data : coreUpdatesQuery.data
    const queryError =
        activeTab === 'node'
            ? nodeUpdatesQuery.error || updateInfo?.error
            : coreUpdatesQuery.error || updateInfo?.error
    const targetVersion =
        activeTab === 'node'
            ? nodeTargetVersion
            : coreDownloadUrl.trim()
              ? 'По ссылке'
              : coreTargetVersion
    const canUpdate =
        activeTab === 'node'
            ? Boolean(nodeTargetVersion)
            : Boolean(coreDownloadUrl.trim() || coreTargetVersion)

    const releaseSelector = (
        channel: ReleaseChannel,
        setChannel: (value: ReleaseChannel) => void,
        selectedVersion: string | null,
        setSelectedVersion: (value: string | null) => void,
        options: { value: string; label: string }[],
        targetVersion: string,
        versionLabel: string
    ) => (
        <Stack gap="xs">
            <Select
                data={[
                    {
                        value: 'stable',
                        label: `Последний Release${channel === 'stable' && targetVersion ? ` · ${targetVersion}` : ''}`
                    },
                    {
                        value: 'prerelease',
                        label: `Последний Pre-release${channel === 'prerelease' && targetVersion ? ` · ${targetVersion}` : ''}`
                    },
                    { value: 'version', label: 'Выбрать конкретную версию…' }
                ]}
                label="Канал релиза"
                onChange={(value) => {
                    setChannel((value as ReleaseChannel) || 'stable')
                    setConfirmed(false)
                    applyMutation.reset()
                }}
                value={channel}
            />
            {channel === 'version' && (
                <Select
                    data={options}
                    label={versionLabel}
                    onChange={(value) => {
                        setSelectedVersion(value)
                        setConfirmed(false)
                        applyMutation.reset()
                    }}
                    placeholder="Выберите release или pre-release"
                    searchable
                    value={selectedVersion}
                />
            )}
        </Stack>
    )

    return (
        <Modal
            {...modalProps}
            centered
            size="min(640px, 95vw)"
            title={
                <BaseOverlayHeader
                    iconColor="blue"
                    IconComponent={TbCloudDownload}
                    iconVariant="soft"
                    subtitle={node.name}
                    title="Управление версиями ноды"
                />
            }
        >
            <Stack gap="md">
                <Alert
                    color="orange"
                    icon={<TbAlertTriangle size={20} />}
                    radius="md"
                    title="Перезапуск службы"
                    variant="light"
                >
                    Установка GoWave Node или Xray перезапустит службу. Активные соединения
                    кратковременно прервутся.
                </Alert>

                <Tabs
                    onChange={(value) => {
                        setActiveTab((value as 'node' | 'core') || 'node')
                        setConfirmed(false)
                        applyMutation.reset()
                    }}
                    value={activeTab}
                >
                    <Tabs.List grow>
                        <Tabs.Tab value="node">GoWave Node Go</Tabs.Tab>
                        <Tabs.Tab leftSection={<TbCpu size={16} />} value="core">
                            Xray Core
                        </Tabs.Tab>
                    </Tabs.List>
                </Tabs>

                {isLoading ? (
                    <Group justify="center" p="xl">
                        <Loader color="blue" size="sm" />
                        <Text c="dimmed" size="xs">
                            Загрузка списка релизов…
                        </Text>
                    </Group>
                ) : (
                    <>
                        {queryError && (
                            <Alert color="red" title="Не удалось получить релизы" variant="light">
                                {typeof queryError === 'string'
                                    ? queryError
                                    : getErrorMessage(queryError)}
                            </Alert>
                        )}

                        <Group justify="space-between">
                            <Group gap="xs">
                                <Text fw={500} size="sm">
                                    Установлено:
                                </Text>
                                <Badge color="gray" size="md" variant="light">
                                    {activeTab === 'node'
                                        ? updateInfo?.currentVersion || 'не получена'
                                        : updateInfo?.currentCoreVersion ||
                                          node.versions?.xray ||
                                          'не получена'}
                                </Badge>
                            </Group>
                            <Badge color="dark" size="sm" variant="outline">
                                {updateInfo?.os || 'linux'}/{updateInfo?.architecture || '—'}
                            </Badge>
                        </Group>

                        <Divider opacity={0.4} />

                        {activeTab === 'node' ? (
                            <Stack gap="sm">
                                <Text fw={600} size="sm">
                                    Выберите релиз GoWave Node Go
                                </Text>
                                {releaseSelector(
                                    nodeChannel,
                                    setNodeChannel,
                                    selectedNodeVersion,
                                    setSelectedNodeVersion,
                                    nodeOptions,
                                    nodeTargetVersion,
                                    'Релиз GoWave Node Go'
                                )}
                            </Stack>
                        ) : (
                            <Stack gap="sm">
                                <Text fw={600} size="sm">
                                    Выберите официальный релиз Xray Core
                                </Text>
                                {releaseSelector(
                                    coreChannel,
                                    setCoreChannel,
                                    selectedCoreVersion,
                                    setSelectedCoreVersion,
                                    coreOptions,
                                    coreTargetVersion,
                                    'Релиз Xray Core'
                                )}

                                <TextInput
                                    description="Если ссылка указана, она имеет приоритет над выбранным релизом. Поддерживается прямой бинарник, ZIP или TAR.GZ."
                                    label="Прямая HTTPS-ссылка на ядро"
                                    onChange={(event) => {
                                        setCoreDownloadUrl(event.currentTarget.value)
                                        setConfirmed(false)
                                        applyMutation.reset()
                                    }}
                                    placeholder="https://example.org/Xray-linux-64.zip"
                                    value={coreDownloadUrl}
                                />
                            </Stack>
                        )}

                        <Group gap="xs">
                            <Text c="dimmed" size="xs">
                                Установка:
                            </Text>
                            <Badge color="gray" size="sm" variant="outline">
                                {activeTab === 'node'
                                    ? updateInfo?.currentVersion || 'текущая версия'
                                    : updateInfo?.currentCoreVersion ||
                                      node.versions?.xray ||
                                      'текущая версия'}
                            </Badge>
                            <TbArrowRight size={14} />
                            <Badge
                                color={targetVersion ? 'blue' : 'gray'}
                                size="sm"
                                variant="filled"
                            >
                                {targetVersion || 'выберите релиз'}
                            </Badge>
                        </Group>

                        <Divider opacity={0.4} />

                        <Checkbox
                            checked={confirmed}
                            color="orange"
                            label={`Подтверждаю установку ${activeTab === 'node' ? 'GoWave Node Go' : 'Xray Core'} и перезапуск службы`}
                            onChange={(event) => setConfirmed(event.currentTarget.checked)}
                            size="sm"
                        />

                        {applyMutation.isSuccess && (
                            <Alert color="teal" icon={<TbCheck size={18} />} title="Запрос принят">
                                {activeTab === 'node'
                                    ? `GoWave Node обновляется до ${nodeTargetVersion}.`
                                    : `Xray Core обновляется${coreDownloadUrl.trim() ? ' по указанной ссылке' : ` до ${coreTargetVersion}`}.`}{' '}
                                Служба перезапускается.
                            </Alert>
                        )}
                        {applyMutation.isError && (
                            <Alert color="red" title="Обновление не выполнено" variant="light">
                                {getErrorMessage(applyMutation.error)}
                            </Alert>
                        )}

                        <Group justify="flex-end" mt="sm">
                            <Button
                                disabled={applyMutation.isPending}
                                onClick={hide}
                                size="sm"
                                variant="subtle"
                            >
                                Закрыть
                            </Button>
                            <Button
                                color="orange"
                                disabled={
                                    !confirmed ||
                                    !canUpdate ||
                                    applyMutation.isPending ||
                                    Boolean(
                                        queryError &&
                                        (activeTab === 'node' || !coreDownloadUrl.trim())
                                    )
                                }
                                leftSection={<TbCloudDownload size={16} />}
                                loading={applyMutation.isPending}
                                onClick={() => applyMutation.mutate()}
                                size="sm"
                            >
                                {applyMutation.isPending ? 'Установка…' : 'Установить сейчас'}
                            </Button>
                        </Group>
                    </>
                )}
            </Stack>
        </Modal>
    )
})
