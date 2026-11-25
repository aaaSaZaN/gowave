<script lang="ts">
    import { DeleteApiTokenCommand } from '@remnawave/backend-contract'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { executeCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Icon from '../ui/Icon.svelte'
    import { type ApiToken, apiTokens } from './queries'
    import SettingsCard from './SettingsCard.svelte'
    import TokenDialog from './TokenDialog.svelte'

    let t = $derived(locale.t)
    let selected = $state<ApiToken | null>()
    let dates = $derived(
        new Intl.DateTimeFormat(locale.language, { dateStyle: 'medium', timeStyle: 'short' })
    )
    const tokens = createQuery(() => apiTokens)
    const remove = createMutation(() => ({
        mutationFn: (id: string) => {
            const command = DeleteApiTokenCommand
            const { uuid } = command.RequestParamSchema.parse({ uuid: id })
            return executeCommand({ ...command, TSQ_url: command.url(uuid) })
        },
        onSuccess: () => queryClient.invalidateQueries({ queryKey: apiTokens.queryKey })
    }))
</script>

<SettingsCard
    title={t('api-tokens-card.widget.api-tokens')}
    description={t('api-tokens-card.widget.api-tokens-description')}
    icon="TbCookie"
>
    {#snippet footer()}
        <div class="token-actions">
            <ActionIcon
                icon="TbRefresh"
                tone="cyan"
                label={t('common.action.refresh')}
                disabled={tokens.isFetching}
                onclick={() => void tokens.refetch()}
            />
            <ActionIcon
                icon="TbPlus"
                tone="teal"
                label={t('api-tokens-card.widget.create-api-token')}
                onclick={() => (selected = null)}
            />
        </div>
    {/snippet}
    {#if tokens.isPending}
        <p role="status">{t('common.message.loading')}</p>
    {/if}
    {#if tokens.error || remove.error}
        <p role="alert">{tokens.error?.message || remove.error?.message}</p>
    {/if}
    {#if tokens.data?.tokens.length}
        <ul>
            {#each tokens.data.tokens as token (token.uuid)}
                <li>
                    <h3>{token.name}</h3>
                    <p>
                        {token.scopes.includes('*')
                            ? t('api-tokens-card.widget.full-access')
                            : token.scopes.join(', ')}
                        · {t('api-tokens-card.widget.col-expires')}
                        {dates.format(new Date(token.expireAt))}
                        {new Date(token.expireAt).getTime() < Date.now()
                            ? ` · ${t('api-tokens-card.widget.expired')}`
                            : ''}
                    </p>
                    <div class="button-row">
                        <button type="button" onclick={() => (selected = token)}>
                            {t('common.action.view')}
                        </button>
                        <button
                            type="button"
                            disabled={remove.isPending}
                            onclick={() => {
                                if (window.confirm(t('common.message.confirm-action-description')))
                                    remove.mutate(token.uuid)
                            }}
                        >
                            {t('common.action.delete')}
                        </button>
                    </div>
                </li>
            {/each}
        </ul>
    {:else if tokens.data}
        <div class="no-tokens">
            <Icon name="PiEmpty" size={48} />
            <p>{t('api-tokens-card.widget.no-api-tokens-found')}</p>
        </div>
    {/if}
</SettingsCard>
{#if selected !== undefined}
    <TokenDialog token={selected} close={() => (selected = undefined)} />
{/if}

<style>
    .token-actions {
        display: flex;
        justify-content: space-between;
        align-items: center;
    }
    .no-tokens {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: var(--mantine-spacing-xs);
        height: 300px;
    }
    li p {
        overflow-wrap: anywhere;
    }
    .no-tokens p {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        margin: 0;
    }
    ul {
        list-style: none;
        padding: 0;
    }
    li {
        border-block-start: 1px solid var(--gw-border);
        padding: 0.65rem 0;
    }
    h3 {
        margin: 0;
        font-size: 1rem;
    }
</style>
