<script lang="ts">
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { ZodError } from 'zod'
    import { externalSquadsQuery } from '../external-squads/queries'
    import { locale } from '../i18n.svelte'
    import { squadsQuery } from '../internal-squads/queries'
    import { queryClient } from '../query-client'
    import { systemStatsQuery } from '../system-stats'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import FormCard from '../ui/FormCard.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import Overlay from '../ui/Overlay.svelte'
    import { type BulkFields, runBulkAction } from './actions'
    import { isoDateTime, numberField, textField } from './form-values'
    import {
        trafficStrategies,
        type UserStatus,
        userStatuses,
        usersQueryKey,
        userTagsQuery
    } from './queries'
    import { userSelection } from './selection.svelte'

    let {
        target: scope,
        ids,
        editOnly = false,
        onclose
    }: {
        target: 'selected' | 'all'
        ids: number[]
        editOnly?: boolean
        onclose: () => void
    } = $props()

    type ActionDialog = 'extend' | 'delete-status'
    let subdialog = $state<ActionDialog>()
    let confirmAction = $state<'reset' | 'revoke' | 'delete'>()
    let chosenSquads = $state<string[]>([])
    let squadSearch = $state('')
    let disableDeviceLimit = $state(false)
    let clears = $state<Record<string, boolean>>({})
    let expiration = $state('')
    const trafficUnits = new Map([
        ['B', 1],
        ['KiB', 1024],
        ['MiB', 1_048_576],
        ['GiB', 1_073_741_824],
        ['TiB', 1_099_511_627_776]
    ])
    let t = $derived(locale.t)
    const hwidHelp = $derived(
        t('create-user-modal.widget.hwid-user-limit-description').split(
            /(<\/?(?:highlight|anchor)>)/
        )
    )
    let error = $state('')
    let scopeDescription = $derived(
        scope === 'all'
            ? t('bulk-all-users-actions.modal.perform-actions-on-all-users')
            : t('bulk-user-actions.actions.tab.feature.perform-action-on-users', {
                  usersCount: ids.length
              })
    )
    const squads = createQuery(() => ({
        ...squadsQuery,
        enabled: !editOnly && scope === 'selected'
    }))
    const external = createQuery(() => ({ ...externalSquadsQuery, enabled: editOnly }))
    const tags = createQuery(() => ({ ...userTagsQuery, enabled: editOnly }))
    let filteredSquads = $derived(
        (squads.data?.internalSquads ?? []).filter((squad) =>
            squad.name
                .toLocaleLowerCase(locale.language)
                .includes(squadSearch.trim().toLocaleLowerCase(locale.language))
        )
    )
    const strategyLabels = {
        NO_RESET: 'reset-data.constants.never-reset',
        DAY: 'reset-data.constants.reset-daily',
        WEEK: 'reset-data.constants.reset-weekly',
        MONTH: 'reset-data.constants.reset-monthly',
        MONTH_ROLLING: 'reset-data.constants.reset-monthly-by-creation-date'
    } as const
    const fieldId = $props.id()

    const mutation = createMutation(() => ({
        mutationFn: runBulkAction,
        onSuccess: async () => {
            userSelection.clear()
            await Promise.all([
                queryClient.invalidateQueries({ queryKey: usersQueryKey }),
                queryClient.invalidateQueries({ queryKey: systemStatsQuery.queryKey })
            ])
            onclose()
        },
        onError: showError
    }))

    function showError(reason: unknown): void {
        error =
            reason instanceof ZodError
                ? reason.issues
                      .map((issue) => `${issue.path.join('.')}: ${issue.message}`)
                      .join('\n')
                : reason instanceof Error
                  ? reason.message
                  : t('common.message.error')
    }

    function getFields(data: FormData): BulkFields {
        const expireAt = textField(data, 'expireAt')
        const status = textField(data, 'status') as UserStatus | ''
        const strategy = textField(data, 'trafficLimitStrategy') as
            | (typeof trafficStrategies)[number]
            | ''
        const externalId = textField(data, 'externalSquadUuid')
        const traffic = numberField(data, 'trafficLimitBytes')
        const fields = {
            status: status || undefined,
            expireAt: expireAt ? isoDateTime(expireAt) : undefined,
            trafficLimitBytes:
                traffic === undefined
                    ? undefined
                    : Math.round(traffic * (trafficUnits.get(textField(data, 'trafficUnit')) ?? 1)),
            trafficLimitStrategy: strategy || undefined,
            description:
                textField(data, 'description') || (data.has('clearDescription') ? null : undefined),
            email: textField(data, 'email').trim() || (data.has('clearEmail') ? null : undefined),
            telegramId:
                numberField(data, 'telegramId') ?? (data.has('clearTelegramId') ? null : undefined),
            hwidDeviceLimit: disableDeviceLimit
                ? 0
                : (numberField(data, 'hwidDeviceLimit') ??
                  (data.has('clearDeviceLimit') ? null : undefined)),
            tag: textField(data, 'tag').trim() || (data.has('clearTag') ? null : undefined),
            externalSquadUuid: externalId === '__clear__' ? null : externalId || undefined
        }
        if (Object.values(fields).every((value) => value === undefined)) {
            throw new Error('Enter at least one change')
        }
        return fields
    }

    function submitUpdate(event: SubmitEvent): void {
        event.preventDefault()
        if (!(event.currentTarget instanceof HTMLFormElement) || mutation.isPending) return
        error = ''
        try {
            const fields = getFields(new FormData(event.currentTarget))
            if (!window.confirm(`${scopeDescription}?`)) return
            mutation.mutate(
                scope === 'all'
                    ? { scope, action: 'update', fields }
                    : { scope, action: 'update', userIds: ids, fields }
            )
        } catch (reason) {
            showError(reason)
        }
    }

    function submitAction(event: SubmitEvent): void {
        event.preventDefault()
        if (!(event.currentTarget instanceof HTMLFormElement) || mutation.isPending) return
        const data = new FormData(event.currentTarget)
        error = ''
        try {
            if (subdialog === 'extend') {
                const extendDays = numberField(data, 'extendDays')
                if (extendDays === undefined) return
                mutation.mutate(
                    scope === 'all'
                        ? { scope, action: 'extend', extendDays }
                        : { scope, action: 'extend', extendDays, userIds: ids }
                )
            } else if (subdialog === 'delete-status') {
                const status = textField(data, 'status') as UserStatus
                if (window.confirm(`${t('common.action.delete')} ${status}?`))
                    mutation.mutate({ scope: 'all', action: 'delete-status', status })
            }
        } catch (reason) {
            showError(reason)
        }
    }

    function openAction(action: ActionDialog): void {
        confirmAction = undefined
        error = ''
        subdialog = action
    }

    function runConfirmed(action: 'reset' | 'revoke' | 'delete'): void {
        if (confirmAction !== action) {
            confirmAction = action
            return
        }
        error = ''
        confirmAction = undefined
        if (scope === 'selected') mutation.mutate({ scope, action, userIds: ids })
        else if (action === 'reset') mutation.mutate({ scope, action })
    }
</script>

{#snippet actionCard(
    title: string,
    description: string,
    icon: IconName,
    tone: string,
    activate: () => void,
    armed?: boolean
)}
    <button
        type="button"
        class="bulk-action mantine-UnstyledButton-root"
        class:armed
        aria-label={title}
        aria-pressed={armed}
        onclick={activate}
        onblur={() => (confirmAction = undefined)}
        disabled={mutation.isPending}
    >
        <span
            class="card-icon"
            style:--action-tint={`var(--mantine-color-${tone}-6)`}
            style:--action-ink={`var(--mantine-color-${tone}-4)`}
        >
            <Icon name={icon} size={20} />
        </span>
        <span class="action-copy">
            <strong>{title}</strong>
            <small>{description}</small>
        </span>
        <span class="action-arrow"
            ><Icon name={armed ? 'TbArrowRight' : 'TbChevronRight'} size={20} /></span
        >
    </button>
{/snippet}

{#snippet feedback()}
    {#if error}
        <p class="error" role="alert">{error}</p>
    {/if}
{/snippet}

{#snippet clearField(key: string, label: string)}
    <ActionIcon
        icon="TbX"
        label={`${t('common.action.clear')} ${label}`}
        size={24}
        iconSize={15}
        subtle
        class="clear-field"
        aria-pressed={Boolean(clears[key])}
        onclick={(event) => {
            clears[key] = !clears[key]
            const field = event.currentTarget.parentElement?.querySelector('input, textarea')
            if (
                clears[key] &&
                (field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement)
            )
                field.value = ''
        }}
    />
    {#if clears[key]}
        <input type="hidden" name={key} value="1">
    {/if}
{/snippet}

{#snippet optionalField(
    name: string,
    label: string,
    icon: IconName,
    type: 'text' | 'number' | 'email',
    clear: string,
    placeholder?: string
)}
    <div class="optional-field">
        {#if name !== 'hwidDeviceLimit'}
            <label for={`${fieldId}-${name}`}>{label}</label>
        {/if}
        {#if name === 'tag'}
            <small>Create or select a tag</small>
        {/if}
        <span class="input-icon with-clear">
            <Icon name={icon} size={16} />
            <input
                id={`${fieldId}-${name}`}
                {name}
                {type}
                {placeholder}
                aria-label={name === 'hwidDeviceLimit' ? label : undefined}
                list={name === 'tag' ? `${fieldId}-tags` : undefined}
                step={type === 'number' ? '1' : undefined}
                disabled={name === 'hwidDeviceLimit' && disableDeviceLimit}
                oninput={() => (clears[clear] = false)}
            >
            {#if name !== 'hwidDeviceLimit' || !disableDeviceLimit}
                {@render clearField(clear, label)}
            {/if}
        </span>
    </div>
{/snippet}

<Overlay
    title={t(
        editOnly
            ? 'bulk-all-user-actions-tabs.update.tab.feature.update-users'
            : 'common.action.bulk-actions'
    )}
    subtitle={scopeDescription}
    icon={editOnly ? 'TbUsers' : scope === 'all' ? 'TbBolt' : 'TbDots'}
    width={editOnly ? 1000 : 620}
    class={editOnly ? 'bulk-users bulk-edit' : 'bulk-users'}
    close={onclose}
    preventClose={mutation.isPending}
>
    {#if editOnly}
        <form onsubmit={submitUpdate} tabindex="-1" data-autofocus>
            <fieldset class="edit-fields" disabled={mutation.isPending}>
                <div class="edit-column personal-fields">
                    <FormCard
                        title={t('contact-information-card.contact-information')}
                        icon="TbMail"
                        tone="teal"
                    >
                        {@render optionalField(
                            'telegramId',
                            'Telegram ID',
                            'PiTelegramLogoDuotone',
                            'number',
                            'clearTelegramId',
                            "Enter user's Telegram ID (optional)"
                        )}
                        {@render optionalField(
                            'email',
                            'Email',
                            'PiEnvelopeDuotone',
                            'email',
                            'clearEmail',
                            "Enter user's email (optional)"
                        )}
                    </FormCard>
                    <FormCard
                        title={t('device-tag-settings-card.device-and-tag-settings')}
                        icon="TbSettings"
                        tone="orange"
                    >
                        <div class="device-limit">
                            <span class="field-label"
                                >{t('create-user-modal.widget.hwid-device-limit')}</span
                            >
                            <small>
                                {#each hwidHelp as part, index (index)}
                                    {#if part && !part.startsWith('<')}
                                        {#if hwidHelp[index - 1] === '<anchor>'}
                                            <a
                                                href="https://docs.rw/docs/features/hwid-device-limit"
                                                target="_blank"
                                                rel="noreferrer"
                                                >{part}</a
                                            >
                                        {:else if hwidHelp[index - 1] === '<highlight>'}
                                            <code>{part}</code>
                                        {:else}
                                            {part}
                                        {/if}
                                    {/if}
                                {/each}
                            </small>
                            <label class="checkbox">
                                <input type="checkbox" bind:checked={disableDeviceLimit}>
                                {t('create-user-modal.widget.disable-hwid-limit')}
                            </label>
                            {@render optionalField(
                                'hwidDeviceLimit',
                                t('create-user-modal.widget.hwid-device-limit'),
                                'TbDevices2',
                                'number',
                                'clearDeviceLimit',
                                'Fallback Device Limit in use'
                            )}
                        </div>
                        {@render optionalField(
                            'tag',
                            t('common.field.tag'),
                            'TbTag',
                            'text',
                            'clearTag',
                            'EXAMPLE_TAG_1'
                        )}
                        <datalist id={`${fieldId}-tags`}>
                            {#each tags.data?.tags ?? [] as tag (tag)}
                                <option value={tag}></option>
                            {/each}
                        </datalist>
                        <div class="optional-field">
                            <label for={`${fieldId}-description`}
                                >{t('common.field.description')}</label
                            >
                            <small>{t('create-user-modal.widget.user-description')}</small>
                            <span class="input-icon with-clear">
                                <textarea
                                    id={`${fieldId}-description`}
                                    name="description"
                                    rows="2"
                                    oninput={() => (clears['clearDescription'] = false)}
                                ></textarea>
                                {@render clearField(
                                    'clearDescription',
                                    t('common.field.description')
                                )}
                            </span>
                        </div>
                    </FormCard>
                </div>
                <div class="edit-column">
                    <FormCard
                        title={t('traffic-limits-card.traffic-and-limits')}
                        icon="TbChartLine"
                        tone="violet"
                    >
                        <div class="traffic-limit">
                            <label>
                                {t('traffic-limits-card.traffic-limit')}
                                <small>{t('traffic-limits-card.traffic-limit-description')}</small>
                                <span class="input-icon"
                                    ><Icon name="TbChartLine" size={16} />
                                    <input
                                        name="trafficLimitBytes"
                                        type="number"
                                        min="0"
                                        step="any"
                                    ></span
                                >
                            </label>
                            <select name="trafficUnit" aria-label="Traffic unit" value="GiB">
                                {#each trafficUnits.keys() as unit (unit)}
                                    <option value={unit}>{unit}</option>
                                {/each}
                            </select>
                        </div>
                        <label>
                            {t('create-user-modal.widget.traffic-reset-strategy')}
                            <small
                                >{t(
                                    'create-user-modal.widget.traffic-reset-strategy-description'
                                )}</small
                            >
                            <span class="input-icon"
                                ><Icon name="PiClockDuotone" size={16} />
                                <select name="trafficLimitStrategy">
                                    <option value="">
                                        {t('create-user-modal.widget.pick-value')}
                                    </option>
                                    {#each trafficStrategies as strategy (strategy)}
                                        <option value={strategy}>
                                            {t(strategyLabels[strategy])}
                                        </option>
                                    {/each}
                                </select></span
                            >
                        </label>
                    </FormCard>
                    <FormCard
                        title={t('access-settings-card.access-settings')}
                        icon="TbShield"
                        tone="indigo"
                    >
                        <label>
                            {t('common.field.status')}
                            <small>{t('common.field.user-status')}</small>
                            <span class="input-icon"
                                ><Icon name="TbUser" size={16} />
                                <select name="status">
                                    <option value="">
                                        {t(
                                            'bulk-all-user-actions-tabs.update.tab.feature.select-status'
                                        )}
                                    </option>
                                    {#each userStatuses as status (status)}
                                        <option value={status}>{status}</option>
                                    {/each}
                                </select></span
                            >
                        </label>
                        <label>
                            {t('create-user-modal.widget.expiry-date')}
                            <small>{t('create-user-modal.widget.expire-at-description')}</small>
                            <span class="input-icon"
                                ><Icon name="PiCalendarDuotone" size={16} />
                                <input
                                    class:empty-date={!expiration}
                                    name="expireAt"
                                    type="datetime-local"
                                    bind:value={expiration}
                                ></span
                            >
                        </label>
                        <label>
                            {t('access-settings-card.external-squad')}
                            <small
                                >{t(
                                    'access-settings-card.select-an-external-squad-to-apply-custom-settings-to-this-user'
                                )}</small
                            >
                            <span class="input-icon"
                                ><Icon name="TbWebhook" size={16} />
                                <select name="externalSquadUuid">
                                    <option value="">
                                        {t('access-settings-card.select-an-external-squad')}
                                    </option>
                                    <option value="__clear__">Clear external squad</option>
                                    {#each external.data?.externalSquads ??
                                        [] as squad (squad.uuid)}
                                        <option value={squad.uuid}>{squad.name}</option>
                                    {/each}
                                </select></span
                            >
                        </label>
                    </FormCard>
                </div>
            </fieldset>
            {@render feedback()}
            <footer class="edit-footer">
                <Button
                    type="submit"
                    size="md"
                    variant="light"
                    icon="PiFloppyDiskDuotone"
                    pending={mutation.isPending}
                    >{t('common.action.update')}</Button
                >
            </footer>
        </form>
    {:else}
        <div class="bulk-action-list" tabindex="-1" data-autofocus>
            {@render actionCard(
                t('bulk-user-actions.actions.tab.feature.extend-expiration-date'),
                t('bulk-user-actions.actions.tab.feature.extend-expiration-date-description'),
                'TbCalendarUp',
                'cyan',
                () => openAction('extend')
            )}
            {@render actionCard(
                t('common.action.reset-traffic'),
                t('bulk-all-user-actions-tabs.actions.tab.feature.reset-traffic-description'),
                'TbRefresh',
                'blue',
                () => runConfirmed('reset'),
                confirmAction === 'reset'
            )}
            {#if scope === 'selected'}
                {@render actionCard(
                    t('bulk-user-actions.actions.tab.feature.revoke-subscription'),
                    t(
                        'bulk-user-actions.actions.tab.feature.revokes-subscription-for-all-selected-users'
                    ),
                    'TbRefreshAlert',
                    'orange',
                    () => runConfirmed('revoke'),
                    confirmAction === 'revoke'
                )}
                {@render actionCard(
                    t('bulk-user-actions.danger.tab.feature.delete-users'),
                    t(
                        'bulk-user-actions.danger.tab.feature.permanently-deletes-all-selected-users-and-their-data'
                    ),
                    'TbUsersMinus',
                    'red',
                    () => runConfirmed('delete'),
                    confirmAction === 'delete'
                )}
                <FormCard
                    class="bulk-squads"
                    title={t('common.field.internal-squads')}
                    description={t(
                        'bulk-user-actions.actions.tab.feature.specify-internal-squads-that-will-be-assigned-to-the-user'
                    )}
                    icon="TbCirclesRelation"
                    tone="cyan"
                >
                    <span class="input-icon">
                        <Icon name="TbCirclesRelation" size={16} />
                        <input
                            type="search"
                            aria-label={t('internal-squads-list.widget.search-internal-squads')}
                            placeholder={t('internal-squads-list.widget.search-internal-squads')}
                            bind:value={squadSearch}
                        >
                    </span>
                    <div class="squad-choices">
                        {#each filteredSquads as squad (squad.uuid)}
                            <label class="checkbox"
                                ><input
                                    type="checkbox"
                                    checked={chosenSquads.includes(squad.uuid)}
                                    disabled={mutation.isPending}
                                    onchange={(event) => {
                                        chosenSquads = event.currentTarget.checked
                                            ? [...chosenSquads, squad.uuid]
                                            : chosenSquads.filter((id) => id !== squad.uuid)
                                    }}
                                >
                                <strong>{squad.name}</strong>
                                <span class="squad-count teal" title={t('constants.users')}
                                    ><Icon name="TbUsers" size={14} />
                                    {squad.info.membersCount}</span
                                >
                                <span class="squad-count blue" title={t('common.field.inbounds')}
                                    ><Icon name="TbTag" size={14} />{squad.info.inboundsCount}</span
                                >
                            </label>
                        {:else}
                            <small
                                >{t(
                                    squads.isPending
                                        ? 'common.message.loading'
                                        : 'common.message.nothing-found'
                                )}</small
                            >
                        {/each}
                    </div>
                    {#if squads.error}
                        <p class="error" role="alert">{squads.error.message}</p>
                    {/if}
                    <div class="squad-action">
                        <Button
                            variant="soft"
                            disabled={mutation.isPending || !squads.isSuccess}
                            onclick={() => {
                                error = ''
                                if (window.confirm(`${scopeDescription}?`))
                                    mutation.mutate({
                                        scope: 'selected',
                                        action: 'squads',
                                        userIds: ids,
                                        activeInternalSquads: chosenSquads.slice()
                                    })
                            }}
                            >{t('common.action.change')}</Button
                        >
                    </div>
                </FormCard>
            {:else}
                {@render actionCard(
                    t('bulk-all-user-actions-tabs.danger.tab.feature.delete-users-by-status'),
                    t(
                        'bulk-all-user-actions-tabs.danger.tab.feature.delete-users-by-status-description'
                    ),
                    'TbUsersMinus',
                    'red',
                    () => openAction('delete-status')
                )}
            {/if}
            {#if !subdialog}
                {@render feedback()}
            {/if}
        </div>
    {/if}
</Overlay>

{#if subdialog}
    <Overlay
        title={t(
            subdialog === 'extend'
                ? 'bulk-user-actions.actions.tab.feature.extend-expiration-date'
                : 'bulk-all-user-actions-tabs.danger.tab.feature.delete-users-by-status'
        )}
        icon={subdialog === 'extend' ? 'TbCalendarUp' : 'TbTrash'}
        tone={subdialog === 'extend' ? 'teal' : 'red'}
        width={440}
        fullscreenOnMobile={false}
        close={() => {
            subdialog = undefined
            error = ''
        }}
        preventClose={mutation.isPending}
    >
        <form onsubmit={submitAction}>
            {#if subdialog === 'extend'}
                <label>
                    {t('bulk-user-actions.actions.tab.feature.extend-days')}
                    <small
                        >{t(
                            'bulk-user-actions.actions.tab.feature.enter-the-number-of-days-to-extend-the-expiration-date'
                        )}</small
                    >
                    <input
                        name="extendDays"
                        type="number"
                        min="1"
                        max="9999"
                        step="1"
                        value="1"
                        required
                        data-autofocus
                    >
                </label>
            {:else}
                <label>
                    {t('common.field.status')}
                    <select name="status" required data-autofocus>
                        <option value="">Choose a status</option>
                        {#each userStatuses as status (status)}
                            <option value={status}>{status}</option>
                        {/each}
                    </select>
                </label>
            {/if}
            {@render feedback()}
            <Button
                block
                type="submit"
                color={subdialog === 'extend' ? 'cyan' : 'red'}
                variant="soft"
                pending={mutation.isPending}
            >
                {t(
                    subdialog === 'extend'
                        ? 'bulk-user-actions.actions.tab.feature.extend'
                        : 'common.action.delete'
                )}
            </Button>
        </form>
    </Overlay>
{/if}

<style>
    :global(.bulk-users .overlay-title h2) {
        font-size: var(--mantine-h5-font-size);
        line-height: var(--mantine-h5-line-height);
    }

    form,
    .edit-column {
        display: grid;
        gap: var(--mantine-spacing-md);
    }

    label {
        display: grid;
        gap: 2px;
        font-size: var(--mantine-font-size-sm);
        font-weight: var(--mantine-font-weight-medium);
    }

    small {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        font-weight: 400;
        line-height: var(--mantine-line-height-xs);
    }

    input:not([type="checkbox"]),
    textarea,
    select {
        min-width: 0;
        width: 100%;
        min-height: 36px;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        padding: 6px 12px;
        background: var(--mantine-color-default);
        color: var(--mantine-color-text);
        font-family: inherit;
        font-size: var(--mantine-font-size-sm);
    }

    textarea {
        resize: vertical;
    }

    .edit-fields {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        align-items: start;
        min-width: 0;
        gap: var(--mantine-spacing-md);
        border: 0;
        padding: 0;
        margin: 0;
    }

    .input-icon {
        display: flex;
        align-items: center;
        position: relative;
        color: var(--mantine-color-dimmed);
    }

    .input-icon > :global(svg) {
        position: absolute;
        inset-inline-start: 12px;
        pointer-events: none;
    }

    .input-icon input:not([type="hidden"]),
    .input-icon select {
        padding-inline-start: 36px;
    }

    .input-icon.with-clear input:not([type="hidden"]),
    .input-icon.with-clear textarea {
        padding-inline-end: 36px;
    }

    .input-icon :global(.clear-field) {
        position: absolute;
        inset-inline-end: 4px;
        opacity: 0;
    }

    .input-icon:focus-within :global(.clear-field),
    .input-icon:hover :global(.clear-field),
    .input-icon :global(.clear-field[aria-pressed="true"]) {
        opacity: 1;
    }

    .input-icon :global(.clear-field[aria-pressed="true"]) {
        color: var(--mantine-color-red-light-color);
    }

    .field-label {
        font-size: var(--mantine-font-size-sm);
        font-weight: var(--mantine-font-weight-medium);
    }

    .device-limit {
        display: grid;
        gap: 4px;
    }

    .device-limit code {
        border-radius: var(--mantine-radius-xs);
        padding-inline: 3px;
        background: var(--mantine-color-default);
    }

    .traffic-limit {
        display: grid;
        grid-template-columns: minmax(0, 1fr) 92px;
        align-items: end;
        gap: var(--mantine-spacing-xs);
    }

    .empty-date:not(:focus) {
        color: transparent;
    }

    .empty-date::-webkit-calendar-picker-indicator {
        opacity: 0;
    }

    [data-autofocus]:focus {
        outline: none;
    }

    .checkbox {
        display: flex;
        align-items: center;
        gap: 8px;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }

    .device-limit .checkbox {
        padding-block: 5px;
        font-size: var(--mantine-font-size-sm);
    }

    .checkbox input {
        min-height: 0;
        accent-color: var(--mantine-primary-color-filled);
    }

    .optional-field {
        display: grid;
        gap: 4px;
        min-width: 0;
    }

    .bulk-action-list {
        display: grid;
        gap: var(--mantine-spacing-xs);
    }

    .bulk-action {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-md);
        width: 100%;
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }

    .bulk-action:hover {
        background: var(--mantine-color-dark-6);
    }

    .bulk-action.armed {
        border-color: rgba(250, 82, 82, 0.4);
        background: rgba(250, 82, 82, 0.04);
    }

    .card-icon {
        display: grid;
        place-items: center;
        flex: none;
        width: 44px;
        height: 44px;
        border-radius: var(--mantine-radius-md);
        border: 1px solid color-mix(in srgb, var(--action-tint) 30%, transparent);
        color: var(--action-ink);
        background: color-mix(in srgb, var(--action-tint) 14%, transparent);
    }

    .action-copy {
        display: grid;
        gap: 2px;
        flex: 1;
        text-align: start;
    }

    .action-copy strong {
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
        line-height: var(--mantine-line-height-sm);
    }

    .action-arrow {
        display: inline-flex;
        flex: none;
        color: var(--mantine-color-dimmed);
    }

    .armed .action-arrow {
        color: var(--mantine-color-red-5);
    }

    :global(.bulk-squads header) {
        border-block-end: 0;
        padding-block-end: 0;
        gap: var(--mantine-spacing-md);
    }

    :global(.bulk-squads header > span) {
        width: 44px;
        height: 44px;
    }

    :global(.bulk-squads header h3) {
        font-size: var(--mantine-font-size-sm);
    }

    .squad-choices {
        display: grid;
        gap: 8px;
        min-height: 80px;
        max-height: 12rem;
        overflow: auto;
        padding: 8px;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-card-bg);
    }

    .squad-choices label {
        min-height: 48px;
        padding: 6px 12px;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }

    .squad-choices strong {
        flex: 1;
        color: var(--mantine-color-text);
        font-family: var(--mantine-font-family-monospace);
    }

    .squad-count {
        display: flex;
        align-items: center;
        gap: 4px;
        padding: 0 9px;
        border-radius: var(--mantine-radius-md);
        font-weight: 700;
    }

    .squad-count.teal {
        color: var(--mantine-color-teal-light-color);
        background: var(--mantine-color-teal-light);
    }

    .squad-count.blue {
        color: var(--mantine-color-blue-light-color);
        background: var(--mantine-color-blue-light);
    }

    .squad-action,
    .edit-footer {
        display: flex;
        justify-content: flex-end;
        padding-block-start: var(--mantine-spacing-sm);
        border-block-start: 1px solid rgba(var(--gw-fg-rgb), 0.06);
    }

    .edit-footer {
        position: sticky;
        bottom: calc(-1 * var(--mantine-spacing-sm));
        margin: 0 calc(-1 * var(--mantine-spacing-sm)) calc(-1 * var(--mantine-spacing-sm));
        padding: var(--mantine-spacing-sm);
        background: rgba(var(--gw-surface-rgb), 0.95);
    }

    .error {
        color: var(--mantine-color-red-light-color);
        white-space: pre-wrap;
    }

    @media (max-width: 48em) {
        .edit-fields {
            grid-template-columns: minmax(0, 1fr);
        }

        .personal-fields {
            order: 1;
        }

        .edit-footer :global(button) {
            width: 100%;
        }
    }
</style>
