import { prettifyBytesUtil } from '@shared/utils/bytes'
import type { TFunction } from 'i18next'

import type { User } from './queries'
import type { UserTableState } from './table-state'

type Column = {
    id: string
    label: string
    value: (user: User) => string | number | null
    filter?: 'text' | 'status' | 'tag' | 'node'
    sortable?: boolean
    width?: number
}

export function userColumns(
    t: TFunction,
    language: string,
    nodes: ReadonlyMap<string, string>,
    squads: ReadonlyMap<string, string>
): Column[] {
    const dates = new Intl.DateTimeFormat(language, { dateStyle: 'short', timeStyle: 'short' })
    const date = (value: Date | null) => (value ? dates.format(value) : null)
    return [
        {
            id: 'username',
            label: t('common.field.username'),
            value: (user) => user.username,
            sortable: true,
            filter: 'text',
            width: 220
        },
        {
            id: 'id',
            label: 'ID',
            value: (user) => user.id,
            sortable: true,
            filter: 'text',
            width: 80
        },
        {
            id: 'status',
            label: t('common.field.status'),
            value: (user) => user.status,
            filter: 'status',
            width: 180
        },
        {
            id: 'nodeName',
            label: t('use-table-columns.last-connected-node'),
            value: (user) => nodes.get(user.userTraffic.lastConnectedNodeUuid ?? '') ?? null,
            filter: 'node',
            width: 180
        },
        {
            id: 'expireAt',
            label: t('use-table-columns.expire-at'),
            value: (user) => date(user.expireAt),
            sortable: true,
            width: 180
        },
        {
            id: 'usedTrafficBytes',
            label: t('use-table-columns.data-usage'),
            value: (user) => prettifyBytesUtil(user.userTraffic.usedTrafficBytes, true),
            sortable: true,
            width: 300
        },
        {
            id: 'usedTrafficPercentage',
            label: t('use-table-columns.used-traffic'),
            value: (user) =>
                user.trafficLimitBytes
                    ? ((100 * user.userTraffic.usedTrafficBytes) / user.trafficLimitBytes).toFixed(
                          1
                      )
                    : null
        },
        {
            id: 'trafficLimitBytes',
            label: t('traffic-limits-card.traffic-limit'),
            value: (user) =>
                user.trafficLimitBytes
                    ? prettifyBytesUtil(user.trafficLimitBytes, true)
                    : 'Unlimited',
            sortable: true
        },
        {
            id: 'shortUuid',
            label: t('use-table-columns.sub-link'),
            value: (user) => user.shortUuid,
            filter: 'text'
        },
        {
            id: 'description',
            label: t('common.field.description'),
            value: (user) => user.description,
            filter: 'text'
        },
        {
            id: 'telegramId',
            label: 'Telegram ID',
            value: (user) => user.telegramId,
            filter: 'text'
        },
        {
            id: 'tag',
            label: t('common.field.tag'),
            value: (user) => user.tag,
            sortable: true,
            filter: 'tag',
            width: 180
        },
        {
            id: 'activeInternalSquads',
            label: t('use-table-columns.internal-squads'),
            value: (user) =>
                user.activeInternalSquads.map((squad) => squad.name).join(', ') || null,
            width: 180
        },
        {
            id: 'externalSquadUuid',
            label: t('constants.external-squads'),
            value: (user) => squads.get(user.externalSquadUuid ?? '') ?? null
        },
        { id: 'email', label: 'Email', value: (user) => user.email, filter: 'text' },
        {
            id: 'userTraffic.firstConnectedAt',
            label: t('common.field.first-connected-at'),
            value: (user) => date(user.userTraffic.firstConnectedAt),
            width: 250
        },
        {
            id: 'lastTrafficResetAt',
            label: t('use-table-columns.traffic-reset'),
            value: (user) => date(user.lastTrafficResetAt) ?? t('use-table-columns.never'),
            width: 250
        },
        {
            id: 'userTraffic.onlineAt',
            label: t('use-table-columns.online-at'),
            value: (user) => date(user.userTraffic.onlineAt) ?? t('use-table-columns.never'),
            width: 250
        },
        {
            id: 'userTraffic.lifetimeUsedTrafficBytes',
            label: t('use-table-columns.lifetime-used'),
            value: (user) => prettifyBytesUtil(user.userTraffic.lifetimeUsedTrafficBytes, true),
            width: 170
        },
        {
            id: 'subRevokedAt',
            label: t('use-table-columns.sub-link-revoked-at'),
            value: (user) => date(user.subRevokedAt)
        },
        {
            id: 'createdAt',
            label: t('use-table-columns.created-at'),
            value: (user) => date(user.createdAt),
            sortable: true
        },
        { id: 'vlessUuid', label: 'VLESS UUID', value: (user) => user.vlessUuid, width: 235 },
        { id: 'trojanPassword', label: 'Trojan password', value: (user) => user.trojanPassword },
        { id: 'hwidDeviceLimit', label: 'HWID device limit', value: (user) => user.hwidDeviceLimit }
    ]
}

export function orderUserColumns(columns: Column[], order: string[]): Column[] {
    const byId = new Map(columns.map((column) => [column.id, column]))
    const sorted: Column[] = []
    for (const id of order) {
        const column = byId.get(id)
        if (column) {
            sorted.push(column)
            byId.delete(id)
        }
    }
    return [...sorted, ...byId.values()]
}

export function visibleUserColumns(columns: Column[], state: UserTableState) {
    const shown = orderUserColumns(columns, state.columnOrder).filter(
        (column) => state.columnVisibility[column.id] !== false
    )
    const pinnedIds = [...state.columnPinning.left, ...state.columnPinning.right]
    const leading = shown.filter((column) => state.columnPinning.left.includes(column.id))
    const middle = shown.filter((column) => !pinnedIds.includes(column.id))
    const trailing = shown.filter(
        (column) =>
            state.columnPinning.right.includes(column.id) &&
            !state.columnPinning.left.includes(column.id)
    )

    function width(column: Column): number {
        return Math.max(75, state.columnSize[column.id] ?? column.width ?? 180)
    }

    return [...leading, ...middle, ...trailing].map((column) => {
        const left = leading.indexOf(column)
        const right = trailing.indexOf(column)
        const inset =
            left >= 0
                ? `left:${60 + leading.slice(0, left).reduce((sum, item) => sum + width(item), 0)}px`
                : right >= 0
                  ? `right:${trailing.slice(right + 1).reduce((sum, item) => sum + width(item), 0)}px`
                  : undefined

        return { ...column, width: width(column), inset }
    })
}
