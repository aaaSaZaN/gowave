import {
    SUBSCRIPTION_TEMPLATE_TYPE,
    type TSubscriptionTemplateType
} from '@remnawave/backend-contract'
import { useGetHosts, useGetSubscriptionTemplate } from '@shared/api/hooks'
import { ROUTES } from '@shared/constants'
import { runAsync } from '@shared/lib/run-async'
import { LoadingScreen } from '@shared/ui'
import { useNavigate, useParams } from 'react-router'

import { TemplateEditorPageComponent } from '../components/template-editor-page.component.'

export function TemplateEditorPageConnector() {
    const { type, uuid } = useParams()
    const navigate = useNavigate()

    const { data: template, isLoading: isTemplateLoading } = useGetSubscriptionTemplate({
        route: {
            uuid: uuid as string
        },
        rQueryParams: {
            enabled: !!uuid
        }
    })

    const { data: hosts, isLoading: isHostsLoading } = useGetHosts()

    if (isTemplateLoading || isHostsLoading || !template || !hosts) {
        return <LoadingScreen text="Loading template..." />
    }

    let title: string
    let editorType: 'json' | 'yaml'

    switch (type as TSubscriptionTemplateType) {
        case SUBSCRIPTION_TEMPLATE_TYPE.CLASH:
            title = 'Clash'
            editorType = 'yaml'
            break
        case SUBSCRIPTION_TEMPLATE_TYPE.MIHOMO:
            title = 'Mihomo'
            editorType = 'yaml'
            break
        case SUBSCRIPTION_TEMPLATE_TYPE.SINGBOX:
            title = 'Singbox'
            editorType = 'json'
            break
        case SUBSCRIPTION_TEMPLATE_TYPE.STASH:
            title = 'Stash'
            editorType = 'yaml'
            break
        case SUBSCRIPTION_TEMPLATE_TYPE.XRAY_JSON:
            title = 'Xray JSON'
            editorType = 'json'
            break
        default:
            runAsync(navigate(ROUTES.DASHBOARD.HOME, { replace: true }))
            return null
    }

    if (!type) {
        runAsync(navigate(ROUTES.DASHBOARD.HOME, { replace: true }))
        return null
    }

    return (
        <TemplateEditorPageComponent
            editorType={editorType}
            hosts={hosts}
            template={template}
            title={title}
        />
    )
}
