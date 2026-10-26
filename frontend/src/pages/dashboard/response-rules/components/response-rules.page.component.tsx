import { Box, Flex } from '@mantine/core'
import type { TSubscriptionTemplateType } from '@remnawave/backend-contract'
import type { ResponseRules } from '@shared/api/schemas/subscription-settings.schema'
import { Page, PageHeaderShared } from '@shared/ui'
import { SrrAdvancedWarningOverlay } from '@shared/ui/srr-advanced-warning-overlay/srr-advanced-warning-overlay'
import { ResponseRulesEditorWidget } from '@widgets/dashboard/response-rules/response-rules-editor'
import { useTranslation } from 'react-i18next'
import { TbRoute } from 'react-icons/tb'

interface Props {
    groupedTemplates: Record<TSubscriptionTemplateType, string[]>
    responseRules: ResponseRules
    subscriptionSettingsUuid: string
}

export const ResponseRulesPageComponent = (props: Props) => {
    const { groupedTemplates, responseRules, subscriptionSettingsUuid } = props

    const { t } = useTranslation()

    return (
        <Page title={t('constants.response-rules')}>
            <PageHeaderShared icon={<TbRoute size={24} />} title={t('constants.response-rules')} />

            <SrrAdvancedWarningOverlay />

            <Flex gap="md">
                <Box style={{ flex: 1, minWidth: 0 }}>
                    <ResponseRulesEditorWidget
                        groupedTemplates={groupedTemplates}
                        responseRules={responseRules}
                        subscriptionSettingsUuid={subscriptionSettingsUuid}
                    />
                </Box>
            </Flex>
        </Page>
    )
}
