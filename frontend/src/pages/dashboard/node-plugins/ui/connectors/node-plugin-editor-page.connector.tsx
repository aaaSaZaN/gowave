import { useGetNodePlugin } from '@shared/api/hooks'
import { ROUTES } from '@shared/constants'
import { runAsync } from '@shared/lib/run-async'
import { LoadingScreen } from '@shared/ui'
import { useNavigate, useParams } from 'react-router'

import { NodePluginEditorPageComponent } from '../components/node-plugin-editor-page.component'

export function NodePluginEditorPageConnector() {
    const { uuid } = useParams()
    const navigate = useNavigate()

    const { data: plugin, isLoading: isPluginLoading } = useGetNodePlugin({
        route: {
            uuid: uuid as string
        },
        rQueryParams: {
            enabled: !!uuid
        }
    })

    if (!uuid) {
        runAsync(navigate(ROUTES.DASHBOARD.HOME, { replace: true }))
        return null
    }

    if (isPluginLoading || !plugin) {
        return <LoadingScreen text="Loading plugin..." />
    }

    return <NodePluginEditorPageComponent plugin={plugin} />
}
