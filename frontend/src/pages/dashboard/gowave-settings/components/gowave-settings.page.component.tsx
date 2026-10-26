import { Container } from '@mantine/core'
import { GetApiTokensCommand, GetRemnawaveSettingsCommand } from '@remnawave/backend-contract'
import { ApiTokensCardWidget } from '@widgets/gowave-settings/api-tokens-card/api-tokens-card.widget'
import { AuthentificationSettingsCardWidget } from '@widgets/gowave-settings/authentification-settings-card/authentification-settings-card.widget'
import { BackendToolsCardWidget } from '@widgets/gowave-settings/backend-tools-card/backend-tools-card.widget'
import { BrandingSettingsCardWidget } from '@widgets/gowave-settings/branding-settings-card/branding-settings-card.widget'
import { ExperimentalSettingsCardWidget } from '@widgets/gowave-settings/experimental-settings-card/experimental-settings-card.widget'
import { useTranslation } from 'react-i18next'

import { LoadingScreen, Logo, Page, PageHeaderShared } from '@shared/ui'
import Masonry from '@shared/ui/masonry'

interface IProps {
    apiTokensData: GetApiTokensCommand.Response['response']
    gowaveSettings: GetRemnawaveSettingsCommand.Response['response']
}

export const GowaveSettingsPageComponent = (props: IProps) => {
    const { gowaveSettings, apiTokensData } = props

    const { t } = useTranslation()

    if (!gowaveSettings || !apiTokensData) {
        return <LoadingScreen />
    }

    if (
        !gowaveSettings.oauth2Settings ||
        !gowaveSettings.passkeySettings ||
        !gowaveSettings.passwordSettings ||
        !gowaveSettings.brandingSettings
    ) {
        return <LoadingScreen />
    }

    return (
        <Page title={t('constants.gowave-settings')}>
            <PageHeaderShared icon={<Logo size={24} />} title={t('constants.gowave-settings')} />
            <Container fluid p={0} size="xl">
                <Masonry columns={{ 300: 1, 1400: 2, 2000: 3, 3000: 4 }} gap={16}>
                    <AuthentificationSettingsCardWidget
                        oauth2Settings={gowaveSettings.oauth2Settings}
                        passkeySettings={gowaveSettings.passkeySettings}
                        passwordSettings={gowaveSettings.passwordSettings}
                    />
                    <ApiTokensCardWidget apiTokensData={apiTokensData} />
                    <BackendToolsCardWidget />
                    <BrandingSettingsCardWidget
                        brandingSettings={gowaveSettings.brandingSettings}
                    />
                    <ExperimentalSettingsCardWidget />
                </Masonry>
            </Container>
        </Page>
    )
}
