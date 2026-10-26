import NiceModal, { useModal } from '@ebay/nice-modal-react'
import {
    ActionIcon,
    Badge,
    Box,
    Button,
    CopyButton,
    Group,
    Loader,
    Modal,
    ScrollArea,
    SegmentedControl,
    Select,
    Switch,
    Text,
    Tooltip
} from '@mantine/core'
import type { GetNodeCommand } from '@remnawave/backend-contract'
import { useNiceMantineModal } from '@shared/_modals/use-nice-modal'
import { instance } from '@shared/api/axios'
import { BaseOverlayHeader } from '@shared/ui/overlays/base-overlay-header'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { TbCheck, TbCopy, TbListDetails, TbRefresh, TbTerminal2 } from 'react-icons/tb'

interface IProps {
    node: GetNodeCommand.Response['response']
}

export const NodeLogsModal = NiceModal.create((props: IProps) => {
    const { node } = props
    const modal = useModal()
    const { modalProps } = useNiceMantineModal({ modal })

    const [activeTab, setActiveTab] = useState<'xray' | 'node'>('xray')
    const [linesCount, setLinesCount] = useState<string>('500')
    const [autoScroll, setAutoScroll] = useState<boolean>(true)
    const [autoRefresh, setAutoRefresh] = useState<boolean>(true)

    const scrollViewportRef = useRef<HTMLDivElement>(null)

    const {
        data: logs = [],
        isLoading,
        isFetching,
        refetch
    } = useQuery({
        queryKey: ['node-logs', node.uuid, activeTab, linesCount],
        queryFn: async () => {
            const res = await instance.get<{ response: { logs: string[] } }>(
                `/api/nodes/${node.uuid}/logs`,
                {
                    params: {
                        type: activeTab,
                        lines: Number(linesCount) || 500
                    }
                }
            )
            return res.data.response.logs || []
        },
        refetchInterval: autoRefresh ? 2500 : false,
        refetchOnWindowFocus: false
    })

    useEffect(() => {
        if (autoScroll && scrollViewportRef.current) {
            scrollViewportRef.current.scrollTo({
                top: scrollViewportRef.current.scrollHeight,
                behavior: 'smooth'
            })
        }
    }, [autoScroll])

    const getLineColor = (line: string): string => {
        if (line.includes('[Warning]') || line.includes('WARN')) {
            return 'var(--mantine-color-yellow-4)'
        }
        if (line.includes('[Error]') || line.includes('ERROR') || line.includes('FATAL')) {
            return 'var(--mantine-color-red-4)'
        }
        if (line.includes('[Info]') || line.includes('INFO')) {
            return 'var(--mantine-color-cyan-4)'
        }
        if (
            line.includes('started') ||
            line.includes('listening') ||
            line.includes('Listening') ||
            line.includes('OK')
        ) {
            return 'var(--mantine-color-teal-4)'
        }
        return 'var(--mantine-color-gray-4)'
    }

    const fullLogsText = logs.join('\n')

    return (
        <Modal
            {...modalProps}
            centered
            removeScrollProps={{ allowPinchZoom: true }}
            size="min(1150px, 96vw)"
            title={
                <BaseOverlayHeader
                    iconColor="cyan"
                    IconComponent={TbTerminal2}
                    iconVariant="soft"
                    subtitle={node.name}
                    title="Логи ноды и Xray"
                />
            }
            transitionProps={{ transition: 'fade', duration: 200 }}
        >
            <Box>
                <Group justify="space-between" mb="sm" wrap="wrap">
                    <Group gap="sm">
                        <SegmentedControl
                            data={[
                                { label: 'Xray Core', value: 'xray' },
                                { label: 'GoWave Node (Go)', value: 'node' }
                            ]}
                            onChange={(val) => setActiveTab(val as 'xray' | 'node')}
                            size="xs"
                            value={activeTab}
                        />

                        <Select
                            allowDeselect={false}
                            checkIconPosition="right"
                            data={[
                                { value: '100', label: '100 строк' },
                                { value: '500', label: '500 строк' },
                                { value: '1000', label: '1000 строк' },
                                { value: '2000', label: '2000 строк' }
                            ]}
                            onChange={(val) => setLinesCount(val || '500')}
                            size="xs"
                            style={{ width: 130 }}
                            value={linesCount}
                        />

                        <Badge color="gray" size="sm" variant="light">
                            {logs.length} строк
                        </Badge>
                    </Group>

                    <Group gap="sm">
                        <Switch
                            checked={autoRefresh}
                            label="Автообновление (2.5с)"
                            onChange={(e) => setAutoRefresh(e.currentTarget.checked)}
                            size="xs"
                        />

                        <Switch
                            checked={autoScroll}
                            label="Автоскролл"
                            onChange={(e) => setAutoScroll(e.currentTarget.checked)}
                            size="xs"
                        />

                        <Tooltip label="Обновить сейчас" withArrow>
                            <ActionIcon
                                color="gray"
                                loading={isFetching}
                                onClick={() => void refetch()}
                                size="sm"
                                variant="light"
                            >
                                <TbRefresh size={16} />
                            </ActionIcon>
                        </Tooltip>

                        <CopyButton timeout={2000} value={fullLogsText}>
                            {({ copied, copy }) => (
                                <Button
                                    color={copied ? 'teal' : 'gray'}
                                    leftSection={
                                        copied ? <TbCheck size={14} /> : <TbCopy size={14} />
                                    }
                                    onClick={copy}
                                    size="xs"
                                    variant="light"
                                >
                                    {copied ? 'Скопировано' : 'Скопировать всё'}
                                </Button>
                            )}
                        </CopyButton>
                    </Group>
                </Group>

                <Box
                    p="sm"
                    style={{
                        backgroundColor: 'var(--mantine-color-dark-8, #141517)',
                        borderRadius: 'var(--mantine-radius-md)',
                        border: '1px solid var(--mantine-color-dark-4, #2c2e33)',
                        fontFamily:
                            'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
                        fontSize: '12px',
                        lineHeight: '1.45',
                        position: 'relative',
                        minHeight: '360px'
                    }}
                >
                    {isLoading ? (
                        <Group justify="center" p="xl" style={{ minHeight: '360px' }}>
                            <Loader color="cyan" size="md" />
                            <Text c="dimmed" size="xs">
                                Загрузка логов...
                            </Text>
                        </Group>
                    ) : logs.length === 0 ? (
                        <Group justify="center" p="xl" style={{ minHeight: '360px' }}>
                            <TbListDetails color="gray" size={32} />
                            <Text c="dimmed" size="sm">
                                Нет доступных записей логов
                            </Text>
                        </Group>
                    ) : (
                        <ScrollArea.Autosize
                            mah="55vh"
                            offsetScrollbars
                            type="always"
                            viewportRef={scrollViewportRef}
                        >
                            <Box style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                                {logs.map((line, idx) => (
                                    <Text
                                        component="div"
                                        // biome-ignore lint/suspicious/noArrayIndexKey: Log rows are read-only line positions; identical log text can occur repeatedly.
                                        key={idx}
                                        size="xs"
                                        style={{
                                            color: getLineColor(line),
                                            padding: '1px 0',
                                            fontFamily: 'inherit'
                                        }}
                                    >
                                        {line}
                                    </Text>
                                ))}
                            </Box>
                        </ScrollArea.Autosize>
                    )}
                </Box>
            </Box>
        </Modal>
    )
})
