import type { TSubscriptionTemplateType } from '@remnawave/backend-contract'

import type { ResponseRules } from '@shared/api/schemas/subscription-settings.schema'

export interface IProps {
    groupedTemplates: Record<TSubscriptionTemplateType, string[]>
    responseRules: ResponseRules
    subscriptionSettingsUuid: string
}
