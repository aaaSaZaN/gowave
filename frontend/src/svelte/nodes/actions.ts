import {
    DeleteNodeCommand,
    DisableNodeCommand,
    EnableNodeCommand,
    ResetNodeTrafficCommand,
    RestartNodeCommand
} from '@remnawave/backend-contract'

import { executeCommand } from '../commands'

export const nodeActionLabels = {
    enable: 'common.action.enable',
    disable: 'common.action.disable',
    restart: 'restart-node-button.feature.restart',
    reset: 'common.action.reset-traffic',
    delete: 'common.action.delete'
} as const

export type NodeAction = keyof typeof nodeActionLabels

export function runNodeAction(uuid: string, action: NodeAction) {
    const command = {
        enable: EnableNodeCommand,
        disable: DisableNodeCommand,
        restart: RestartNodeCommand,
        reset: ResetNodeTrafficCommand,
        delete: DeleteNodeCommand
    }[action]
    return executeCommand({ ...command, TSQ_url: command.url(uuid) })
}
