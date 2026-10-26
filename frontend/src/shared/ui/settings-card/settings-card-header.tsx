import {
    Box,
    Divider,
    Group,
    Stack,
    Text,
    ThemeIcon,
    type ThemeIconProps,
    Title
} from '@mantine/core'
import { optionalProp } from '@shared/lib/optional-prop'
import type { ReactNode } from 'react'

interface SettingsCardHeaderProps {
    description: ReactNode | string
    icon: ReactNode
    iconColor: ThemeIconProps['color']
    iconVariant: ThemeIconProps['variant']
    title: string
}

export function SettingsCardHeader({
    description,
    icon,
    iconColor,
    iconVariant,
    title
}: SettingsCardHeaderProps) {
    return (
        <Box>
            <Group align="flex-start" justify="space-between">
                <Group align="flex-start" gap="md" wrap="nowrap">
                    <ThemeIcon
                        {...optionalProp('color', iconColor)}
                        size="xl"
                        {...optionalProp('variant', iconVariant)}
                    >
                        {icon}
                    </ThemeIcon>

                    <Stack gap={0}>
                        <Title order={4}>{title}</Title>
                        <Text c="dimmed" size="sm">
                            {description}
                        </Text>
                    </Stack>
                </Group>
            </Group>

            <Divider
                my="md"
                style={{
                    opacity: 0.3
                }}
            />
        </Box>
    )
}
