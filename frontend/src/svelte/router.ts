import { OPEN_ENTITY, ROUTES } from '@shared/constants/routes'
import { panelBasePath, panelUrl } from '@shared/lib/panel-path'
import { isValidUuid } from '@shared/utils/misc/is'
import { consumeReturnTo, saveReturnTo } from '@shared/utils/return-to.util'
import { createRouter, type HooksContext, serializeSearch } from 'sv-router'
import { writable } from 'svelte/store'

import { queryClient } from './query-client'
import RouteLoading from './RouteLoading.svelte'
import { session } from './session.svelte'
import { templateType } from './templates/templates'
import { userSelection } from './users/selection.svelte'

const management = ROUTES.DASHBOARD.MANAGEMENT
const subpages = ROUTES.DASHBOARD.SUBPAGE_CONFIGS
const entityFallback = {
    [OPEN_ENTITY.USER]: management.USERS,
    [OPEN_ENTITY.HOST]: management.HOSTS,
    [OPEN_ENTITY.NODE]: management.NODES,
    [OPEN_ENTITY.CONFIG_PROFILE]: management.CONFIG_PROFILES,
    [OPEN_ENTITY.INTERNAL_SQUAD]: management.INTERNAL_SQUADS,
    [OPEN_ENTITY.EXTERNAL_SQUAD]: management.EXTERNAL_SQUADS,
    [OPEN_ENTITY.NODE_PLUGIN]: management.NODE_PLUGINS.ROOT,
    [OPEN_ENTITY.SUBPAGE_CONFIG]: subpages.ROOT
} as const

const oauthCallback = () => import('./auth/OauthCallback.svelte')
const externalSquads = () => import('./external-squads/ExternalSquads.svelte')
const externalSquadDetailsPath = /^\/dashboard\/open\/external-squad\/[^/]+$/
const navigationTarget = writable('')
export const navigationPending = { subscribe: navigationTarget.subscribe }

export function dashboardViewPath(pathname: string): string {
    const path = pathname.replace(/\/$/, '')
    return externalSquadDetailsPath.test(path) ? management.EXTERNAL_SQUADS : path
}

export const { navigate, p, route } = createRouter(
    {
        hooks: {
            beforeLoad({ pathname }) {
                const changingPage =
                    dashboardViewPath(pathname) !== dashboardViewPath(route.pathname)
                navigationTarget.set(changingPage ? pathname : '')
            },
            afterLoad: () => navigationTarget.set(''),
            onError(_error, { pathname }) {
                navigationTarget.update((target) => (target === pathname ? '' : target))
            }
        },
        '/': {
            hooks: {
                beforeLoad({ pathname }) {
                    if (pathname === '/') redirect(ROUTES.DASHBOARD.HOME)
                }
            },
            '/': RouteLoading
        },
        [ROUTES.AUTH.ROOT]: {
            hooks: {
                beforeLoad({ pathname }) {
                    if (session.authenticated) {
                        returnToDashboard()
                        return false
                    }
                    if (pathname.replace(/\/+$/, '') === ROUTES.AUTH.ROOT)
                        redirect(ROUTES.AUTH.LOGIN)
                    return true
                }
            },
            '/': RouteLoading,
            '/login': () => import('./auth/Login.svelte')
        },
        [ROUTES.OAUTH2.ROOT]: oauthCallback,
        [ROUTES.OAUTH2.AUTH_CALLBACK]: oauthCallback,
        [ROUTES.DASHBOARD.ROOT]: {
            layout: () => import('./Dashboard.svelte'),
            hooks: { beforeLoad: requireSession },
            '/': RouteLoading,
            '/home': () => import('./Home.svelte'),
            '/management': RouteLoading,
            '/crm/infra-billing': () => import('./billing/InfraBilling.svelte'),
            '/subpage': () => import('./subpages/Subpages.svelte'),
            '/subpage/:uuid': () => import('./subpages/SubpageEditor.svelte'),
            '/templates': () => import('./templates/Templates.svelte'),
            '/templates/:type': () => import('./templates/Templates.svelte'),
            '/templates/:type/:uuid': () => import('./templates/TemplateEditor.svelte'),
            '/management/stats/nodes': () => import('./nodes/NodeUsage.svelte'),
            '/management/metrics/nodes': () => import('./nodes/NodeMetrics.svelte'),
            '/management/settings': () => import('./settings/GowaveSettings.svelte'),
            '/management/plugins': () => import('./plugins/NodePlugins.svelte'),
            '/management/plugins/:uuid': () => import('./plugins/PluginEditor.svelte'),
            '/management/nodes': () => import('./nodes/Nodes.svelte'),
            '/open/node/:id': () => import('./nodes/NodeDetails.svelte'),
            '/management/hosts': () => import('./hosts/Hosts.svelte'),
            '/open/host/:id': () => import('./hosts/HostDetails.svelte'),
            '/management/users': () => import('./users/Users.svelte'),
            '/open/user/:id': () => import('./users/UserDetails.svelte'),
            '/open/external-squad/:id': {
                layout: externalSquads,
                '/': () => import('./external-squads/ExternalSquadDetails.svelte')
            },
            '/management/external-squads': externalSquads,
            '/tools/torrent-blocker-reports': () =>
                import('./torrent-reports/TorrentReports.svelte'),
            '/tools/sessions-explorer': () => import('./sessions/SessionsExplorer.svelte'),
            '/tools/http-stats': () => import('./tools/HttpStats.svelte'),
            '/tools/quick-open': () => import('./tools/QuickOpen.svelte'),
            '/tools/hwid-inspector': () => import('./tools/HwidInspector.svelte'),
            '/tools/srh-inspector': () => import('./tools/HistoryInspector.svelte'),
            '/open/internal-squad/:id': () => import('./internal-squads/SquadDetails.svelte'),
            '/open/:entity/:id': RouteLoading,
            '/management/internal-squads': () => import('./internal-squads/InternalSquads.svelte'),
            '/management/subscription-settings': () =>
                import('./subscriptions/SubscriptionSettings.svelte'),
            '/management/response-rules': () => import('./response-rules/ResponseRules.svelte'),
            '/management/config-profiles': () => import('./config-profiles/ConfigProfiles.svelte'),
            '/management/config-profiles/:uuid': () =>
                import('./config-profiles/ConfigProfile.svelte')
        },
        '*': () => import('./NotFound.svelte')
    },
    { base: panelBasePath.replace(/\/$/, '') }
)

function redirect(
    path:
        | typeof ROUTES.AUTH.LOGIN
        | typeof ROUTES.DASHBOARD.HOME
        | (typeof entityFallback)[keyof typeof entityFallback]
): never {
    // sv-router treats the result of a navigation thrown from beforeLoad as a redirect.
    throw navigate(path, { replace: true })
}

function resolveEntity(entity: string, id: string): void {
    if (!Object.hasOwn(entityFallback, entity)) redirect(ROUTES.DASHBOARD.HOME)
    const target = entity as keyof typeof entityFallback
    if (!(target === OPEN_ENTITY.USER ? /^\d+$/.test(id) : isValidUuid(id))) {
        redirect(entityFallback[target])
    }
    switch (target) {
        case OPEN_ENTITY.CONFIG_PROFILE:
            throw navigate(management.CONFIG_PROFILE_BY_UUID, {
                params: { uuid: id },
                replace: true
            })
        case OPEN_ENTITY.NODE_PLUGIN:
            throw navigate(management.NODE_PLUGINS.NODE_PLUGIN_BY_UUID, {
                params: { uuid: id },
                replace: true
            })
        case OPEN_ENTITY.SUBPAGE_CONFIG:
            throw navigate(subpages.SUBPAGE_CONFIG_BY_UUID, { params: { uuid: id }, replace: true })
    }
}

function requireSession({ pathname, search, params }: HooksContext): void {
    if (!session.authenticated) {
        const currentPath = window.location.pathname.slice(panelBasePath.length - 1)
        const query = currentPath === pathname ? window.location.search : serializeSearch(search)
        saveReturnTo(`${pathname}${query ?? ''}`)
        redirect(ROUTES.AUTH.LOGIN)
    }
    const normalizedPath = pathname.replace(/\/+$/, '')
    if (normalizedPath === ROUTES.DASHBOARD.ROOT) redirect(ROUTES.DASHBOARD.HOME)
    if (normalizedPath === management.ROOT) redirect(management.USERS)
    if (pathname.startsWith('/dashboard/open/')) {
        const [, entity, id] = pathname.split('/').slice(2)
        if (entity && id) resolveEntity(entity, params['id'] ?? id)
    }
    if (pathname.startsWith(`${ROUTES.DASHBOARD.TEMPLATES.ROOT}/`) && !templateType(params['type']))
        redirect(ROUTES.DASHBOARD.HOME)
}

export function returnToDashboard(): void {
    const requested = consumeReturnTo()
    if (requested) {
        window.location.replace(panelUrl(requested))
    } else {
        void navigate(ROUTES.DASHBOARD.HOME, { replace: true })
    }
}

export function signOut(): void {
    session.setToken('')
    userSelection.clear()
    queryClient.clear()
    void navigate(ROUTES.AUTH.LOGIN, { replace: true })
}
