import {
    BulkAllExtendExpirationDateCommand,
    BulkAllResetTrafficUsersCommand,
    BulkAllUpdateUsersCommand,
    BulkDeleteUsersByStatusCommand,
    BulkDeleteUsersCommand,
    BulkExtendExpirationDateCommand,
    BulkResetTrafficUsersCommand,
    BulkRevokeUsersSubscriptionCommand,
    BulkUpdateUsersCommand,
    BulkUpdateUsersSquadsCommand,
    DeleteUserCommand,
    DisableUserCommand,
    EnableUserCommand,
    ResetUserTrafficCommand,
    RevokeUserSubscriptionCommand
} from '@remnawave/backend-contract'
import type { z } from 'zod'

import { callCommand, executeCommand, submitCommand, submitVoidCommand } from '../commands'
import type { UserStatus } from './queries'

export type BulkFields = z.input<typeof BulkAllUpdateUsersCommand.RequestBodySchema>
type BulkAction =
    | { action: 'update'; fields: BulkFields }
    | { action: 'extend'; extendDays: number }
    | { action: 'reset' }

export type BulkRequest =
    | ({ userIds: number[]; scope: 'selected' } & (
          | BulkAction
          | { action: 'delete' | 'revoke' }
          | { action: 'squads'; activeInternalSquads: string[] }
      ))
    | ({ scope: 'all' } & (BulkAction | { action: 'delete-status'; status: UserStatus }))

export function runBulkAction(request: BulkRequest): Promise<void> {
    if (request.scope === 'selected' && request.userIds.length === 0) {
        throw new Error('Select at least one user')
    }
    switch (request.action) {
        case 'reset':
            return request.scope === 'all'
                ? executeCommand(BulkAllResetTrafficUsersCommand)
                : submitVoidCommand(BulkResetTrafficUsersCommand, { userIds: request.userIds })
        case 'extend':
            return request.scope === 'all'
                ? submitVoidCommand(BulkAllExtendExpirationDateCommand, {
                      extendDays: request.extendDays
                  })
                : submitVoidCommand(BulkExtendExpirationDateCommand, {
                      userIds: request.userIds,
                      extendDays: request.extendDays
                  })
        case 'update':
            return request.scope === 'all'
                ? submitVoidCommand(BulkAllUpdateUsersCommand, request.fields)
                : submitVoidCommand(BulkUpdateUsersCommand, {
                      userIds: request.userIds,
                      fields: request.fields
                  })
        case 'delete-status':
            return submitVoidCommand(BulkDeleteUsersByStatusCommand, { status: request.status })
        case 'delete':
            return submitVoidCommand(BulkDeleteUsersCommand, { userIds: request.userIds })
        case 'revoke':
            return submitVoidCommand(BulkRevokeUsersSubscriptionCommand, {
                userIds: request.userIds
            })
        case 'squads':
            return submitVoidCommand(BulkUpdateUsersSquadsCommand, {
                userIds: request.userIds,
                activeInternalSquads: request.activeInternalSquads
            })
    }
}

export function changeUserStatus(userId: number, enable: boolean) {
    const command = enable ? EnableUserCommand : DisableUserCommand
    return callCommand({ ...command, TSQ_url: command.url(String(userId)) })
}

export function resetUserTraffic(userId: number) {
    const command = ResetUserTrafficCommand
    return callCommand({ ...command, TSQ_url: command.url(String(userId)) })
}

export function revokeSubscription(userId: number, revokeOnlyPasswords: boolean) {
    const command = RevokeUserSubscriptionCommand
    return submitCommand(
        { ...command, TSQ_url: command.url(String(userId)) },
        { revokeOnlyPasswords }
    )
}

export function deleteUser(userId: number) {
    const command = DeleteUserCommand
    return executeCommand({ ...command, TSQ_url: command.url(String(userId)) })
}
