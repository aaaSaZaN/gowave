import { notifications } from '@mantine/notifications'
import logger from './logger'

export function reportAsyncError(error: unknown): void {
    logger.error('Asynchronous operation failed', error)
    notifications.show({
        color: 'red',
        title: 'Operation failed',
        message: error instanceof Error ? error.message : 'The operation could not be completed.'
    })
}

// Start immediately to retain clipboard/share user gestures, and always observe
// rejection. Router navigation also supports a synchronous void result.
// biome-ignore lint/suspicious/noConfusingVoidType: Router navigation returns void synchronously or a Promise in data routers.
type AsyncResult = PromiseLike<unknown> | void

export function runAsync(operation: AsyncResult): void {
    void Promise.resolve(operation).catch(reportAsyncError)
}
