import { notifications } from '@mantine/notifications'
import { UpdateSubscriptionSettingsCommand } from '@remnawave/backend-contract'

import {
    SubscriptionSettingsResponseSchema,
    UpdateSubscriptionSettingsBodySchema
} from '../../schemas/subscription-settings.schema'
import { createMutationHook } from '../../tsq-helpers'

export const useUpdateSubscriptionSettings = createMutationHook({
    endpoint: UpdateSubscriptionSettingsCommand.TSQ_url,
    bodySchema: UpdateSubscriptionSettingsBodySchema,
    responseSchema: SubscriptionSettingsResponseSchema,
    requestMethod: UpdateSubscriptionSettingsCommand.endpointDetails.REQUEST_METHOD,
    rMutationParams: {
        onSuccess: () => {
            notifications.show({
                title: 'Success',
                message: 'Subscription settings updated successfully',
                color: 'teal'
            })
        },
        onError: (error) => {
            notifications.show({
                title: 'Update Subscription Settings',
                message:
                    error instanceof Error ? error.message : `Request failed with unknown error.`,
                color: 'red'
            })
        }
    }
})
