<script lang="ts">
    import { CreateApiTokenCommand } from '@remnawave/backend-contract'
    import {
        buildScopes,
        countSelected,
        expandScopesToKeys,
        getReadKeys,
        getWriteKeys,
        humanizeResource
    } from '@shared/lib/api-token-scopes'
    import { formatTimeUtil } from '@shared/utils/time-utils'
    import { createQuery } from '@tanstack/svelte-query'
    import { SvelteSet } from 'svelte/reactivity'
    import { submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import CopyableField from '../ui/CopyableField.svelte'
    import { dismissDialog } from '../ui/dialog-motion'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import { type ApiToken, apiScopes, apiTokens } from './queries'

    let { token, close }: { token: ApiToken | null; close: () => void } = $props()
    const scopes = createQuery(() => apiScopes)
    let resources = $derived(scopes.data?.resources ?? [])
    let selected = $state(new Set<string>())
    const expanded = new SvelteSet<string>()
    let requestedScopes = $derived(buildScopes(resources, selected))
    let name = $state('')
    let expires = $state<number | undefined>(30)
    let issued = $state.raw<CreateApiTokenCommand.Response['response']>()
    let current = $derived(issued ?? token)
    let granted = $derived(new Set(expandScopesToKeys(resources, current?.scopes ?? [])))
    let t = $derived(locale.t)
    let isExpired = $derived(current ? new Date(current.expireAt).getTime() < Date.now() : false)
    let noticeParts = $derived(t('api-tokens-card.widget.one-time-notice').split(/<\/?emphasis>/g))
    let busy = $state(false)
    let error = $state('')

    function toggle(keys: string[], checked: boolean): void {
        const next = new Set(selected)
        for (const key of keys) {
            if (checked) next.add(key)
            else next.delete(key)
        }
        selected = next
    }

    function toggleResource(resource: string): void {
        if (expanded.has(resource)) expanded.delete(resource)
        else expanded.add(resource)
    }

    function preset(kind: 'all' | 'read' | 'page' | 'clear'): void {
        const pages = [
            'subscription-page-configs:list',
            'subscription-page-configs:get',
            'subscriptions:subpage-config',
            'system:metadata',
            'users:by-username'
        ]
        const endpoints = resources.flatMap(({ endpoints }) => endpoints)
        selected = new Set(
            endpoints
                .filter(
                    ({ key, kind: type }) =>
                        kind === 'all' ||
                        (kind === 'read' && type === 'read') ||
                        (kind === 'page' && pages.includes(key))
                )
                .map(({ key }) => key)
        )
    }

    async function pasteScopes(): Promise<void> {
        error = ''
        try {
            const value: unknown = JSON.parse(await navigator.clipboard.readText())
            if (
                !Array.isArray(value) ||
                !value.every((scope): scope is string => typeof scope === 'string')
            ) {
                throw new Error(t('api-tokens-card.widget.invalid-scopes'))
            }
            selected = new Set(expandScopesToKeys(resources, value))
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
        } catch (reason) {
            error = reason instanceof Error ? reason.message : String(reason)
        }
    }

    function copy(value: string): void {
        error = ''
        void navigator.clipboard.writeText(value).then(
            () =>
                notifications.show({
                    title: t('common.message.copied'),
                    message: value,
                    color: 'teal'
                }),
            (reason: unknown) => (error = reason instanceof Error ? reason.message : String(reason))
        )
    }

    function requestClose(event: MouseEvent): void {
        if (busy) return
        const dialog =
            event.currentTarget instanceof Element ? event.currentTarget.closest('dialog') : null
        if (dialog instanceof HTMLDialogElement) dismissDialog(dialog)
        else close()
    }

    async function create(event: SubmitEvent): Promise<void> {
        event.preventDefault()
        if (busy || !requestedScopes.length || !Number.isSafeInteger(expires)) return
        busy = true
        error = ''
        try {
            issued = await submitCommand(CreateApiTokenCommand, {
                name: name.trim(),
                expiresInDays: expires,
                scopes: requestedScopes
            })
            await queryClient.invalidateQueries({ queryKey: apiTokens.queryKey })
        } catch (reason) {
            error = reason instanceof Error ? reason.message : String(reason)
        } finally {
            busy = false
        }
    }
</script>

<Overlay
    title={current?.name ?? t('api-tokens-card.widget.create-api-token')}
    icon="TbCookie"
    tone="teal"
    width={800}
    preventClose={busy}
    {close}
>
    <div class="token-dialog-body">
        {#if scopes.error || error}
            <p class="error-banner" role="alert">{error || scopes.error?.message}</p>
        {/if}
        {#if current}
            <div class="meta-grid">
                <div class="meta-cell">
                    <span class="meta-label">{t('api-tokens-card.widget.col-created')}</span>
                    <span class="meta-value">
                        {formatTimeUtil({
                            time: current.createdAt,
                            template: 'TIME_FIRST_DATETIME',
                            language: locale.language
                        })}
                    </span>
                </div>
                <div class="meta-cell">
                    <span class="meta-label">{t('api-tokens-card.widget.col-expires')}</span>
                    <span class="meta-value" class:expired={isExpired}>
                        {formatTimeUtil({
                            time: current.expireAt,
                            template: 'TIME_FIRST_DATETIME',
                            language: locale.language
                        })}
                    </span>
                </div>
                <div class="meta-cell">
                    <span class="meta-label">UUID</span>
                    <div class="meta-uuid">
                        <span class="meta-uuid-text">{current.uuid}</span>
                        <ActionIcon
                            icon="TbCopy"
                            size={24}
                            iconSize={14}
                            subtle
                            label={t('common.action.copy-uuid')}
                            onclick={() => copy(current.uuid)}
                        />
                    </div>
                </div>
            </div>

            {#if issued}
                <div class="one-time-notice">
                    <span class="one-time-icon" aria-hidden="true">
                        <Icon name="TbAlertTriangle" size={18} />
                    </span>
                    <p class="one-time-text">
                        {#each noticeParts as part, index}
                            {#if index % 2 === 1}
                                <span class="one-time-emphasis">{part}</span>
                            {:else}
                                {part}
                            {/if}
                        {/each}
                    </p>
                </div>
                <CopyableField
                    label={t('api-tokens-card.widget.token')}
                    value={issued.token}
                    leftIcon="TbKey"
                />
            {/if}

            <div class="resources-list">
                {#each resources.filter((resource) =>
                    countSelected(resource.endpoints, granted)
                ) as resource (resource.resource)}
                    {@const open = expanded.has(resource.resource)}
                    <div class="resource-row">
                        <div class="resource-header">
                            <button
                                type="button"
                                class="resource-trigger mantine-UnstyledButton-root"
                                aria-expanded={open}
                                onclick={() => toggleResource(resource.resource)}
                            >
                                <span class="chevron" class:open>
                                    <Icon name="TbChevronRight" size={14} />
                                </span>
                                <span class="resource-name"
                                    >{humanizeResource(resource.resource)}</span
                                >
                                <span class="count-pill">
                                    {countSelected(resource.endpoints, granted)}
                                </span>
                            </button>
                        </div>
                        {#if open}
                            <ul class="endpoint-list">
                                {#each resource.endpoints.filter(({ key }) =>
                                    granted.has(key)
                                ) as endpoint (endpoint.key)}
                                    <li class="endpoint-row readonly">
                                        <span class="method-badge">{endpoint.method}</span>
                                        <code class="endpoint-path">{endpoint.path}</code>
                                        <span class="endpoint-desc">{endpoint.description}</span>
                                    </li>
                                {/each}
                            </ul>
                        {/if}
                    </div>
                {/each}
            </div>

            <div class="footer-actions">
                <ActionIcon
                    icon="TbCopy"
                    size={36}
                    iconSize={20}
                    tone="gray"
                    label={t('api-tokens-card.widget.copy-scopes')}
                    onclick={() => copy(JSON.stringify(current.scopes, null, 2))}
                />
                <Button color="teal" variant="light" size="md" onclick={requestClose}>
                    {t('common.action.close')}
                </Button>
            </div>
        {:else}
            <form class="token-form" onsubmit={(event) => void create(event)}>
                <fieldset disabled={busy}>
                    <div class="form-row">
                        <label class="field-label">
                            <span>{t('api-tokens-card.widget.token-name')}</span>
                            <input
                                name="tokenName"
                                class="m_8fb7ebe7 mantine-Input-input text-input"
                                required
                                minlength="2"
                                maxlength="30"
                                bind:value={name}
                            >
                        </label>
                        <label class="field-label">
                            <span>{t('api-tokens-card.widget.expires-in-days')}</span>
                            <input
                                name="expiresInDays"
                                class="m_8fb7ebe7 mantine-Input-input text-input"
                                type="number"
                                required
                                min="1"
                                step="1"
                                bind:value={expires}
                            >
                        </label>
                    </div>
                    {#if scopes.isPending}
                        <RouteLoading compact />
                    {/if}
                    <div class="button-row">
                        <Button variant="default" size="xs" onclick={() => preset('read')}>
                            {t('api-tokens-card.widget.read-only')}
                        </Button>
                        <Button variant="default" size="xs" onclick={() => preset('all')}>
                            {t('api-tokens-card.widget.full-access')}
                        </Button>
                        <Button variant="default" size="xs" onclick={() => preset('page')}>
                            {t('constants.subscription-page')}
                        </Button>
                        <Button variant="default" size="xs" onclick={() => preset('clear')}>
                            {t('api-tokens-card.widget.clear-scopes')}
                        </Button>
                        <Button variant="default" size="xs" onclick={() => void pasteScopes()}>
                            {t('api-tokens-card.widget.paste-scopes')}
                        </Button>
                    </div>
                    <div class="resources-list">
                        {#each resources as resource (resource.resource)}
                            {@const open = expanded.has(resource.resource)}
                            <div class="resource-row">
                                <div class="resource-header">
                                    <button
                                        type="button"
                                        class="resource-trigger mantine-UnstyledButton-root"
                                        aria-expanded={open}
                                        onclick={() => toggleResource(resource.resource)}
                                    >
                                        <span class="chevron" class:open>
                                            <Icon name="TbChevronRight" size={14} />
                                        </span>
                                        <span class="resource-name"
                                            >{humanizeResource(resource.resource)}</span
                                        >
                                        <span class="count-pill">
                                            {countSelected(resource.endpoints, selected)}
                                        </span>
                                    </button>
                                    <div class="kind-pills">
                                        {#each [
                                            {
                                                keys: getReadKeys(resource),
                                                label: t('api-tokens-card.widget.read')
                                            },
                                            {
                                                keys: getWriteKeys(resource),
                                                label: t('api-tokens-card.widget.write')
                                            }
                                        ] as group (group.label)}
                                            {#if group.keys.length}
                                                {@const allChecked = group.keys.every((key) =>
                                                    selected.has(key)
                                                )}
                                                <label class="kind-pill" class:active={allChecked}>
                                                    <input
                                                        type="checkbox"
                                                        checked={allChecked}
                                                        indeterminate={group.keys.some((key) =>
                                                            selected.has(key)
                                                        ) && !allChecked}
                                                        onchange={(event) =>
                                                            toggle(
                                                                group.keys,
                                                                event.currentTarget.checked
                                                            )}
                                                    >
                                                    <span>{group.label}</span>
                                                </label>
                                            {/if}
                                        {/each}
                                    </div>
                                </div>
                                {#if open}
                                    <ul class="endpoint-list">
                                        {#each resource.endpoints as endpoint (endpoint.key)}
                                            <li
                                                class="endpoint-row"
                                                class:checked={selected.has(endpoint.key)}
                                            >
                                                <label class="endpoint-label">
                                                    <input
                                                        type="checkbox"
                                                        name={endpoint.key}
                                                        checked={selected.has(endpoint.key)}
                                                        onchange={(event) =>
                                                            toggle(
                                                                [endpoint.key],
                                                                event.currentTarget.checked
                                                            )}
                                                    >
                                                    <span class="method-badge"
                                                        >{endpoint.method}</span
                                                    >
                                                    <code class="endpoint-path"
                                                        >{endpoint.path}</code
                                                    >
                                                    <span class="endpoint-desc"
                                                        >{endpoint.description}</span
                                                    >
                                                </label>
                                            </li>
                                        {/each}
                                    </ul>
                                {/if}
                            </div>
                        {/each}
                    </div>
                    <div class="footer-actions">
                        <Button variant="default" onclick={requestClose}>
                            {t('common.action.cancel')}
                        </Button>
                        <Button
                            type="submit"
                            color="teal"
                            variant="filled"
                            disabled={!selected.size || name.trim().length < 2 || busy}
                            aria-busy={busy || undefined}
                        >
                            {t('common.action.create')}
                        </Button>
                    </div>
                </fieldset>
            </form>
        {/if}
    </div>
</Overlay>

<style>
    .token-dialog-body {
        display: flex;
        flex-direction: column;
        gap: var(--mantine-spacing-md);
    }

    .error-banner {
        margin: 0;
        padding: var(--mantine-spacing-xs) var(--mantine-spacing-sm);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-red-light);
        color: var(--mantine-color-red-light-color);
        font-size: var(--mantine-font-size-sm);
    }

    .meta-grid {
        display: grid;
        grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1.4fr);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        overflow: hidden;
        background: rgba(var(--gw-fg-rgb), 0.02);
    }

    @media (max-width: 48em) {
        .meta-grid {
            grid-template-columns: minmax(0, 1fr);
        }
    }

    .meta-cell {
        display: flex;
        flex-direction: column;
        gap: 5px;
        min-width: 0;
        padding: 10px 14px;
        border-right: 1px solid rgba(var(--gw-fg-rgb), 0.06);
    }

    .meta-cell:last-child {
        border-right: none;
    }

    @media (max-width: 48em) {
        .meta-cell {
            border-right: none;
            border-bottom: 1px solid rgba(var(--gw-fg-rgb), 0.06);
        }
        .meta-cell:last-child {
            border-bottom: none;
        }
    }

    .meta-label {
        font-size: 11px;
        font-family: var(--mantine-font-family-monospace);
        letter-spacing: 0.04em;
        text-transform: uppercase;
        color: var(--mantine-color-dark-2);
    }

    .meta-value {
        font-size: var(--mantine-font-size-sm);
        color: var(--mantine-color-gray-2);
    }

    .meta-value.expired {
        color: var(--mantine-color-red-5);
    }

    .meta-uuid {
        display: flex;
        align-items: center;
        gap: 6px;
        min-width: 0;
    }

    .meta-uuid-text {
        font-size: 12.5px;
        font-family: var(--mantine-font-family-monospace);
        color: var(--mantine-color-gray-3);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .one-time-notice {
        display: flex;
        gap: 12px;
        align-items: flex-start;
        padding: 12px 14px;
        border-radius: var(--mantine-radius-md);
        border: 1px solid rgba(245, 159, 11, 0.22);
        background: linear-gradient(
            135deg,
            rgba(245, 159, 11, 0.1) 0%,
            rgba(245, 159, 11, 0.02) 100%
        );
    }

    .one-time-icon {
        flex: none;
        margin-top: 1px;
        color: var(--mantine-color-yellow-5);
    }

    .one-time-text {
        margin: 0;
        font-size: var(--mantine-font-size-sm);
        line-height: 1.45;
        color: var(--mantine-color-gray-3);
    }

    .one-time-emphasis {
        font-weight: 700;
        color: var(--mantine-color-yellow-4);
    }

    .token-form fieldset {
        border: 0;
        padding: 0;
        margin: 0;
        min-width: 0;
        display: grid;
        gap: var(--mantine-spacing-md);
    }

    .form-row {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: var(--mantine-spacing-sm);
    }

    @media (max-width: 36em) {
        .form-row {
            grid-template-columns: minmax(0, 1fr);
        }
    }

    .field-label {
        display: grid;
        gap: 4px;
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
    }

    .text-input {
        --input-height: var(--input-height-sm);
        --input-padding: calc(var(--input-height) / 3);
        --input-padding-inline-start: var(--input-padding);
        --input-padding-inline-end: var(--input-padding);
        --input-line-height: calc(var(--input-height) - 2px);
        --input-radius: var(--mantine-radius-default);
        --input-fz: var(--mantine-font-size-sm);
        --input-bd: var(--mantine-color-default-border);
        --input-bg: var(--mantine-color-default);
        --input-bd-focus: var(--mantine-primary-color-filled);
        --input-color: var(--mantine-color-text);
    }

    .button-row {
        display: flex;
        flex-wrap: wrap;
        gap: var(--mantine-spacing-xs);
    }

    .resources-list {
        display: flex;
        flex-direction: column;
        gap: 6px;
    }

    .resource-row {
        border-radius: var(--mantine-radius-md);
        overflow: hidden;
        background: rgba(var(--gw-fg-rgb), 0.02);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
    }

    .resource-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--mantine-spacing-xs);
        padding: 7px 9px;
    }

    .resource-trigger {
        flex: 1;
        min-width: 0;
        display: flex;
        align-items: center;
        gap: 8px;
        cursor: pointer;
        text-align: start;
    }

    .chevron {
        display: inline-flex;
        flex: none;
        color: var(--mantine-color-dark-2);
        transition: transform 150ms ease;
    }

    .chevron.open {
        transform: rotate(90deg);
    }

    .resource-name {
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        font-family: var(--mantine-font-family-monospace);
        color: var(--mantine-color-gray-2);
    }

    .count-pill {
        padding: 1px 6px;
        border-radius: var(--mantine-radius-sm);
        background: var(--mantine-color-gray-light);
        color: var(--mantine-color-gray-light-color);
        font-size: var(--mantine-font-size-xs);
        font-family: var(--mantine-font-family-monospace);
    }

    .kind-pills {
        display: flex;
        align-items: center;
        gap: 6px;
        flex: none;
    }

    .kind-pill {
        display: inline-flex;
        align-items: center;
        gap: 5px;
        height: 24px;
        padding: 0 9px;
        border-radius: var(--mantine-radius-md);
        border: 1px solid var(--mantine-color-dark-4);
        background: rgba(var(--gw-fg-rgb), 0.02);
        color: var(--mantine-color-dark-1);
        font-size: 11px;
        font-weight: 500;
        font-family: var(--mantine-font-family-monospace);
        cursor: pointer;
    }

    .kind-pill input {
        --gw-checkbox-size: 13px;
    }

    .kind-pill.active {
        background: rgba(45, 212, 168, 0.05);
        border-color: var(--mantine-color-teal-9);
        color: var(--mantine-color-teal-4);
    }

    .endpoint-list {
        list-style: none;
        margin: 0;
        padding: 4px;
        border-top: 1px solid var(--mantine-color-dark-5);
        display: grid;
        gap: 2px;
    }

    .endpoint-row {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        padding: 5px 8px;
        border-radius: var(--mantine-radius-sm);
        font-size: var(--mantine-font-size-xs);
    }

    .endpoint-row:not(.readonly):hover {
        background-color: rgba(var(--gw-fg-rgb), 0.025);
    }

    .endpoint-row.checked {
        background-color: var(--mantine-color-cyan-light);
    }

    .endpoint-label {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        width: 100%;
        cursor: pointer;
    }

    .endpoint-label input {
        --gw-checkbox-size: 15px;
    }

    .method-badge {
        flex: none;
        width: 54px;
        text-align: center;
        padding: 1px 4px;
        border-radius: var(--mantine-radius-xs);
        background: var(--mantine-color-gray-light);
        color: var(--mantine-color-gray-light-color);
        font-size: 9.5px;
        font-weight: 700;
        font-family: var(--mantine-font-family-monospace);
        letter-spacing: 0.04em;
    }

    .endpoint-path {
        font-size: 12.5px;
        font-family: var(--mantine-font-family-monospace);
        color: var(--mantine-color-gray-3);
        overflow-wrap: anywhere;
    }

    .endpoint-desc {
        color: var(--mantine-color-dimmed);
        margin-inline-start: auto;
        overflow-wrap: anywhere;
    }

    .footer-actions {
        display: flex;
        align-items: center;
        justify-content: flex-end;
        gap: var(--mantine-spacing-xs);
        margin-top: var(--mantine-spacing-xs);
    }
</style>
