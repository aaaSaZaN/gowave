export const USERS_TABLE_STORAGE_KEY = 'x-rmnw-users-table'
export const USERS_TABLE_VERSION = 13

export const USERS_TABLE_DEFAULTS = {
    columnVisibility: {
        shortUuid: false,
        createdAt: false,
        subRevokedAt: false,
        description: false,
        telegramId: false,
        email: false,
        externalSquadUuid: false,
        vlessUuid: false,
        trojanPassword: false,
        hwidDeviceLimit: false,
        usedTrafficPercentage: false,
        trafficLimitBytes: false
    },
    columnSize: {
        shortUuid: 100,
        createdAt: 100,
        subRevokedAt: 100,
        totalUsedBytes: 100,
        onlineAt: 100
    },
    sorting: [{ id: 'id', desc: true }]
}
