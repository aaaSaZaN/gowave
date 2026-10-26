import type {
    GetSubscriptionTemplatesCommand,
    TSubscriptionTemplateType
} from '@remnawave/backend-contract'

export interface IProps {
    templates: GetSubscriptionTemplatesCommand.Response['response']['templates']
    type: TSubscriptionTemplateType
}
