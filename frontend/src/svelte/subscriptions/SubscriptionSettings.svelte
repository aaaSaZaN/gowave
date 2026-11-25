<script lang="ts">
    import { UpdateSubscriptionSettingsCommand } from '@remnawave/backend-contract'
    import {
        SubscriptionSettingsResponseSchema,
        UpdateSubscriptionSettingsBodySchema
    } from '@shared/api/schemas/subscription-settings.schema'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import SettingsCard from '../settings/SettingsCard.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import HelpDrawer from '../ui/HelpDrawer.svelte'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import PageHeader from '../ui/PageHeader.svelte'
    import Switch from '../ui/Switch.svelte'
    import HwidFields from './HwidFields.svelte'
    import { subscriptionSettingsQuery } from './queries'
    import RemarksFields from './RemarksFields.svelte'
    import ResponseHeaderFields from './ResponseHeaderFields.svelte'
    import {
        readHeaders,
        readHwidSettings,
        readRemarks,
        subscriptionOptions
    } from './settings-fields'

    const sections = [
        { id: 'general', label: 'subscription-settings.widget.main-settings', icon: 'PiInfo' },
        {
            id: 'remarks',
            label: 'subscription-settings.widget.custom-remarks',
            icon: 'TbListLetters'
        },
        {
            id: 'headers',
            label: 'subscription-tabs.widget.additional-response-headers',
            icon: 'TbPrescription'
        }
    ] as const
    const additional = [
        {
            key: 'serveJsonAtBaseSubscription',
            description: 'subscription-settings.widget.serve-json-description'
        },
        {
            key: 'randomizeHosts',
            description: 'subscription-tabs.widget.randomize-hosts-description'
        },
        {
            key: 'isShowCustomRemarks',
            description: 'subscription-tabs.widget.show-custom-remark-description-line-2'
        }
    ] as const
    type Tab = (typeof sections)[number]['id']
    type Section = Tab | 'hwid'

    let section = $state<Tab>('general')
    let headersHelp = $state(false)
    let remarksRevision = $state(0)
    let headersRevision = $state(0)
    let t = $derived(locale.t)
    const tabsId = $props.id()
    const settings = createQuery(() => subscriptionSettingsQuery)

    const save = createMutation(() => ({
        mutationFn: ({ data, target }: { data: FormData; target: Section }) => {
            const uuid = settings.data?.uuid
            if (!uuid) throw new Error('Subscription settings are unavailable')

            return submitCommand(
                {
                    ...UpdateSubscriptionSettingsCommand,
                    RequestBodySchema: UpdateSubscriptionSettingsBodySchema,
                    ResponseSchema: SubscriptionSettingsResponseSchema
                },
                { uuid, ...changes(target, data) }
            )
        },
        onSuccess: (updated, { target }) => {
            queryClient.setQueryData(subscriptionSettingsQuery.queryKey, updated)
            if (target === 'remarks') remarksRevision += 1
            if (target === 'headers') headersRevision += 1
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
        }
    }))

    function changes(
        target: Section,
        data: FormData
    ): Omit<UpdateSubscriptionSettingsCommand.RequestBody, 'uuid' | 'responseRules'> {
        switch (target) {
            case 'general':
                return {
                    serveJsonAtBaseSubscription: data.has('serveJsonAtBaseSubscription'),
                    randomizeHosts: data.has('randomizeHosts'),
                    isShowCustomRemarks: data.has('isShowCustomRemarks')
                }
            case 'hwid':
                return { hwidSettings: readHwidSettings(data) }
            case 'remarks':
                return { customRemarks: readRemarks(data) }
            case 'headers':
                return { customResponseHeaders: readHeaders(data) }
        }
    }

    function selectSection(next: Tab): void {
        if (section === next) return
        section = next
        save.reset()
    }

    function navigateTabs(event: KeyboardEvent, current: number): void {
        const rtl = document.documentElement.dir === 'rtl'
        const last = sections.length - 1
        let next: number
        switch (event.key) {
            case 'ArrowLeft':
                next = (current + (rtl ? 1 : last)) % sections.length
                break
            case 'ArrowRight':
                next = (current + (rtl ? last : 1)) % sections.length
                break
            case 'Home':
                next = 0
                break
            case 'End':
                next = last
                break
            default:
                return
        }
        const tab = sections[next]
        if (!tab || save.isPending) return
        event.preventDefault()
        selectSection(tab.id)
        document.getElementById(`${tabsId}-tab-${tab.id}`)?.focus()
    }

    function submit(event: SubmitEvent, target: Section): void {
        event.preventDefault()
        if (save.isPending || !(event.currentTarget instanceof HTMLFormElement)) return
        save.mutate({ data: new FormData(event.currentTarget), target })
    }
</script>

<svelte:head><title>{t('constants.subscription-settings')} | Gowave</title></svelte:head>

{#snippet saveButton()}
    <Button type="submit" size="md" variant="soft" color="teal" pending={save.isPending}
        >{t('common.action.save')}</Button
    >
{/snippet}

<main class="subscription-page">
    {#if settings.isPending}
        <RouteLoading label={t('common.message.loading')} />
    {:else}
        <PageHeader title={t('constants.subscription-settings')} icon="TbHexagon" />
        {#if settings.isError}
            <p role="alert">{settings.error.message}</p>
            <Button onclick={() => void settings.refetch()}>{t('common.action.try-again')}</Button>
        {:else}
            {@const current = settings.data}
            <div role="tablist" aria-label={t('constants.subscription-settings')} class="tabs">
                {#each sections as item, index (item.id)}
                    <button
                        type="button"
                        class="m_4ec4dce6 mantine-UnstyledButton-root settings-tab"
                        role="tab"
                        id={`${tabsId}-tab-${item.id}`}
                        disabled={save.isPending}
                        aria-selected={section === item.id}
                        aria-controls={`${tabsId}-panel-${item.id}`}
                        tabindex={section === item.id ? 0 : -1}
                        onclick={() => selectSection(item.id)}
                        onkeydown={(event) => navigateTabs(event, index)}
                    >
                        <Icon name={item.icon} size="1.2rem" />
                        {t(item.label)}
                    </button>
                {/each}
            </div>
            {#each sections as item (item.id)}
                <div
                    role="tabpanel"
                    id={`${tabsId}-panel-${item.id}`}
                    aria-labelledby={`${tabsId}-tab-${item.id}`}
                    hidden={section !== item.id}
                >
                    {#if item.id === 'general'}
                        <div class="main-cards">
                            <form onsubmit={(event) => submit(event, 'general')}>
                                <SettingsCard
                                    title={t('subscription-tabs.widget.additional-options')}
                                    description={t(
                                        'subscription-tabs.widget.configure-additional-subscription-options'
                                    )}
                                    icon="PiGear"
                                >
                                    {#snippet footer()}
                                        <div class="save">{@render saveButton()}</div>
                                    {/snippet}
                                    <fieldset disabled={save.isPending}>
                                        {#each additional as { key, description } (key)}
                                            <Switch
                                                label={t(subscriptionOptions[key])}
                                                description={t(description)}
                                                switchSize="sm"
                                                tone="cyan"
                                                name={key}
                                                checked={current[key]}
                                            />
                                        {/each}
                                    </fieldset>
                                </SettingsCard>
                            </form>
                            <form onsubmit={(event) => submit(event, 'hwid')}>
                                <SettingsCard
                                    title={t('subscription-hwid-settings.widget.hwid-settings')}
                                    description={t(
                                        'subscription-hwid-settings.widget.hwid-card-description'
                                    )}
                                    icon="PiIdentificationBadge"
                                >
                                    {#snippet footer()}
                                        <div class="save">
                                            <a
                                                class="documentation"
                                                href="https://docs.rw/docs/features/hwid-device-limit"
                                                target="_blank"
                                                rel="noopener noreferrer"
                                                ><Icon name="PiGear" size="1.2rem" />
                                                {t('common.action.documentation')}</a
                                            >
                                            {@render saveButton()}
                                        </div>
                                    {/snippet}
                                    <fieldset disabled={save.isPending}>
                                        <HwidFields value={current.hwidSettings} defaultLimit={0} />
                                    </fieldset>
                                </SettingsCard>
                            </form>
                        </div>
                    {:else if item.id === 'remarks'}
                        <form class="editable-card" onsubmit={(event) => submit(event, 'remarks')}>
                            <SettingsCard
                                title={t('subscription-settings.widget.custom-remarks')}
                                description={`${t('subscription-settings.widget.user-status-remarks-description')}\n${t('subscription-settings.widget.user-status-remarks-description-line-2')}`}
                                icon="TbListLetters"
                            >
                                {#snippet footer()}
                                    <div class="save">{@render saveButton()}</div>
                                {/snippet}
                                <fieldset disabled={save.isPending}>
                                    {#key remarksRevision}
                                        <RemarksFields remarks={current.customRemarks} />
                                    {/key}
                                </fieldset>
                            </SettingsCard>
                        </form>
                    {:else}
                        <form class="editable-card" onsubmit={(event) => submit(event, 'headers')}>
                            {#key headersRevision}
                                <ResponseHeaderFields
                                    headers={current.customResponseHeaders}
                                    compact
                                >
                                    {#snippet children(fields, addHeader)}
                                        <SettingsCard
                                            title={t(
                                                'subscription-tabs.widget.additional-response-headers'
                                            )}
                                            description={t(
                                                'subscription-tabs.widget.headers-that-will-be-sent-with-subscription-content'
                                            )}
                                            icon="PiChatsCircle"
                                        >
                                            <fieldset
                                                class="header-fields"
                                                disabled={save.isPending}
                                            >
                                                {@render fields()}
                                            </fieldset>
                                            {#snippet footer()}
                                                <div class="save">
                                                    <ActionIcon
                                                        icon="TbQuestionMark"
                                                        tone="lime"
                                                        label={t(
                                                            'help-action-icon.shared.help-article'
                                                        )}
                                                        onclick={() => (headersHelp = true)}
                                                    />
                                                    <Button
                                                        size="md"
                                                        variant="soft"
                                                        icon="TbPlus"
                                                        disabled={save.isPending}
                                                        onclick={addHeader}
                                                    >
                                                        {t('headers-manager.widget.add-header')}
                                                    </Button>
                                                    {@render saveButton()}
                                                </div>
                                            {/snippet}
                                        </SettingsCard>
                                    {/snippet}
                                </ResponseHeaderFields>
                            {/key}
                        </form>
                    {/if}
                </div>
            {/each}
            {#if save.error}
                <p role="alert">{save.error.message}</p>
            {/if}
        {/if}
    {/if}
</main>

{#if headersHelp}
    <HelpDrawer screen="PAGE_RESPONSE_HEADERS" close={() => (headersHelp = false)} />
{/if}

<style>
    .subscription-page,
    .main-cards,
    fieldset {
        display: grid;
        gap: var(--mantine-spacing-md);
        min-width: 0;
    }
    .main-cards {
        align-items: start;
    }
    .editable-card {
        max-width: 1500px;
    }
    .tabs {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
        margin-bottom: var(--mantine-spacing-md);
    }
    .settings-tab {
        display: inline-flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        background: var(--gw-raised-bg);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        white-space: nowrap;
        transition:
            color 150ms ease,
            background-color 150ms ease,
            border-color 150ms ease;
    }
    .settings-tab:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }
    .settings-tab:hover {
        background: var(--mantine-color-default-hover);
        border-color: var(--mantine-color-default-border);
    }
    .settings-tab[aria-selected="true"] {
        color: var(--mantine-primary-color-4);
        background: color-mix(in srgb, var(--mantine-primary-color-6) 12%, transparent);
        border-color: color-mix(in srgb, var(--mantine-primary-color-6) 30%, transparent);
    }
    fieldset {
        margin: 0;
        padding: 0;
        border: 0;
    }
    .save {
        display: flex;
        flex-wrap: wrap;
        justify-content: end;
        align-items: center;
        gap: var(--mantine-spacing-md);
    }
    .header-fields {
        gap: var(--mantine-spacing-xs);
        padding: var(--mantine-spacing-lg);
        margin-inline: calc(-1 * var(--mantine-spacing-md));
    }
    .header-fields:not(:empty) {
        padding-block-end: calc(var(--mantine-spacing-lg) + var(--mantine-spacing-xs));
    }
    .documentation {
        display: inline-flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        min-height: 42px;
        padding-inline: 22px;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-grape-light);
        color: var(--mantine-color-grape-light-color);
        font-weight: 600;
        text-decoration: none;
    }
    .documentation:hover {
        background: var(--mantine-color-grape-light-hover);
    }
    @media (min-width: 1400px) {
        .main-cards {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    @media (min-width: 2000px) {
        .main-cards {
            grid-template-columns: repeat(3, minmax(0, 1fr));
        }
    }
    @media (min-width: 3000px) {
        .main-cards {
            grid-template-columns: repeat(4, minmax(0, 1fr));
        }
    }
</style>
