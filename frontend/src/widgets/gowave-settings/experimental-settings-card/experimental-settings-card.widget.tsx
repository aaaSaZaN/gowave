import {
    type IExperimentalFeatures,
    useExperimentalFeatures,
    useViewPreferencesStoreActions
} from '@entities/dashboard/view-preferences-store'
import { Checkbox, Group, Stack, Text, ThemeIcon, type ThemeIconProps } from '@mantine/core'
import { useIsMobile } from '@shared/hooks'
import { optionalProp } from '@shared/lib/optional-prop'
import { SettingsCardShared } from '@shared/ui/settings-card'
import type { TFunction } from 'i18next'
import type { ComponentType } from 'react'
import { useTranslation } from 'react-i18next'
import { TbFlask, TbLayoutSidebar, TbPlugConnected, TbRocket, TbTerminal2 } from 'react-icons/tb'

import classes from './ExperimentalSettingsCard.module.css'

interface IExperimentalFeatureItem {
    color: ThemeIconProps['color']
    desktopOnly?: boolean
    feature: keyof IExperimentalFeatures
    IconComponent: ComponentType<{ size: number }>
    label: string
}

const getExperimentalFeatures = (t: TFunction): IExperimentalFeatureItem[] => [
    {
        color: 'cyan',
        desktopOnly: true,
        feature: 'legacyLayoutStyle',
        IconComponent: TbLayoutSidebar,
        label: t('experimental-settings-card.widget.legacy-layout-style')
    },
    {
        color: 'pink',
        feature: 'nodeIntegrations',
        IconComponent: TbPlugConnected,
        label: t('node-integrations.modal.title')
    },
    {
        color: 'cyan',
        desktopOnly: true,
        feature: 'sshTerminal',
        IconComponent: TbTerminal2,
        label: t('node-ssh.title')
    },
    {
        color: 'teal',
        desktopOnly: true,
        feature: 'quickLauncher',
        IconComponent: TbRocket,
        label: t('constants.quick-launcher')
    }
]

export const ExperimentalSettingsCardWidget = () => {
    const { t } = useTranslation()

    const isMobile = useIsMobile()

    const experimental = useExperimentalFeatures()
    const { setExperimentalFeature } = useViewPreferencesStoreActions()

    return (
        <SettingsCardShared.Container>
            <SettingsCardShared.Header
                description={t('experimental-settings-card.widget.description')}
                icon={<TbFlask size={24} />}
                iconColor="grape"
                iconVariant="soft"
                title={t('experimental-settings-card.widget.experimental-features')}
            />

            <SettingsCardShared.Content>
                <Stack gap="xs">
                    {getExperimentalFeatures(t)
                        .filter(({ desktopOnly }) => !desktopOnly || !isMobile)
                        .map(({ color, feature, IconComponent, label }) => (
                            <Checkbox.Card
                                checked={experimental[feature]}
                                className={classes.feature}
                                key={feature}
                                onClick={() =>
                                    setExperimentalFeature(feature, !experimental[feature])
                                }
                                radius="md"
                            >
                                <Group align="center" gap="sm" wrap="nowrap">
                                    <ThemeIcon
                                        {...optionalProp('color', color)}
                                        radius="md"
                                        size="lg"
                                        variant="soft"
                                    >
                                        <IconComponent size={20} />
                                    </ThemeIcon>

                                    <Text className={classes.label} flex={1} size="sm" truncate>
                                        {label}
                                    </Text>

                                    <Checkbox.Indicator size="sm" />
                                </Group>
                            </Checkbox.Card>
                        ))}
                </Stack>
            </SettingsCardShared.Content>
        </SettingsCardShared.Container>
    )
}
