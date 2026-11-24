import type { AxiosRequestConfig } from 'axios'
import type { z } from 'zod'

import { instance } from './axios'

export async function requestResponse<T>(
    config: AxiosRequestConfig,
    schema: z.ZodType<{ response: T }>
): Promise<T> {
    const { data } = await instance.request<unknown>(config)
    return (await schema.parseAsync(data)).response
}
