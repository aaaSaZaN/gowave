import { sToMs } from '@shared/utils/time-utils'
import { QueryClient } from '@tanstack/svelte-query'

export const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            // A conditional template can start reading data after the request has completed.
            notifyOnChangeProps: 'all',
            refetchOnWindowFocus: false,
            refetchIntervalInBackground: false,
            retry: false,
            gcTime: sToMs(120),
            staleTime: sToMs(60)
        },
        mutations: {
            retry: false
        }
    }
})
