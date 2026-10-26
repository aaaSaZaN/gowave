import { useGetSubscriptionPageConfig } from '@shared/api/hooks'
import { ROUTES } from '@shared/constants'
import { runAsync } from '@shared/lib/run-async'
import { LoadingScreen } from '@shared/ui'
import { useNavigate, useParams } from 'react-router'

import { SubpageConfigEditorPageComponent } from '../components/subpage-config-editor-page.component'

export function SubpageConfigEditorPageConnector() {
    const { uuid } = useParams()
    const navigate = useNavigate()

    const { data: config, isLoading: isConfigLoading } = useGetSubscriptionPageConfig({
        route: {
            uuid: uuid as string
        },
        rQueryParams: {
            enabled: !!uuid
        }
    })

    if (isConfigLoading || !config) {
        return <LoadingScreen text="Loading config..." />
    }

    if (!uuid) {
        runAsync(navigate(ROUTES.DASHBOARD.HOME, { replace: true }))
        return null
    }

    return <SubpageConfigEditorPageComponent config={config} />
}
