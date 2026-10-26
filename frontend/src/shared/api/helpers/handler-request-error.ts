import { isAxiosError } from 'axios'
import { ZodError } from 'zod'

import { consola } from '@shared/lib/logger'

/** Handle request errors */
export function handleRequestError(error: unknown): never {
    if (isAxiosError(error)) {
        const errorData = error.response?.data
        const enhancedError = new Error(errorData?.message || 'Request failed')
        enhancedError.cause = errorData
        throw enhancedError
    }

    if (error instanceof ZodError) {
        consola.error(error.format())
    }

    consola.log(error)

    throw error
}
