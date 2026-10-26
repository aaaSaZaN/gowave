import { GetRemnawaveSettingsCommand } from '@remnawave/backend-contract'

import { createQueryKeys } from '@shared/api/query-keys'
import { sToMs } from '@shared/utils/time-utils'

import { createGetQueryHook, errorHandler } from '../../tsq-helpers'

export const gowaveSettingsQueryKeys = createQueryKeys('gowaveSettings', {
    getGowaveSettings: {
        queryKey: null
    }
})

export const useGetGowaveSettings = createGetQueryHook({
    endpoint: GetRemnawaveSettingsCommand.TSQ_url,
    responseSchema: GetRemnawaveSettingsCommand.ResponseSchema,
    getQueryKey: () => gowaveSettingsQueryKeys.getGowaveSettings.queryKey,
    rQueryParams: {
        refetchOnMount: false,
        staleTime: sToMs(30)
    },
    errorHandler: (error) => errorHandler(error, 'Get Gowave Settings')
})
