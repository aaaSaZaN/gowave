<script lang="ts">
    import type { GetInfraProvidersCommand } from '@remnawave/backend-contract'
    import { faviconResolver } from '@shared/utils/misc/favicon-resolver'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import SelectField from '../ui/SelectField.svelte'

    let {
        providers,
        value,
        onchoose
    }: {
        providers: GetInfraProvidersCommand.Response['response']['providers']
        value: string
        onchoose: (value: string) => void
    } = $props()
    const t = $derived(locale.t)
    const items = $derived.by(() => {
        const options = providers.map(({ uuid, name, faviconLink }) => ({
            value: uuid,
            label: name,
            avatar: {
                image: faviconResolver(faviconLink),
                initials: name
                    .split(/\s+/)
                    .slice(0, 2)
                    .map((part) => part.charAt(0))
                    .join('')
                    .toUpperCase()
            }
        }))
        if (value && !options.some((option) => option.value === value)) {
            return [...options, { value, label: value }]
        }
        return options
    })
    const url = $derived(providers.find(({ uuid }) => uuid === value)?.loginUrl)
    const loginUrl = $derived(url && /^https?:\/\//i.test(url) ? url : undefined)
</script>

<div class="provider-field">
    <SelectField
        name="providerUuid"
        label={t('select-infra-provider.shared.infrastructure-provider')}
        description={providers.length
            ? t('select-infra-provider.shared.select-the-infrastructure-provider')
            : undefined}
        placeholder={t(
            providers.length
                ? 'select-infra-provider.shared.select-provider'
                : 'select-infra-provider.shared.no-providers-available'
        )}
        options={items}
        {value}
        {onchoose}
        disabled={!items.length}
    />
    <ActionIcon
        icon="TbExternalLink"
        label={t('common.action.open')}
        href={loginUrl}
        target="_blank"
        rel="noopener noreferrer"
        disabled={!loginUrl}
        size={36}
        iconSize={18}
    />
</div>

<style>
    .provider-field {
        display: flex;
        align-items: flex-end;
        gap: var(--mantine-spacing-xs);
    }
</style>
