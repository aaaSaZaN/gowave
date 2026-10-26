import {
    ActionIcon,
    Badge,
    Box,
    Button,
    Code,
    CopyButton,
    Group,
    Paper,
    Stack,
    Text,
    Tooltip
} from '@mantine/core'
import type { GetMetadataCommand } from '@remnawave/backend-contract'
import { instance } from '@shared/api'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { TbBrandGithub, TbCheck, TbCopy, TbDownload, TbGitBranch } from 'react-icons/tb'

import { Logo } from '../logo'
import classes from './build-info-modal.module.css'

interface BuildInfoModalProps {
    isNewVersionAvailable: boolean
    gowaveMetadata: GetMetadataCommand.Response['response']
}

interface IPanelUpdateCheck {
    canAutoUpdate: boolean
    isDocker: boolean
    updateCommand?: string
}

export function BuildInfoModal({ gowaveMetadata, isNewVersionAvailable }: BuildInfoModalProps) {
    const [isUpdating, setIsUpdating] = useState(false)

    // Docker installs and native installs without the root update helper are
    // updated on the server; the panel only shows the command.
    const updateCheck = useQuery({
        queryKey: ['panel-update-check'],
        queryFn: async () => {
            const res = await instance.get<IPanelUpdateCheck>('/api/system/updates/check')
            return res.data
        },
        enabled: isNewVersionAvailable,
        refetchOnWindowFocus: false
    })
    const serverUpdateCommand =
        updateCheck.data && !updateCheck.data.canAutoUpdate
            ? updateCheck.data.updateCommand
            : undefined

    const handleUpdate = async () => {
        setIsUpdating(true)
        try {
            // A root helper downloads and verifies the release, then restarts the panel.
            await instance.post('/api/system/updates/apply')
            setTimeout(() => {
                window.location.reload()
            }, 30000)
        } catch {
            setIsUpdating(false)
        }
    }

    return (
        <Stack gap="md">
            {isNewVersionAvailable && (
                <Paper className={classes.updateCard} p="md" radius="md">
                    <Group align="center" gap="md" wrap="wrap">
                        <Group gap="sm" wrap="nowrap">
                            <Box className={classes.updateIconBox}>
                                <Logo color="var(--mantine-color-teal-4)" size={24} />
                            </Box>
                            <Stack className={classes.updateTextWrapper} gap={4}>
                                <Text c="teal.4" fw={600} size="sm">
                                    Update available
                                </Text>
                                <Text c="dimmed" size="xs">
                                    A new version is available
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
                                Update panel
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
