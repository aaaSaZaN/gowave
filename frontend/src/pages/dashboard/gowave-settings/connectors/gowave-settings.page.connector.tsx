import { useGetApiTokens, useGetGowaveSettings, useGetScopes } from '@shared/api/hooks'
import { LoadingScreen } from '@shared/ui/loading-screen'

import { GowaveSettingsPageComponent } from '../components'

export const GowaveSettingsConnector = () => {
    const { data: gowaveSettings, isLoading: isGowaveSettingsLoading } = useGetGowaveSettings()
    const { data: apiTokensData, isLoading: isApiTokensLoading } = useGetApiTokens()
    const { data: scopes, isLoading: isScopesLoading } = useGetScopes()

    if (
        isGowaveSettingsLoading ||
        isApiTokensLoading ||
        isScopesLoading ||
        !gowaveSettings ||
        !apiTokensData ||
        !scopes
    ) {
        return <LoadingScreen />
    }

    return (
        <GowaveSettingsPageComponent
            apiTokensData={apiTokensData}
            gowaveSettings={gowaveSettings}
        />
    )
}
