import { Group, Stack, Text, ThemeIcon } from '@mantine/core'
import { Trans } from 'react-i18next'
import { TbClock, TbRadar } from 'react-icons/tb'

const HIGHLIGHT_SPAN = <Text c="var(--mantine-color-bright)" component="span" fw={600} size="sm" />

export const RequirementsWidget = () => {
    return (
        <Stack gap="md">
            <Group gap="sm" wrap="nowrap">
                <ThemeIcon color="cyan" size="md" variant="soft">
                    <TbClock size={16} />
                </ThemeIcon>
                <Text c="dimmed" size="sm">
                    <Trans
                        components={{
                            highlight: HIGHLIGHT_SPAN
                        }}
                        i18nKey="active-sessions-drawer.widget.warning-activity"
                    />
                </Text>
            </Group>

            <Group gap="sm" wrap="nowrap">
                <ThemeIcon color="yellow" size="md" variant="soft">
                    <TbRadar size={16} />
                </ThemeIcon>
                <Text c="dimmed" size="sm">
                    <Trans
                        components={{
                            highlight: HIGHLIGHT_SPAN
                        }}
                        i18nKey="active-sessions-drawer.widget.warning-n-plus-one"
                    />
                </Text>
            </Group>
        </Stack>
    )
}
