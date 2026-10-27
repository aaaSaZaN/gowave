import {
    ActionIcon,
    Button,
    Card,
    type DefaultMantineColor,
    Divider,
    Group,
    Stack,
    Text,
    TextInput,
    ThemeIcon
} from '@mantine/core'
import { useAutoAnimate } from '@shared/hooks/use-auto-animate'
import { TemplateInfoPopoverShared } from '@shared/ui/popovers/template-info-popover/template-info-popover.shared'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { PiPlus, PiTrash } from 'react-icons/pi'

const EMPTY_REMARKS = ['']

interface RemarkRow {
    id: string
    value: string
}

const remarkRows = (values: string[]): RemarkRow[] =>
    values.map((value, index) => ({ id: `initial:${index}`, value }))

const hasSameValues = (rows: RemarkRow[], values: string[]) =>
    rows.length === values.length && rows.every((row, index) => row.value === values[index])

export const RemarksManager = ({
    initialRemarks = EMPTY_REMARKS,
    title,
    onChange,
    icon,
    iconColor
}: {
    icon: React.ReactNode
    iconColor: DefaultMantineColor
    initialRemarks?: string[]
    onChange: (remarks: string[]) => void
    title: string
}) => {
    const [localRemarks, setLocalRemarks] = useState(() => remarkRows(initialRemarks))
    const { t } = useTranslation()

    const [parent] = useAutoAnimate((el, action) => {
        let keyframes: Keyframe[] | undefined
        if (action === 'add') {
            keyframes = [
                { transform: 'scale(.98)', opacity: 0 },
                { transform: 'scale(1)', opacity: 1 }
            ]
        }
        if (action === 'remove') {
            keyframes = [
                { transform: 'scale(1)', opacity: 1 },
                { transform: 'scale(.98)', opacity: 0 }
            ]
        }
        if (action === 'remain') {
            keyframes = [{ opacity: 0.98 }, { opacity: 1 }]
        }

        return new KeyframeEffect(el, keyframes as Keyframe[], {
            duration: 250,
            easing: 'ease-in-out'
        })
    })

    // Parents store what onChange reports and pass it back as a new array. Only a different
    // value (initial load, save, another squad) replaces the rows; the echo of our own edit
    // must not, or the rows reset and report again in a render loop.
    const [previousRemarks, setPreviousRemarks] = useState(initialRemarks)
    if (previousRemarks !== initialRemarks) {
        setPreviousRemarks(initialRemarks)
        if (!hasSameValues(localRemarks, initialRemarks)) {
            setLocalRemarks(remarkRows(initialRemarks))
        }
    }

    const commitRemarks = (nextRemarks: RemarkRow[]) => {
        setLocalRemarks(nextRemarks)
        onChange(nextRemarks.map((remark) => remark.value))
    }

    const addLocalRemark = () => {
        commitRemarks([...localRemarks, { id: crypto.randomUUID(), value: '' }])
    }

    const removeLocalRemark = (index: number) => {
        const newRemarks = localRemarks.filter((_, remarkIndex) => remarkIndex !== index)
        if (newRemarks.length === 0) newRemarks.push({ id: crypto.randomUUID(), value: '' })
        commitRemarks(newRemarks)
    }

    const updateLocalRemark = (index: number, value: string) => {
        if (!localRemarks[index]) return
        commitRemarks(
            localRemarks.map((remark, remarkIndex) =>
                remarkIndex === index ? { ...remark, value } : remark
            )
        )
    }

    return (
        <Card>
            <Stack gap="md">
                <Group align="center" gap="xs" justify="space-between" wrap="nowrap">
                    <Group align="center" gap="xs" wrap="nowrap">
                        <ThemeIcon color={iconColor} size="lg" variant="light">
                            {icon}
                        </ThemeIcon>
                        <Text fw={600} size="md">
                            {title}
                        </Text>
                    </Group>

                    <Button
                        leftSection={<PiPlus size="16px" />}
                        onClick={addLocalRemark}
                        size="sm"
                        variant="light"
                    >
                        {t('common.action.add')}
                    </Button>
                </Group>

                <Divider />

                <Stack gap="xs" ref={parent}>
                    {localRemarks.map((remark, index) => (
                        <Group align="flex-start" gap="sm" key={remark.id}>
                            <TextInput
                                leftSection={<TemplateInfoPopoverShared />}
                                onChange={(e) => updateLocalRemark(index, e.target.value)}
                                placeholder={t('remarks-manager.widget.enter-remark')}
                                style={{ flex: 1 }}
                                value={remark.value}
                            />
                            <ActionIcon
                                aria-label={t('common.action.delete')}
                                color="red"
                                disabled={localRemarks.length === 1}
                                onClick={() => removeLocalRemark(index)}
                                size="lg"
                                variant="light"
                            >
                                <PiTrash size="16px" />
                            </ActionIcon>
                        </Group>
                    ))}
                </Stack>
            </Stack>
        </Card>
    )
}
