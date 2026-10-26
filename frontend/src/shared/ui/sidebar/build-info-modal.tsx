import {
    ActionIcon,
    Alert,
    Anchor,
    Badge,
    Box,
    Button,
    Code,
    CopyButton,
    Group,
    Paper,
    SegmentedControl,
    Stack,
    Text,
    Tooltip
} from '@mantine/core'
import { GetMetadataCommand } from '@remnawave/backend-contract'
import { useState } from 'react'
import {
    TbAlertTriangle,
    TbBrandGithub,
    TbCheck,
    TbCopy,
    TbDownload,
    TbGitBranch,
    TbGitCommit
} from 'react-icons/tb'

import { instance } from '@shared/api'

import { Logo } from '../logo'
import classes from './build-info-modal.module.css'
import { UpdateChannel, usePanelUpdateCheck } from './panel-update-check'

interface BuildInfoModalProps {
    gowaveMetadata: GetMetadataCommand.Response['response']
}

export function BuildInfoModal({ gowaveMetadata }: BuildInfoModalProps) {
    const currentChannel: UpdateChannel =
        gowaveMetadata.git?.backend?.branch === 'dev' ? 'dev' : 'stable'
    const [targetChannel, setTargetChannel] = useState<UpdateChannel>(currentChannel)
    const [isUpdating, setIsUpdating] = useState(false)
    const isSwitching = targetChannel !== currentChannel

    // Docker installs and native installs without the root update helper are
    // updated on the server; the panel only shows the command.
    const updateCheck = usePanelUpdateCheck(isSwitching ? targetChannel : undefined)
    const hasUpdate = Boolean(updateCheck.data?.hasUpdate)
    const serverUpdateCommand =
        updateCheck.data && !updateCheck.data.canAutoUpdate
            ? updateCheck.data.updateCommand
            : undefined
    const commitSha = gowaveMetadata.git?.backend?.commitSha

    const handleUpdate = async () => {
        setIsUpdating(true)
        try {
            // A root helper downloads and verifies the channel's newest build,
            // then restarts the panel.
            await instance.post('/api/system/updates/apply', { channel: targetChannel })
            setTimeout(() => {
                window.location.reload()
            }, 30000)
        } catch {
            setIsUpdating(false)
        }
    }

    let updateTitle = 'Update available'
    let updateDescription = `${updateCheck.data?.latestVersion ?? ''} is available`
    if (isSwitching) {
        updateTitle = targetChannel === 'dev' ? 'Switch to dev' : 'Switch to stable'
        updateDescription = `Installs ${updateCheck.data?.latestVersion ?? ''}`
    }

    return (
        <Stack gap="md">
            <Stack gap={6}>
                <Text c="dimmed" size="xs">
                    Update channel
                </Text>
                <SegmentedControl
                    data={[
                        { label: 'Stable (releases)', value: 'stable' },
                        { label: 'Dev (every commit)', value: 'dev' }
                    ]}
                    disabled={isUpdating}
                    fullWidth
                    onChange={(value) => setTargetChannel(value as UpdateChannel)}
                    radius="md"
                    size="xs"
                    value={targetChannel}
                />
            </Stack>

            {isSwitching && (
                <Alert
                    color={targetChannel === 'dev' ? 'red' : 'yellow'}
                    icon={<TbAlertTriangle size={16} />}
                    radius="md"
                    variant="light"
                >
                    <Text size="xs">
                        {targetChannel === 'dev'
                            ? 'Dev builds are made from every commit to main and may be unstable. Make a backup first.'
                            : 'The latest stable release may be older than this dev build. Database changes made by newer dev builds stay in place; make a backup first.'}
                    </Text>
                </Alert>
            )}

            {updateCheck.isError && (
                <Text c="dimmed" size="xs">
                    Could not check for updates.
                </Text>
            )}

            {!updateCheck.isLoading && !updateCheck.isError && !hasUpdate && (
                <Text c="dimmed" size="xs">
                    You are running the latest {currentChannel} build.
                </Text>
            )}

            {hasUpdate && (
                <Paper className={classes.updateCard} p="md" radius="md">
                    <Group align="center" gap="md" wrap="wrap">
                        <Group gap="sm" wrap="nowrap">
                            <Box className={classes.updateIconBox}>
                                <Logo color="var(--mantine-color-teal-4)" size={24} />
                            </Box>
                            <Stack className={classes.updateTextWrapper} gap={4}>
                                <Text c="teal.4" fw={600} size="sm">
                                    {updateTitle}
                                </Text>
                                <Text c="dimmed" size="xs">
                                    {updateDescription}
                                </Text>
                            </Stack>
                        </Group>

                        {!serverUpdateCommand && (
                            <Button
                                color="teal"
                                disabled={updateCheck.isLoading}
                                leftSection={<TbDownload size={14} />}
                                ml="auto"
                                radius="md"
                                size="xs"
                                variant="light"
                                loading={isUpdating}
                                onClick={handleUpdate}
                            >
                                {isSwitching ? 'Switch' : 'Update panel'}
                            </Button>
                        )}
                    </Group>

                    {serverUpdateCommand && (
                        <Stack gap={6} mt="sm">
                            <Text c="dimmed" size="xs">
                                {updateCheck.data?.isDocker
                                    ? 'The panel runs in Docker. Update it on the server:'
                                    : 'Update the panel on the server:'}
                            </Text>
                            <Group gap="xs" wrap="nowrap">
                                <Code block style={{ flex: 1, whiteSpace: 'pre-wrap' }}>
                                    {serverUpdateCommand}
                                </Code>
                                <CopyButton timeout={2000} value={serverUpdateCommand}>
                                    {({ copied, copy }) => (
                                        <Tooltip label={copied ? 'Copied' : 'Copy command'}>
                                            <ActionIcon
                                                color={copied ? 'teal' : 'gray'}
                                                onClick={copy}
                                                size="md"
                                                variant="subtle"
                                            >
                                                {copied ? (
                                                    <TbCheck size={14} />
                                                ) : (
                                                    <TbCopy size={14} />
                                                )}
                                            </ActionIcon>
                                        </Tooltip>
                                    )}
                                </CopyButton>
                            </Group>
                        </Stack>
                    )}
                </Paper>
            )}

            <Paper className={classes.mainCard} p="md">
                <Group justify="space-between">
                    <Group gap="sm">
                        <Badge
                            color="cyan"
                            leftSection={<Logo size={16} />}
                            size="lg"
                            variant="light"
                        >
                            {gowaveMetadata.version}
                        </Badge>

                        <Badge
                            color={gowaveMetadata.git?.backend?.branch === 'dev' ? 'red' : 'teal'}
                            leftSection={<TbGitBranch size={16} />}
                            size="lg"
                            variant="light"
                        >
                            {gowaveMetadata.git?.backend?.branch || 'main'}
                        </Badge>
                    </Group>
                    <CopyButton timeout={2000} value={JSON.stringify(gowaveMetadata, null, 2)}>
                        {({ copied, copy }) => (
                            <Tooltip label="Copy build info">
                                <ActionIcon
                                    color={copied ? 'teal' : 'gray'}
                                    onClick={copy}
                                    size="md"
                                    variant="subtle"
                                >
                                    {copied ? <TbCheck size={14} /> : <TbCopy size={14} />}
                                </ActionIcon>
                            </Tooltip>
                        )}
                    </CopyButton>
                </Group>
                {commitSha && (
                    <Group gap={6} mt="sm" wrap="nowrap">
                        <TbGitCommit size={14} />
                        <Anchor
                            ff="monospace"
                            href={gowaveMetadata.git?.backend?.commitUrl}
                            size="xs"
                            target="_blank"
                        >
                            {commitSha.slice(0, 7)}
                        </Anchor>
                    </Group>
                )}
            </Paper>

            <Group gap="sm" grow>
                <Button
                    component="a"
                    href="https://github.com/aaaSaZaN/gowave"
                    leftSection={<TbBrandGithub size={16} />}
                    radius="md"
                    size="sm"
                    target="_blank"
                    variant="default"
                >
                    GitHub
                </Button>
            </Group>
        </Stack>
    )
}
