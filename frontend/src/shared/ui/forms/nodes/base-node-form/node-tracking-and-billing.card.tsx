import {
    Box,
    Button,
    Collapse,
    Group,
    NumberInput,
    SegmentedControl,
    Stack,
    Switch,
    TagsInput,
    Text,
    Textarea
} from '@mantine/core'
import { UseFormReturnType } from '@mantine/form'
import { CreateNodeCommand, UpdateNodeCommand } from '@remnawave/backend-contract'
import { ForwardRefComponent, HTMLMotionProps, Variants } from 'motion/react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { PiTagDuotone } from 'react-icons/pi'
import { TbBell, TbChartBar, TbChartLine, TbClock, TbExternalLink } from 'react-icons/tb'

import { TTrafficTrackingStart, useGetNodesTags } from '@shared/api/hooks'
import { TrafficLimitInput } from '@shared/ui/forms/traffic-limit-input'
import { SelectInfraProviderShared } from '@shared/ui/infra-billing/select-infra-provider/select-infra-provider.shared'
import { BaseOverlayHeader } from '@shared/ui/overlays/base-overlay-header'
import { SectionCard } from '@shared/ui/section-card'
import { TagInputPill } from '@shared/ui/tag-input-pill'
import { prettifyBytesUtil } from '@shared/utils/bytes'
import { getNodeTrafficPeriodStartUtil } from '@shared/utils/time-utils'

import classes from './node-tracking-and-billing.card.module.css'

const URL_REGEX = /https?:\/\/[^\s]+/i

function extractFirstUrl(text: string): null | string {
    const match = text.match(URL_REGEX)
    return match ? match[0] : null
}

// How the counter starts when an edit enables tracking on a node that had it off.
export interface ITrafficTrackingStartControl {
    accumulatedBytes: number
    onChange: (value: TTrafficTrackingStart) => void
    value: TTrafficTrackingStart
}

interface IProps<T extends CreateNodeCommand.RequestBody | UpdateNodeCommand.RequestBody> {
    cardVariants: Variants
    form: UseFormReturnType<T>
    motionWrapper: ForwardRefComponent<HTMLDivElement, HTMLMotionProps<'div'>>
    trafficTrackingStart?: ITrafficTrackingStartControl
}

export const NodeTrackingAndBillingCard = <
    T extends CreateNodeCommand.RequestBody | UpdateNodeCommand.RequestBody
>(
    props: IProps<T>
) => {
    const { t, i18n } = useTranslation()
    const { data: nodesTags } = useGetNodesTags()
    const { cardVariants, form, motionWrapper, trafficTrackingStart } = props

    const [advancedOpened, setAdvancedOpened] = useState<boolean>(false)
    const [firstNoteUrl, setFirstNoteUrl] = useState<null | string>(null)
    const [resetDay, setResetDay] = useState<number>(form.getValues().trafficResetDay ?? 1)

    form.watch('isTrafficTrackingActive', ({ value }) => {
        setAdvancedOpened(value ?? false)
    })

    form.watch('trafficResetDay', ({ value }) => {
        setResetDay(typeof value === 'number' ? value : 1)
    })

    const trackingStartDescription = () => {
        switch (trafficTrackingStart?.value) {
            case 'period':
                return t('base-node-form.tracking-start.period-description', {
                    date: getNodeTrafficPeriodStartUtil(resetDay).toLocaleDateString(
                        i18n.language,
                        { day: 'numeric', month: 'long' }
                    )
                })
            case 'zero':
                return t('base-node-form.tracking-start.zero-description')
            default:
                return t('base-node-form.tracking-start.keep-description', {
                    value: prettifyBytesUtil(trafficTrackingStart?.accumulatedBytes ?? 0) || '0 B'
                })
        }
    }

    form.watch('note', ({ value }) => {
        setFirstNoteUrl(extractFirstUrl(typeof value === 'string' ? value : ''))
    })

    const MotionWrapper = motionWrapper

    return (
        <MotionWrapper variants={cardVariants}>
            <SectionCard.Root>
                <SectionCard.Section>
                    <BaseOverlayHeader
                        iconColor="yellow"
                        IconComponent={TbChartBar}
                        iconVariant="soft"
                        title={t('base-node-form.tracking-and-billing')}
                        titleOrder={5}
                    />
                </SectionCard.Section>
                <SectionCard.Section>
                    <Stack gap="md">
                        <SelectInfraProviderShared
                            selectedInfraProviderUuid={form.getValues().providerUuid}
                            setSelectedInfraProviderUuid={(providerUuid) => {
                                form.setFieldValue('providerUuid', providerUuid as never)
                            }}
                        />

                        <Box className={classes.trackingCard}>
                            <Group
                                className={classes.trackingHeader}
                                gap="xs"
                                justify="space-between"
                            >
                                <Group gap="xs">
                                    <TbChartLine
                                        size={18}
                                        style={{ color: 'var(--mantine-color-indigo-6)' }}
                                    />
                                    <Text fw={500} size="sm">
                                        {t('base-node-form.traffic-tracking')}
                                    </Text>
                                </Group>
                                <Switch
                                    key={form.key('isTrafficTrackingActive')}
                                    {...form.getInputProps('isTrafficTrackingActive', {
                                        type: 'checkbox'
                                    })}
                                    onChange={(event) => {
                                        form.getInputProps('isTrafficTrackingActive').onChange(
                                            event
                                        )
                                        setAdvancedOpened(event.currentTarget.checked)
                                    }}
                                    size="md"
                                />
                            </Group>

                            <Collapse expanded={advancedOpened}>
                                <Stack className={classes.trackingBody} gap="sm">
                                    <Group gap="md" grow justify="space-between" w="100%">
                                        <TrafficLimitInput
                                            hideControls
                                            key={form.key('trafficLimitBytes')}
                                            label={t('base-node-form.limit')}
                                            leftSection={<TbChartLine size={16} />}
                                            {...form.getInputProps('trafficLimitBytes')}
                                            styles={{
                                                label: { fontWeight: 500 }
                                            }}
                                        />
                                    </Group>

                                    <Group gap="md" grow justify="space-between" w="100%">
                                        <NumberInput
                                            key={form.key('trafficResetDay')}
                                            label={t('base-node-form.reset-day')}
                                            {...form.getInputProps('trafficResetDay')}
                                            allowDecimal={false}
                                            allowNegative={false}
                                            clampBehavior="strict"
                                            decimalScale={0}
                                            hideControls
                                            leftSection={<TbClock size={16} />}
                                            max={31}
                                            min={1}
                                            placeholder={t('base-node-form.e-g-1-31')}
                                            styles={{
                                                label: { fontWeight: 500 }
                                            }}
                                        />

                                        <NumberInput
                                            key={form.key('notifyPercent')}
                                            label={t('base-node-form.notify-percent')}
                                            {...form.getInputProps('notifyPercent')}
                                            allowDecimal={false}
                                            allowNegative={false}
                                            clampBehavior="strict"
                                            decimalScale={0}
                                            hideControls
                                            leftSection={<TbBell size={16} />}
                                            max={100}
                                            placeholder={t('base-node-form.e-g-50')}
                                            styles={{
                                                label: {
                                                    fontWeight: 500,
                                                    whiteSpace: 'nowrap',
                                                    textOverflow: 'ellipsis'
                                                }
                                            }}
                                        />
                                    </Group>

                                    {trafficTrackingStart && (
                                        <Stack gap={6}>
                                            <Text fw={500} size="sm">
                                                {t('base-node-form.tracking-start.label')}
                                            </Text>
                                            <SegmentedControl
                                                data={[
                                                    {
                                                        value: 'period',
                                                        label: t(
                                                            'base-node-form.tracking-start.period'
                                                        )
                                                    },
                                                    {
                                                        value: 'zero',
                                                        label: t(
                                                            'base-node-form.tracking-start.zero'
                                                        )
                                                    },
                                                    {
                                                        value: 'keep',
                                                        label: t(
                                                            'base-node-form.tracking-start.keep'
                                                        )
                                                    }
                                                ]}
                                                fullWidth
                                                onChange={(value) =>
                                                    trafficTrackingStart.onChange(
                                                        value as TTrafficTrackingStart
                                                    )
                                                }
                                                size="xs"
                                                value={trafficTrackingStart.value}
                                            />
                                            <Text c="dimmed" size="xs">
                                                {trackingStartDescription()}
                                            </Text>
                                        </Stack>
                                    )}
                                </Stack>
                            </Collapse>
                        </Box>

                        <TagsInput
                            clearable
                            data={nodesTags?.tags || []}
                            key={form.key('tags')}
                            label={t('common.field.tags')}
                            leftSection={<PiTagDuotone size="16px" />}
                            maxTags={10}
                            placeholder="Enter tags (comma, space, semicolon)"
                            splitChars={[',', ' ', ';']}
                            {...form.getInputProps('tags')}
                            error={
                                Object.keys(form.errors)
                                    .filter((key) => key.startsWith('tags.'))
                                    .map((key) => form.errors[key])
                                    .join(', ') || form.getInputProps('tags').error
                            }
                            renderPill={({ value, onRemove }) => (
                                <TagInputPill onRemove={onRemove} value={value} />
                            )}
                        />

                        <Stack gap={6}>
                            <Textarea
                                key={form.key('note')}
                                label={t('node-tracking-and-billing.card.note')}
                                resize="vertical"
                                {...form.getInputProps('note')}
                                styles={{
                                    label: { fontWeight: 500 }
                                }}
                            />
                            {firstNoteUrl && (
                                <Button
                                    component="a"
                                    href={firstNoteUrl}
                                    leftSection={<TbExternalLink size={14} />}
                                    maw="100%"
                                    rel="noopener noreferrer"
                                    size="xs"
                                    target="_blank"
                                    variant="soft"
                                    w="fit-content"
                                >
                                    {t('common.action.open')}
                                </Button>
                            )}
                        </Stack>
                    </Stack>
                </SectionCard.Section>
            </SectionCard.Root>
        </MotionWrapper>
    )
}
