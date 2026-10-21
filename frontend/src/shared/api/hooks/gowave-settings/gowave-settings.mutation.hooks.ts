import { notifications } from '@mantine/notifications'
import { UpdateRemnawaveSettingsCommand } from '@remnawave/backend-contract'

import { createMutationHook } from '../../tsq-helpers'

export const useUpdateGowaveSettings = createMutationHook({
    endpoint: UpdateRemnawaveSettingsCommand.TSQ_url,
    bodySchema: UpdateRemnawaveSettingsCommand.RequestBodySchema,
    responseSchema: UpdateRemnawaveSettingsCommand.ResponseSchema,
    requestMethod: UpdateRemnawaveSettingsCommand.endpointDetails.REQUEST_METHOD,
    rMutationParams: {
        onSuccess: () => {
            notifications.show({
                title: 'Success',
                message: 'Gowave settings updated successfully',
                color: 'teal'
            })
        }
    }
})
