import {
    useExperimentalFeature,
    useViewPreferencesStoreActions
} from '@entities/dashboard/view-preferences-store'
import { Checkbox, Group, Text, ThemeIcon } from '@mantine/core'
import { SettingsCardShared } from '@shared/ui/settings-card'
import { useTranslation } from 'react-i18next'
import { TbFlask, TbLayoutSidebar } from 'react-icons/tb'

import classes from './ExperimentalSettingsCard.module.css'

export const ExperimentalSettingsCardWidget = () => {
    const { t } = useTranslation()
    const isLegacyLayoutStyle = useExperimentalFeature('legacyLayoutStyle')
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
                <Checkbox.Card
                    checked={isLegacyLayoutStyle}
                    className={classes.feature}
                    onClick={() =>
                        setExperimentalFeature('legacyLayoutStyle', !isLegacyLayoutStyle)
                    }
                    radius="md"
                >
                    <Group align="center" gap="sm" wrap="nowrap">
                        <ThemeIcon color="cyan" radius="md" size="lg" variant="soft">
                            <TbLayoutSidebar size={20} />
                        </ThemeIcon>

                        <Text className={classes.label} flex={1} size="sm" truncate>
                            {t('experimental-settings-card.widget.legacy-layout-style')}
                        </Text>

                        <Checkbox.Indicator size="sm" />
                    </Group>
                </Checkbox.Card>
            </SettingsCardShared.Content>
        </SettingsCardShared.Container>
    )
}
