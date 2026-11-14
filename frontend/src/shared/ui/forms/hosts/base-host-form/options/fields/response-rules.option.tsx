import { TagsInput } from '@mantine/core'
import { useGetSubscriptionSettings } from '@shared/api/hooks'
import { useSettingsRowControl } from '@shared/ui/settings-row'
import { useTranslation } from 'react-i18next'

import { useHostFormData } from '../host-form-data.context'

export function ResponseRulesOption() {
    const { form } = useHostFormData()
    const { data: settings } = useGetSubscriptionSettings()
    const rowControl = useSettingsRowControl()
    const { t } = useTranslation()

    return (
        <TagsInput
            clearable
            data={[...new Set(settings?.responseRules?.rules.map((rule) => rule.name) ?? [])]}
            isDuplicate={(value, values) => values.includes(value)}
            key={form.key('includeInSrrRules')}
            placeholder={t('base-host-form.response-rules-placeholder')}
            splitChars={[]}
            w="100%"
            {...rowControl}
            {...form.getInputProps('includeInSrrRules')}
        />
    )
}
