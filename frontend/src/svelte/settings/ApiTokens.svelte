<script lang="ts">
    import { DeleteApiTokenCommand } from '@remnawave/backend-contract'
    import { formatTimeUtil } from '@shared/utils/time-utils'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { executeCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import ActionMenu from '../ui/ActionMenu.svelte'
    import { useConfirmation } from '../ui/confirmation'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Tooltip from '../ui/Tooltip.svelte'
    import { type ApiToken, apiTokens } from './queries'
    import SettingsCard from './SettingsCard.svelte'
    import TokenDialog from './TokenDialog.svelte'

    const confirmAction = useConfirmation()

    let t = $derived(locale.t)
    let selected = $state<ApiToken | null>()
    const tokens = createQuery(() => apiTokens)
    const remove = createMutation(() => ({
        mutationFn: (id: string) => {
            const command = DeleteApiTokenCommand
            const { uuid } = command.RequestParamSchema.parse({ uuid: id })
            return executeCommand({ ...command, TSQ_url: command.url(uuid) })
        },
        onSuccess: async () => {
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
            await queryClient.invalidateQueries({ queryKey: apiTokens.queryKey })
        }
    }))

    function dotColor(token: ApiToken): string {
        if (token.scopes.includes('*')) return 'var(--mantine-color-teal-5)'
        if (token.scopes.length > 0) return 'var(--mantine-color-cyan-5)'
        return 'var(--mantine-color-dark-3)'
    }

    function expired(token: ApiToken): boolean {
        return new Date(token.expireAt).getTime() < Date.now()
    }
</script>

<SettingsCard
    title={t('api-tokens-card.widget.api-tokens')}
    description={t('api-tokens-card.widget.api-tokens-description')}
    icon="TbCookie"
>
    {#snippet footer()}
        <div class="token-actions">
            <Tooltip text={t('common.action.refresh')}>
                <ActionIcon
                    icon="TbRefresh"
                    tone="gray"
                    size={36}
                    iconSize={22}
                    label={t('common.action.refresh')}
                    pending={tokens.isFetching}
                    onclick={() => void tokens.refetch()}
                />
            </Tooltip>
            <Tooltip text={t('common.action.create')}>
                <ActionIcon
                    icon="TbPlus"
                    tone="teal"
                    size={36}
                    iconSize={22}
                    label={t('api-tokens-card.widget.create-api-token')}
                    onclick={() => (selected = null)}
                />
            </Tooltip>
        </div>
    {/snippet}

    {#if tokens.isPending}
        <RouteLoading height="300px" label={t('common.message.loading')} />
    {:else if tokens.error || remove.error}
        <p role="alert">{tokens.error?.message || remove.error?.message}</p>
    {:else if tokens.data?.tokens.length}
        <div class="token-table">
            <div class="token-header-row">
                <span class="col-label">{t('common.field.name')}</span>
                <span class="col-label">{t('api-tokens-card.widget.col-scopes')}</span>
                <span class="col-label expires-col">{t('api-tokens-card.widget.col-expires')}</span>
                <span></span>
            </div>
            <div class="token-scroll">
                {#each tokens.data.tokens as token (token.uuid)}
                    {@const isFull = token.scopes.includes('*')}
                    {@const isExpired = expired(token)}
                    <div class="token-row">
                        <div class="name-cell">
                            <span
                                class="status-dot"
                                style:background={dotColor(token)}
                                aria-hidden="true"
                            ></span>
                            <span class="token-name">{token.name}</span>
                        </div>
                        <span class="scopes-cell">
                            {isFull ? t('api-tokens-card.widget.full-access') : token.scopes.length}
                        </span>
                        <span class="expires-col expires-cell" class:expired={isExpired}>
                            {isExpired
                                ? t('api-tokens-card.widget.expired')
                                : formatTimeUtil({
                                      time: token.expireAt,
                                      template: 'TIME_FIRST_DATETIME',
                                      language: locale.language
                                  })}
                        </span>
                        <ActionMenu
                            icon="TbDots"
                            iconSize={18}
                            size={28}
                            subtle
                            hover={false}
                            alignEnd
                            label={t('common.action.actions')}
                        >
                            <button
                                type="button"
                                role="menuitem"
                                tabindex="-1"
                                onclick={() => (selected = token)}
                            >
                                <Icon name="TbEye" size={15} />
                                {t('common.action.view')}
                            </button>
                            <hr>
                            <button
                                type="button"
                                role="menuitem"
                                tabindex="-1"
                                class="danger"
                                disabled={remove.isPending}
                                onclick={async () => {
                                    if (
                                        await confirmAction(
                                            t('common.message.confirm-action-description')
                                        )
                                    )
                                        remove.mutate(token.uuid)
                                }}
                            >
                                <Icon name="TbTrash" size={15} />
                                {t('common.action.delete')}
                            </button>
                        </ActionMenu>
                    </div>
                {/each}
            </div>
        </div>
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

    .token-table {
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        overflow: hidden;
        background: rgba(var(--gw-fg-rgb), 0.02);
    }

    .token-header-row,
    .token-row {
        display: grid;
        grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr) minmax(0, 1fr) 44px;
        gap: 16px;
        align-items: center;
        padding: 11px 16px;
    }

    @media (max-width: 48em) {
        .token-header-row,
        .token-row {
            grid-template-columns: minmax(0, 1.5fr) minmax(0, 1fr) 44px;
            gap: 10px;
        }
        .expires-col {
            display: none;
        }
    }

    .token-header-row {
        border-bottom: 1px solid rgba(var(--gw-fg-rgb), 0.06);
    }

    .col-label {
        font-size: 11px;
        font-family: var(--mantine-font-family-monospace);
        letter-spacing: 0.04em;
        text-transform: uppercase;
        color: var(--mantine-color-dark-2);
    }

    .token-scroll {
        min-height: 300px;
        max-height: 300px;
        overflow-y: auto;
    }

    .token-row {
        border-bottom: 1px solid rgba(var(--gw-fg-rgb), 0.04);
        transition: background-color 120ms ease;
    }

    .token-row:last-child {
        border-bottom: none;
    }

    .token-row:hover {
        background-color: rgba(var(--gw-fg-rgb), 0.02);
    }

    .name-cell {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        min-width: 0;
    }

    .status-dot {
        width: 8px;
        height: 8px;
        flex-shrink: 0;
        border-radius: 50%;
    }

    .token-name {
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        color: var(--mantine-color-text);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .scopes-cell {
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-xs);
        color: var(--mantine-color-dimmed);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .expires-cell {
        font-size: var(--mantine-font-size-xs);
        color: var(--mantine-color-dimmed);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .expires-cell.expired {
        color: var(--mantine-color-red-5);
        font-weight: 600;
    }

    .no-tokens {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: var(--mantine-spacing-xs);
        height: 300px;
        color: var(--mantine-color-dimmed);
    }

    .no-tokens p {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        margin: 0;
    }

    hr {
        width: 100%;
        height: 1px;
        margin: 4px 0;
        border: 0;
        background: rgba(var(--gw-fg-rgb), 0.08);
    }

    button.danger {
        color: var(--mantine-color-red-4);
    }
</style>
