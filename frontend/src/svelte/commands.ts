import { instance } from '@shared/api/axios'
import { handleRequestError } from '@shared/api/helpers/handler-request-error'
import { requestResponse } from '@shared/api/request-response'
import type { AxiosRequestConfig } from 'axios'
import type { z } from 'zod'

type Command = {
    endpointDetails: { REQUEST_METHOD: string }
    TSQ_url: string
}

type ResponseCommand<T> = Command & {
    ResponseSchema: z.ZodType<{ response: T }>
}

type Body = { RequestBodySchema: z.ZodType }

export function executeCommand(
    command: Command,
    options?: Omit<AxiosRequestConfig, 'method' | 'url'>
): Promise<undefined> {
    return instance
        .request<unknown>({
            ...options,
            url: command.TSQ_url,
            method: command.endpointDetails.REQUEST_METHOD
        })
        .then(() => undefined)
        .catch(handleRequestError)
}

export function callCommand<T>(
    command: ResponseCommand<T>,
    options?: Omit<AxiosRequestConfig, 'method' | 'url'>
): Promise<T> {
    const { endpointDetails, TSQ_url, ResponseSchema } = command
    return requestResponse(
        { ...options, url: TSQ_url, method: endpointDetails.REQUEST_METHOD },
        ResponseSchema
    ).catch(handleRequestError)
}

export function submitCommand<T>(command: ResponseCommand<T> & Body, values: unknown): Promise<T> {
    return callCommand(command, { data: command.RequestBodySchema.parse(values) })
}

export function submitVoidCommand(command: Command & Body, values: unknown): Promise<void> {
    return executeCommand(command, { data: command.RequestBodySchema.parse(values) })
}
