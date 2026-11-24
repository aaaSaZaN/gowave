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
    import { createQuery } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import { cancelDialog, dismissDialog } from '../ui/dialog-motion'
    import { type ApiToken, apiScopes, apiTokens } from './queries'

    let { token, close }: { token: ApiToken | null; close: () => void } = $props()
    const scopes = createQuery(() => apiScopes)
    let resources = $derived(scopes.data?.resources ?? [])
    let dialog: HTMLDialogElement
    let selected = $state(new Set<string>())
    let requestedScopes = $derived(buildScopes(resources, selected))
    let name = $state('')
    let expires = $state<number | undefined>(30)
    let issued = $state.raw<CreateApiTokenCommand.Response['response']>()
    let current = $derived(issued ?? token)
    let granted = $derived(new Set(expandScopesToKeys(resources, current?.scopes ?? [])))
    let dates = $derived(
        new Intl.DateTimeFormat(locale.language, { dateStyle: 'medium', timeStyle: 'short' })
    )
    let busy = $state(false)
    let error = $state('')
    let status = $state('')
    let t = $derived(locale.t)

    onMount(() => dialog.showModal())

    function toggle(keys: string[], checked: boolean): void {
        const next = new Set(selected)
        for (const key of keys) {
            if (checked) next.add(key)
            else next.delete(key)
        }
        selected = next
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
        status = ''
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
        } catch (reason) {
            error = reason instanceof Error ? reason.message : String(reason)
        }
    }

    function copy(value: string): void {
        status = ''
        error = ''
        void navigator.clipboard.writeText(value).then(
            () => (status = t('common.message.copied')),
            (reason: unknown) => (error = reason instanceof Error ? reason.message : String(reason))
        )
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

<dialog
    bind:this={dialog}
    onclose={close}
    aria-labelledby="token-heading"
    oncancel={(event) => cancelDialog(event, busy)}
>
    <h2 id="token-heading">{current?.name ?? t('api-tokens-card.widget.create-api-token')}</h2>
    {#if scopes.error || error}
        <p role="alert">{error || scopes.error?.message}</p>
    {/if}
    {#if status}
        <p role="status">{status}</p>
    {/if}
    {#if current}
        <p>
            UUID <code>{current.uuid}</code>
            <button type="button" onclick={() => copy(current.uuid)}>
                {t('common.action.copy-uuid')}
            </button>
        </p>
        <p>
            {t('api-tokens-card.widget.col-created')} {dates.format(new Date(current.createdAt))}
            · {t('api-tokens-card.widget.col-expires')} {dates.format(new Date(current.expireAt))}
        </p>
        {#if issued}
            <p>{t('api-tokens-card.widget.one-time-notice').replace(/<\/?emphasis>/g, '')}</p>
            <label
                >{t('api-tokens-card.widget.token')}
                <input name="issuedToken" readonly value={issued.token}></label
            >
            <button
                type="button"
                onclick={() => {
                    if (issued) copy(issued.token)
                }}
            >
                {t('api-tokens-card.widget.copy-token')}
            </button>
        {/if}
        {#each resources.filter((resource) =>
            countSelected(resource.endpoints, granted)
        ) as resource (resource.resource)}
            <details>
                <summary>
                    {humanizeResource(resource.resource)}
                    · {countSelected(resource.endpoints, granted)}
                </summary>
                <ul>
                    {#each resource.endpoints.filter(({ key }) =>
                        granted.has(key)
                    ) as endpoint (endpoint.key)}
                        <li>
                            <code>{endpoint.method} {endpoint.path}</code> {endpoint.description}
                        </li>
                    {/each}
                </ul>
            </details>
        {/each}
        <div class="button-row">
            <button type="button" onclick={() => copy(JSON.stringify(current.scopes, null, 2))}>
                {t('api-tokens-card.widget.copy-scopes')}
            </button>
            <button type="button" onclick={() => dismissDialog(dialog)}>
                {t('common.action.close')}
            </button>
        </div>
    {:else}
        <form onsubmit={(event) => void create(event)}>
            <fieldset disabled={busy}>
                <label
                    >{t('api-tokens-card.widget.token-name')}
                    <input name="tokenName" required minlength="2" maxlength="30" bind:value={name}>
                </label>
                <label
                    >{t('api-tokens-card.widget.expires-in-days')}
                    <input
                        name="expiresInDays"
                        type="number"
                        required
                        min="1"
                        step="1"
                        bind:value={expires}
                    >
                </label>
                {#if scopes.isPending}
                    <p role="status">{t('common.message.loading')}</p>
                {/if}
                <div class="button-row">
                    <button type="button" onclick={() => preset('read')}>
                        {t('api-tokens-card.widget.read-only')}
                    </button>
                    <button type="button" onclick={() => preset('all')}>
                        {t('api-tokens-card.widget.full-access')}
                    </button>
                    <button type="button" onclick={() => preset('page')}>
                        {t('constants.subscription-page')}
                    </button>
                    <button type="button" onclick={() => preset('clear')}>
                        {t('api-tokens-card.widget.clear-scopes')}
                    </button>
                    <button type="button" onclick={() => void pasteScopes()}>
                        {t('api-tokens-card.widget.paste-scopes')}
                    </button>
                </div>
                {#each resources as resource (resource.resource)}
                    <details>
                        <summary>
                            {humanizeResource(resource.resource)}
                            · {countSelected(resource.endpoints, selected)}
                        </summary>
                        <div class="button-row">
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
                                    <label class="endpoint">
                                        <input
                                            type="checkbox"
                                            checked={group.keys.every((key) => selected.has(key))}
                                            indeterminate={group.keys.some((key) =>
                                                selected.has(key)
                                            ) && !group.keys.every((key) => selected.has(key))}
                                            onchange={(event) =>
                                                toggle(group.keys, event.currentTarget.checked)}
                                        >
                                        {group.label}
                                    </label>
                                {/if}
                            {/each}
                        </div>
                        <ul>
                            {#each resource.endpoints as endpoint (endpoint.key)}
                                <li>
                                    <label class="endpoint">
                                        <input
                                            type="checkbox"
                                            name={endpoint.key}
                                            checked={selected.has(endpoint.key)}
                                            onchange={(event) =>
                                                toggle([endpoint.key], event.currentTarget.checked)}
                                        >
                                        <span
                                            ><code>{endpoint.method} {endpoint.path}</code>
                                            {endpoint.description}</span
                                        >
                                    </label>
                                </li>
                            {/each}
                        </ul>
                    </details>
                {/each}
                <div class="button-row">
                    <button type="submit" disabled={!selected.size || name.trim().length < 2}>
                        {t('common.action.create')}
                    </button>
                    <button type="button" onclick={() => dismissDialog(dialog)}>
                        {t('common.action.cancel')}
                    </button>
                </div>
            </fieldset>
        </form>
    {/if}
</dialog>

<style>
    dialog {
        width: min(48rem, calc(100% - 1rem));
    }
    fieldset {
        border: 0;
        padding: 0;
        margin: 0;
        min-width: 0;
        display: grid;
        gap: 0.75rem;
    }
    label {
        display: grid;
        gap: 0.3rem;
    }
    ul {
        list-style: none;
        padding-inline-start: 0;
    }
    .endpoint {
        display: flex;
        align-items: baseline;
    }
    .endpoint input {
        flex: none;
    }
    details {
        padding: 0.5rem;
        border: 1px solid var(--gw-border);
        border-radius: 0.35rem;
        margin-block: 0.25rem;
    }
    summary {
        cursor: pointer;
    }
    details .button-row,
    dialog > .button-row {
        margin-block-start: 0.75rem;
    }
    code,
    p {
        overflow-wrap: anywhere;
    }
</style>
