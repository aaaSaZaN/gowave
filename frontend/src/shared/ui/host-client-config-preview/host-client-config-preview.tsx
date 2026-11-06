import { Button, CopyButton, Group, Loader, SegmentedControl, Stack, Textarea } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { instance } from '@shared/api/axios'
import { runAsync } from '@shared/lib/run-async'
import { CompoundModalShared } from '@shared/ui/compound-modal/compound-modal.shared'
import { useEffect, useState } from 'react'

export interface HostPreviewHostInput {
    address?: string
    configProfileInboundUuid?: string | null
    configProfileUuid?: string | null
    fingerprint?: string
    path?: string
    sni?: string
    remark?: string
    securityLayer?: string
    port?: number
}

export type HostPreviewPayload = { hostUuid: string } | { host: HostPreviewHostInput }

type PreviewFormat = 'key' | 'clash' | 'sing-box' | 'xray'

interface IProps {
    inboundType?: string | null
    opened: boolean
    onClose: () => void
    payload: HostPreviewPayload
}

// The share link tab follows the inbound protocol; everything else serves the
// dedicated client formats.
const shareLinkLabels: Record<string, string> = {
    shadowsocks: 'SS key',
    trojan: 'Trojan key',
    vless: 'VLESS key'
}

const userAgentByFormat: Record<Exclude<PreviewFormat, 'key'>, string> = {
    'sing-box': 'Sing-Box',
    clash: 'ClashMeta',
    xray: 'xray'
}

export const HostClientConfigPreviewModal = (props: IProps) => {
    const { payload, inboundType, opened, onClose } = props

    const [format, setFormat] = useState<PreviewFormat>('key')
    const [content, setContent] = useState('')
    const [loading, setLoading] = useState(false)

    const shareLabel = shareLinkLabels[(inboundType ?? '').toLowerCase()] ?? 'VLESS key'

    useEffect(() => {
        if (!opened) {
            return
        }
        let cancelled = false
        const load = async () => {
            setLoading(true)
            const userAgent =
                format === 'key' ? (inboundType ?? 'vless') : userAgentByFormat[format]
            const body =
                'hostUuid' in payload
                    ? { hostUuid: payload.hostUuid, userAgent }
                    : { host: payload.host, userAgent }
            try {
                const res = await instance.post('/api/hosts/preview', body)
                if (!cancelled) {
                    setContent(res.data.response?.previewContent || '')
                }
            } catch (error: unknown) {
                const responseMessage = (error as { response?: { data?: { message?: string } } })
                    ?.response?.data?.message
                notifications.show({
                    message: responseMessage || (error as Error).message || 'Failed to preview',
                    title: 'Preview Error',
                    color: 'red'
                })
            } finally {
                if (!cancelled) {
                    setLoading(false)
                }
            }
        }

        runAsync(load())
        return () => {
            cancelled = true
        }
    }, [opened, format, payload, inboundType])

    return (
        <CompoundModalShared
            modalProps={{ onClose, opened, size: 'lg' }}
            title="Host Client Config Preview"
        >
            <Stack gap="md">
                <Group align="center" justify="space-between">
                    <SegmentedControl
                        data={[
                            { label: shareLabel, value: 'key' },
                            { label: 'Clash / Mihomo', value: 'clash' },
                            { label: 'Sing-Box', value: 'sing-box' },
                            { label: 'Xray-Json', value: 'xray' }
                        ]}
                        onChange={(value) => {
                            setFormat(value as PreviewFormat)
                        }}
                        value={format}
                    />
                    {loading && <Loader color="teal" size="xs" />}
                </Group>

                <Textarea
                    label="Client Output"
                    readOnly
                    rows={12}
                    styles={{ input: { fontFamily: 'monospace', fontSize: '12px' } }}
                    value={content}
                />
                <Group justify="flex-end">
                    <CopyButton value={content}>
                        {({ copied, copy }) => (
                            <Button color={copied ? 'teal' : 'blue'} onClick={copy}>
                                {copied ? 'Copied' : 'Copy Config'}
                            </Button>
                        )}
                    </CopyButton>
                </Group>
            </Stack>
        </CompoundModalShared>
    )
}
