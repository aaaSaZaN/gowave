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
    Loader,
    Paper,
    Progress,
    ScrollArea,
    SegmentedControl,
    Stack,
    Text,
    Tooltip
} from '@mantine/core'
import type { GetMetadataCommand } from '@remnawave/backend-contract'
import { prettifyBytesUtil } from '@shared/utils/bytes'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import {
    TbAlertTriangle,
    TbBrandGithub,
    TbCheck,
    TbCopy,
    TbDownload,
    TbGitBranch,
    TbGitCommit,
    TbRefresh
} from 'react-icons/tb'

import { Logo } from '../logo'
import classes from './build-info-modal.module.css'
import {
    fetchPanelUpdateStatus,
    getUpdateErrorMessage,
    type IPanelUpdateLogLine,
    type IPanelUpdateStatus,
    type IPanelUpdateStatusResponse,
    requestPanelUpdate,
    type UpdateChannel,
    useForceUpdateCheck,
    usePanelUpdateCheck
} from './panel-update-check'

interface PanelUpdateRequest {
    channel: UpdateChannel
    fromVersion: string
    // Server time of the request; statuses that started earlier belong to older updates.
    requestedAt: string
}

const POLL_INTERVAL = 1500
const HELPER_START_TIMEOUT = 30 * 1000
const LEGACY_GRACE = 15 * 1000
const UNREACHABLE_TIMEOUT = 3 * 60 * 1000
const UPDATE_TIMEOUT = 16 * 60 * 1000
const RELOAD_DELAY = 3000

const JOURNAL_COMMAND = 'journalctl -u gowave-update.service -n 100 --no-pager'
const HELPER_COMMAND = 'systemctl status gowave-update.path gowave-update.service'
const PANEL_COMMAND = 'journalctl -u gowave.service -n 100 --no-pager'

const STEP_LABELS: Record<string, string> = {
    starting: 'Starting',
    download: 'Downloading',
    verify: 'Verifying checksums',
    frontend: 'Installing frontend',
    binary: 'Installing backend',
    service: 'Updating service',
    restart: 'Restarting panel',
    done: 'Done'
}

interface PollState {
    lastError: null | string
    now: number
    response: IPanelUpdateStatusResponse | null
    unreachableSince: null | number
}

type UpdateView =
    | { command?: string; error: string; kind: 'failed' }
    | { kind: 'done'; version: string }
    | { kind: 'helperIdle' }
    | { kind: 'legacy' }
    | { kind: 'restarting' }
    | { kind: 'running'; status: IPanelUpdateStatus }
    | { kind: 'waiting' }

function belongsToRequest(
    status: IPanelUpdateStatus | null | undefined,
    requestedAt: string
): status is IPanelUpdateStatus {
    if (!status) return false
    // The root helper writes the status only after it picked up the request.
    return Date.parse(status.startedAt) >= Date.parse(requestedAt) - 2000
}

function deriveUpdateView(
    request: PanelUpdateRequest,
    poll: PollState,
    startedLocally: number
): UpdateView {
    const { response } = poll
    const own = belongsToRequest(response?.status, request.requestedAt)
        ? response?.status
        : undefined
    const elapsed = poll.now - startedLocally

    if (own?.state === 'failed') {
        return {
            kind: 'failed',
            error: own.error || own.message || 'The update failed',
            command: JOURNAL_COMMAND
        }
    }
    if (own?.state === 'done') {
        return { kind: 'done', version: own.installedVersion || response?.currentVersion || '' }
    }
    if (response && response.currentVersion !== request.fromVersion) {
        return { kind: 'done', version: response.currentVersion }
    }
    if (poll.unreachableSince !== null) {
        if (poll.now - poll.unreachableSince > UNREACHABLE_TIMEOUT) {
            return {
                kind: 'failed',
                error: `The panel has not answered for 3 minutes (${poll.lastError ?? 'no response'}). It may have failed to start after the update.`,
                command: PANEL_COMMAND
            }
        }
        return { kind: 'restarting' }
    }
    if (elapsed > UPDATE_TIMEOUT) {
        return {
            kind: 'failed',
            error: 'The update did not finish in time and the panel version did not change.',
            command: JOURNAL_COMMAND
        }
    }
    if (own) return { kind: 'running', status: own }
    if (!response || response.requestPending) {
        return elapsed > HELPER_START_TIMEOUT ? { kind: 'helperIdle' } : { kind: 'waiting' }
    }
    // The request was taken, but the installed helper predates progress reports.
    return elapsed > LEGACY_GRACE ? { kind: 'legacy' } : { kind: 'waiting' }
}

function isFinished(view: UpdateView) {
    return view.kind === 'done' || view.kind === 'failed'
}

function CommandLine({ command }: { command: string }) {
    return (
        <Group gap="xs" wrap="nowrap">
            <Code block style={{ flex: 1, whiteSpace: 'pre-wrap' }}>
                {command}
            </Code>
            <CopyButton timeout={2000} value={command}>
                {({ copied, copy }) => (
                    <Tooltip label={copied ? 'Copied' : 'Copy command'}>
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
    )
}

function formatLogTime(time: string) {
    const date = new Date(time)
    return Number.isNaN(date.getTime()) ? '' : date.toLocaleTimeString()
}

function UpdateLog({ log }: { log: IPanelUpdateLogLine[] }) {
    const viewport = useRef<HTMLDivElement>(null)
    const lines = log.length

    useEffect(() => {
        if (lines > 0) {
            viewport.current?.scrollTo({ top: viewport.current.scrollHeight })
        }
    }, [lines])

    if (lines === 0) return null
    return (
        <ScrollArea.Autosize mah={200} type="auto" viewportRef={viewport}>
            <Code block style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                {log.map((line) => (
                    <Text
                        {...(line.level === 'error' ? { c: 'red.5' } : {})}
                        ff="monospace"
                        key={`${line.time}-${line.level}-${line.message}`}
                        size="xs"
                    >
                        {formatLogTime(line.time)} {line.message}
                    </Text>
                ))}
            </Code>
        </ScrollArea.Autosize>
    )
}

function PanelUpdateProgress({
    request,
    onClose
}: {
    onClose: () => void
    request: PanelUpdateRequest
}) {
    const [startedLocally] = useState(() => Date.now())
    const [poll, setPoll] = useState<PollState>(() => ({
        lastError: null,
        now: Date.now(),
        response: null,
        unreachableSince: null
    }))
    const view = deriveUpdateView(request, poll, startedLocally)
    const finished = isFinished(view)

    useEffect(() => {
        if (finished) return undefined
        let cancelled = false
        let timer: ReturnType<typeof setTimeout> | undefined
        const tick = async () => {
            try {
                const response = await fetchPanelUpdateStatus()
                if (cancelled) return
                setPoll({ lastError: null, now: Date.now(), response, unreachableSince: null })
            } catch (error) {
                if (cancelled) return
                const lastError = getUpdateErrorMessage(error, 'Status request failed')
                setPoll((prev) => ({
                    ...prev,
                    lastError,
                    now: Date.now(),
                    unreachableSince: prev.unreachableSince ?? Date.now()
                }))
            }
            if (!cancelled) timer = setTimeout(() => void tick(), POLL_INTERVAL)
        }
        void tick()
        return () => {
            cancelled = true
            clearTimeout(timer)
        }
    }, [finished])

    useEffect(() => {
        if (view.kind !== 'done') return undefined
        const timer = setTimeout(() => window.location.reload(), RELOAD_DELAY)
        return () => clearTimeout(timer)
    }, [view.kind])

    const own = belongsToRequest(poll.response?.status, request.requestedAt)
        ? poll.response?.status
        : undefined
    const progress = view.kind === 'done' ? 100 : (own?.progress ?? 0)
    const log = own?.log ?? []

    let title = 'Updating the panel'
    let description = 'Waiting for the update helper to start…'
    if (view.kind === 'running') {
        title = STEP_LABELS[view.status.step] ?? view.status.step
        description = view.status.message
    } else if (view.kind === 'restarting') {
        title = 'Restarting panel'
        description = 'The panel is restarting with the new build. This page reconnects by itself.'
    } else if (view.kind === 'legacy') {
        title = 'Update in progress'
        description =
            'The installed update helper is older and does not report progress. Waiting for the panel to restart with the new build…'
    } else if (view.kind === 'helperIdle') {
        title = 'Update helper has not started'
        description = `The request is still waiting after ${Math.round(HELPER_START_TIMEOUT / 1000)} seconds. Check on the server that gowave-update.path is active:`
    }

    if (view.kind === 'done') {
        return (
            <Stack gap="sm">
                <Alert color="teal" icon={<TbCheck size={16} />} radius="md" variant="light">
                    <Text size="sm">
                        {view.version ? `Updated to ${view.version}.` : 'Update finished.'}{' '}
                        Reloading the page…
                    </Text>
                </Alert>
                <UpdateLog log={log} />
                <Button
                    leftSection={<TbRefresh size={14} />}
                    onClick={() => window.location.reload()}
                    radius="md"
                    size="xs"
                    variant="light"
                >
                    Reload now
                </Button>
            </Stack>
        )
    }

    if (view.kind === 'failed') {
        return (
            <Stack gap="sm">
                <Alert
                    color="red"
                    icon={<TbAlertTriangle size={16} />}
                    radius="md"
                    title="Update failed"
                    variant="light"
                >
                    <Text size="xs" style={{ wordBreak: 'break-word' }}>
                        {view.error}
                    </Text>
                </Alert>
                <UpdateLog log={log} />
                {view.command && (
                    <Stack gap={4}>
                        <Text c="dimmed" size="xs">
                            Full log on the server:
                        </Text>
                        <CommandLine command={view.command} />
                    </Stack>
                )}
                <Button onClick={onClose} radius="md" size="xs" variant="default">
                    Close
                </Button>
            </Stack>
        )
    }

    return (
        <Stack gap="sm">
            <Group gap="sm" wrap="nowrap">
                <Loader size="xs" />
                <Text fw={600} size="sm">
                    {title}
                </Text>
                <Text c="dimmed" ml="auto" size="xs">
                    {progress}%
                </Text>
            </Group>
            <Progress animated radius="xl" size="sm" striped value={progress} />
            <Text c="dimmed" size="xs" style={{ wordBreak: 'break-word' }}>
                {description}
            </Text>
            {own?.file && own.total ? (
                <Text c="dimmed" ff="monospace" size="xs">
                    {own.file}: {prettifyBytesUtil(own.downloaded ?? 0, true)} /{' '}
                    {prettifyBytesUtil(own.total, true)}
                </Text>
            ) : null}
            {view.kind === 'helperIdle' && <CommandLine command={HELPER_COMMAND} />}
            {view.kind === 'legacy' && <CommandLine command={JOURNAL_COMMAND} />}
            <UpdateLog log={log} />
        </Stack>
    )
}

function formatCheckedAt(checkedAt?: string) {
    if (!checkedAt) return 'just now'
    const date = new Date(checkedAt)
    return Number.isNaN(date.getTime()) ? 'just now' : `at ${date.toLocaleTimeString()}`
}

interface BuildInfoModalProps {
    gowaveMetadata: GetMetadataCommand.Response['response']
}

export function BuildInfoModal({ gowaveMetadata }: BuildInfoModalProps) {
    const currentChannel: UpdateChannel =
        gowaveMetadata.git?.backend?.branch === 'dev' ? 'dev' : 'stable'
    const [targetChannel, setTargetChannel] = useState<UpdateChannel>(currentChannel)
    const [updateRequest, setUpdateRequest] = useState<null | PanelUpdateRequest>(null)
    const [isRequesting, setIsRequesting] = useState(false)
    const [requestError, setRequestError] = useState<null | string>(null)
    const [dismissedStatus, setDismissedStatus] = useState<null | string>(null)
    const isSwitching = targetChannel !== currentChannel

    // Docker installs and native installs without the root update helper are
    // updated on the server; the panel only shows the command.
    const checkChannel = isSwitching ? targetChannel : undefined
    const updateCheck = usePanelUpdateCheck(checkChannel)
    const forceCheck = useForceUpdateCheck(checkChannel)
    const hasUpdate = Boolean(updateCheck.data?.hasUpdate)
    const serverUpdateCommand =
        updateCheck.data && !updateCheck.data.canAutoUpdate
            ? updateCheck.data.updateCommand
            : undefined
    const commitSha = gowaveMetadata.git?.backend?.commitSha
    const checkError =
        forceCheck.error ??
        (updateCheck.isError
            ? getUpdateErrorMessage(updateCheck.error, 'Could not check for updates')
            : null)

    // The last update the root helper reported: one still running (for example
    // after a page reload) is followed again, a recent failure is shown.
    const lastUpdate = useQuery({
        queryKey: ['panel-update-status'],
        queryFn: fetchPanelUpdateStatus,
        enabled: updateRequest === null,
        staleTime: 0,
        gcTime: 0,
        retry: false
    })
    const lastStatus = lastUpdate.data?.status
    // Ages are measured against the server clock that wrote the status.
    const serverNow = lastUpdate.data ? Date.parse(lastUpdate.data.serverTime) : 0
    const lastStatusKey = lastStatus ? `${lastStatus.startedAt}-${lastStatus.state}` : null
    const showLastStatus = lastStatus && lastStatusKey !== dismissedStatus
    const resumed: null | PanelUpdateRequest =
        showLastStatus &&
        lastStatus.state === 'running' &&
        serverNow - Date.parse(lastStatus.updatedAt) < UPDATE_TIMEOUT
            ? {
                  channel: lastStatus.channel,
                  fromVersion: lastStatus.fromVersion,
                  requestedAt: lastStatus.startedAt
              }
            : null
    const lastFailure =
        showLastStatus &&
        lastStatus.state === 'failed' &&
        lastStatus.fromVersion === gowaveMetadata.version &&
        serverNow - Date.parse(lastStatus.finishedAt ?? lastStatus.updatedAt) < 24 * 60 * 60 * 1000
            ? lastStatus
            : null
    const activeRequest = updateRequest ?? resumed
    const isUpdating = isRequesting || activeRequest !== null

    const handleUpdate = async () => {
        setIsRequesting(true)
        setRequestError(null)
        try {
            // A root helper downloads and verifies the channel's newest build,
            // then restarts the panel; its progress is polled below.
            const request = await requestPanelUpdate(targetChannel)
            setUpdateRequest({
                channel: request.channel,
                fromVersion: gowaveMetadata.version,
                requestedAt: request.requestedAt
            })
        } catch (error) {
            setRequestError(getUpdateErrorMessage(error, 'Could not start the update'))
        } finally {
            setIsRequesting(false)
        }
    }

    const closeProgress = () => {
        setUpdateRequest(null)
        setDismissedStatus(lastStatusKey)
        void lastUpdate.refetch()
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

            {activeRequest && (
                <Paper className={classes.updateCard} p="md" radius="md">
                    <PanelUpdateProgress onClose={closeProgress} request={activeRequest} />
                </Paper>
            )}

            {!activeRequest && lastFailure && (
                <Alert
                    color="red"
                    icon={<TbAlertTriangle size={16} />}
                    onClose={() => setDismissedStatus(lastStatusKey)}
                    radius="md"
                    title="The last panel update failed"
                    variant="light"
                    withCloseButton
                >
                    <Stack gap="xs">
                        <Text size="xs" style={{ wordBreak: 'break-word' }}>
                            {lastFailure.error || lastFailure.message}
                        </Text>
                        <UpdateLog log={lastFailure.log} />
                    </Stack>
                </Alert>
            )}

            {requestError && (
                <Alert
                    color="red"
                    icon={<TbAlertTriangle size={16} />}
                    radius="md"
                    title="Could not start the update"
                    variant="light"
                >
                    <Text size="xs" style={{ wordBreak: 'break-word' }}>
                        {requestError}
                    </Text>
                </Alert>
            )}

            {!activeRequest && (
                <Group gap="xs" justify="space-between" wrap="nowrap">
                    <Text
                        c={checkError ? 'red.5' : 'dimmed'}
                        size="xs"
                        style={{ wordBreak: 'break-word' }}
                    >
                        {checkError ??
                            (updateCheck.isLoading
                                ? 'Checking for updates…'
                                : hasUpdate
                                  ? `Checked ${formatCheckedAt(updateCheck.data?.checkedAt)}`
                                  : `You are running the latest ${targetChannel} build. Checked ${formatCheckedAt(updateCheck.data?.checkedAt)}.`)}
                    </Text>
                    <Button
                        leftSection={<TbRefresh size={14} />}
                        loading={forceCheck.isChecking}
                        onClick={() => void forceCheck.check()}
                        radius="md"
                        size="compact-xs"
                        style={{ flexShrink: 0 }}
                        variant="subtle"
                    >
                        Check now
                    </Button>
                </Group>
            )}

            {hasUpdate && !activeRequest && (
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
                                loading={isRequesting}
                                onClick={() => void handleUpdate()}
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
                            <CommandLine command={serverUpdateCommand} />
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
