import NiceModal, { useModal } from '@ebay/nice-modal-react'
import {
    Alert,
    Button,
    Checkbox,
    CopyButton,
    Group,
    Modal,
    SimpleGrid,
    Stack,
    Text,
    Textarea,
    TextInput
} from '@mantine/core'
import type { CreateHostCommand } from '@remnawave/backend-contract'
import { useNiceMantineModal } from '@shared/_modals/use-nice-modal'
import { instance } from '@shared/api/axios'
import { runAsync } from '@shared/lib/run-async'
import { useMutation } from '@tanstack/react-query'
import { isAxiosError } from 'axios'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { PiEye } from 'react-icons/pi'

type HostPreviewTarget = { hostUuid: string } | { host: CreateHostCommand.RequestBody }

interface HostPreviewResult {
    clientType: string
    headers: Record<string, string>
    previewContent: string
    ruleName: string | null
    status: number | null
}

interface HostPreviewProps {
    target: HostPreviewTarget
    isDisabled?: boolean
}

const HostPreviewModal = NiceModal.create(({ target, isDisabled }: HostPreviewProps) => {
    const { t } = useTranslation()
    const { modalProps } = useNiceMantineModal({ modal: useModal() })
    const [userAgent, setUserAgent] = useState('ClashMeta')
    const [headers, setHeaders] = useState('')
    const [previewAsEnabled, setPreviewAsEnabled] = useState(false)

    const {
        data: result,
        error,
        isPending,
        mutate: generate,
        reset
    } = useMutation({
        mutationFn: async () => {
            const requestHeaders = headers
                .split('\n')
                .map((line) => line.trim())
                .filter(Boolean)
                .map((line) => {
                    const separator = line.indexOf(':')
                    if (separator <= 0) throw new Error(t('host-preview.invalid-header'))
                    return {
                        name: line.slice(0, separator).trim(),
                        value: line.slice(separator + 1).trim()
                    }
                })
            const { data } = await instance.post<{ response: HostPreviewResult }>(
                '/api/hosts/preview',
                { ...target, userAgent, requestHeaders, previewAsEnabled }
            )
            return data.response
        }
    })

    const errorMessage = isAxiosError<{ message?: string }>(error)
        ? (error.response?.data?.message ?? error.message)
        : error?.message

    return (
        <Modal {...modalProps} title={t('host-preview.title')} size="xl">
            <Stack gap="md">
                <Text size="sm" c="dimmed">
                    {t(
                        'hostUuid' in target ? 'host-preview.saved-host' : 'host-preview.draft-host'
                    )}
                </Text>
                <SimpleGrid cols={{ base: 1, sm: 2 }}>
                    <Stack gap="sm">
                        <TextInput
                            label="User-Agent"
                            placeholder="ClashMeta, singbox, v2rayN/7.0.0"
                            value={userAgent}
                            disabled={isPending}
                            onChange={(event) => {
                                setUserAgent(event.currentTarget.value)
                                reset()
                            }}
                        />
                        <Textarea
                            label={t('host-preview.request-headers')}
                            description={t('host-preview.headers-format')}
                            placeholder={'Accept: application/json\nX-Client: desktop'}
                            value={headers}
                            rows={4}
                            disabled={isPending}
                            onChange={(event) => {
                                setHeaders(event.currentTarget.value)
                                reset()
                            }}
                        />
                        {isDisabled && (
                            <Checkbox
                                label={t('host-preview.as-enabled')}
                                checked={previewAsEnabled}
                                disabled={isPending}
                                onChange={(event) => {
                                    setPreviewAsEnabled(event.currentTarget.checked)
                                    reset()
                                }}
                            />
                        )}
                        <Button loading={isPending} onClick={() => generate()}>
                            {t('host-preview.generate')}
                        </Button>
                    </Stack>
                    <Stack gap="sm">
                        <Textarea
                            label={t('host-preview.response-headers')}
                            value={
                                result
                                    ? Object.entries(result.headers)
                                          .map(([name, value]) => `${name}: ${value}`)
                                          .join('\n')
                                    : ''
                            }
                            readOnly
                            rows={7}
                            styles={{ input: { fontFamily: 'monospace' } }}
                        />
                        {result && (
                            <Stack gap={2}>
                                <Text size="sm">
                                    {result.status === null
                                        ? t('host-preview.socket-drop')
                                        : t('host-preview.status', { status: result.status })}
                                    {result.clientType && ` · ${result.clientType}`}
                                </Text>
                                <Text size="sm">
                                    {t('host-preview.matched-rule', {
                                        rule: result.ruleName ?? t('common.message.not-set')
                                    })}
                                </Text>
                            </Stack>
                        )}
                    </Stack>
                </SimpleGrid>
                {errorMessage && <Alert color="red">{errorMessage}</Alert>}
                {result && (
                    <>
                        <Textarea
                            label={t('host-preview.response-body')}
                            value={result.previewContent}
                            readOnly
                            rows={12}
                            styles={{ input: { fontFamily: 'monospace' } }}
                        />
                        <Group justify="flex-end">
                            <CopyButton value={result.previewContent}>
                                {({ copied, copy }) => (
                                    <Button
                                        variant="light"
                                        disabled={!result.previewContent}
                                        onClick={copy}
                                    >
                                        {copied
                                            ? t('common.message.copied')
                                            : t('common.action.copy')}
                                    </Button>
                                )}
                            </CopyButton>
                        </Group>
                    </>
                )}
            </Stack>
        </Modal>
    )
})

interface HostPreviewButtonProps {
    getPreview: () => HostPreviewProps
}

export function HostPreviewButton({ getPreview }: HostPreviewButtonProps) {
    const { t } = useTranslation()

    return (
        <Button
            size="xs"
            variant="light"
            color="cyan"
            leftSection={<PiEye size={16} />}
            onClick={() => runAsync(NiceModal.show(HostPreviewModal, getPreview()))}
        >
            {t('host-preview.open')}
        </Button>
    )
}
