import { handleRequestError } from '@shared/api/helpers/handler-request-error'
import { requestResponse } from '@shared/api/request-response'
import {
    type IUpdatesCheckResponse,
    type UpdateComponent,
    updateRepository
} from '@shared/lib/node-updates'
import { queryOptions } from '@tanstack/svelte-query'
import { isAxiosError } from 'axios'
import { z } from 'zod'

const releaseSchema = z.object({
    tag_name: z.string(),
    name: z
        .string()
        .nullish()
        .transform((name) => name ?? ''),
    prerelease: z.boolean(),
    published_at: z
        .string()
        .nullish()
        .transform((date) => date ?? ''),
    assets: z
        .array(
            z.object({
                name: z.string(),
                browser_download_url: z.string(),
                size: z.number()
            })
        )
        .nullish()
        .transform((assets) => assets ?? [])
})

const updatesResponse = z.object({
    response: z.object({
        currentVersion: z
            .string()
            .nullish()
            .transform((version) => version ?? ''),
        currentCoreVersion: z
            .string()
            .nullish()
            .transform((version) => version ?? ''),
        architecture: z
            .string()
            .nullish()
            .transform((arch) => arch ?? ''),
        os: z
            .string()
            .nullish()
            .transform((os) => os ?? ''),
        error: z
            .string()
            .nullish()
            .transform((value) => value ?? ''),
        allReleases: z
            .array(releaseSchema)
            .nullish()
            .transform((releases) => releases ?? [])
    }) satisfies z.ZodType<IUpdatesCheckResponse>
})

export type NodeUpdate =
    | { type: UpdateComponent; version: string }
    | { type: 'core'; downloadUrl: string }

function updatesUrl(nodeId: string): string {
    return `/api/nodes/${encodeURIComponent(nodeId)}/updates`
}

export function nodeReleasesQuery(id: string, component: UpdateComponent) {
    const repo = updateRepository(component)
    return queryOptions({
        queryKey: ['node-updates-check', id, repo],
        queryFn: ({ signal }) =>
            requestResponse(
                { url: updatesUrl(id), params: { repo }, signal },
                updatesResponse
            ).catch(handleRequestError),
        retry: false,
        refetchOnWindowFocus: false
    })
}

export class LocalCoreUpdateError extends Error {}

export async function installNodeUpdate(id: string, update: NodeUpdate): Promise<void> {
    const data =
        'downloadUrl' in update ? update : { ...update, repo: updateRepository(update.type) }
    const result = await requestResponse(
        { method: 'POST', url: `${updatesUrl(id)}/apply`, data, timeout: 65_000 },
        z.object({ response: z.record(z.string(), z.unknown()).nullable() })
    ).catch((error: unknown) => {
        if (
            update.type === 'core' &&
            isAxiosError<{ errorCode?: string }>(error) &&
            error.response?.status === 409 &&
            error.response.data?.errorCode === 'GW_NODE_LOCAL_CORE'
        ) {
            throw new LocalCoreUpdateError(error.message)
        }
        return handleRequestError(error)
    })
    if (result?.['success'] === false || result?.['accepted'] === false || result?.['error']) {
        const message = result['message'] ?? result['error']
        throw new Error(typeof message === 'string' ? message : 'The node rejected the update.')
    }
}

export function validHttpsDownload(value: string): boolean {
    if (!value.startsWith('https://')) return false
    try {
        return new URL(value).protocol === 'https:'
    } catch {
        return false
    }
}
