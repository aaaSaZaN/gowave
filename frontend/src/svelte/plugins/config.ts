import { PluginExecutorCommand } from '@remnawave/backend-contract'
import { SharedListConfigSchema } from '@remnawave/node-plugins'
import { z } from 'zod'

const objectSchema = z.record(z.string(), z.unknown())

export function readPluginConfig(text: string) {
    return objectSchema.parse(JSON.parse(text))
}

export function readSharedListConfig(text: string) {
    return SharedListConfigSchema.parse(JSON.parse(text))
}

export type ExecutorAction = z.output<typeof PluginExecutorCommand.CommandSchema>['command']

export function executorCommand(action: ExecutorAction, text: string) {
    const lines = text
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean)
    switch (action) {
        case 'blockIps':
            return PluginExecutorCommand.CommandSchema.parse({
                command: action,
                ips: lines.map((line) => {
                    const [ip, timeout, extra] = line.split(';').map((part) => part.trim())
                    if (extra !== undefined || (timeout && !/^\d+$/.test(timeout))) {
                        throw new Error(`Expected IP;timeout: ${line}`)
                    }
                    return {
                        ip,
                        timeout: z
                            .int()
                            .nonnegative()
                            .parse(timeout ? Number(timeout) : 0)
                    }
                })
            })
        case 'unblockIps':
            return PluginExecutorCommand.CommandSchema.parse({ command: action, ips: lines })
        case 'recreateTables':
            return PluginExecutorCommand.CommandSchema.parse({ command: action })
    }
}
