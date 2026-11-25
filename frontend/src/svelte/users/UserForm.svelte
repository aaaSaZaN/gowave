<script lang="ts">
    import { CreateUserCommand, UpdateUserCommand } from '@remnawave/backend-contract'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { type Snippet, untrack } from 'svelte'
    import { ZodError } from 'zod'
    import { submitCommand } from '../commands'
    import { externalSquadsQuery } from '../external-squads/queries'
    import { locale } from '../i18n.svelte'
    import { squadsQuery } from '../internal-squads/queries'
    import SquadInbounds from '../internal-squads/SquadInbounds.svelte'
    import { queryClient } from '../query-client'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import FormCard from '../ui/FormCard.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import Overlay from '../ui/Overlay.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import ExpirationPicker from './ExpirationPicker.svelte'
    import { isoDateTime, localDateTime, numberField, textField } from './form-values'
    import {
        trafficStrategies,
        type User,
        userQuery,
        usersQueryKey,
        userTagsQuery
    } from './queries'

    let {
        user,
        onclose,
        actions,
        identity,
        headerActions,
        overlays
    }: {
        user: User | null
        onclose: () => void
        actions?: Snippet
        identity?: Snippet
        headerActions?: Snippet
        overlays?: Snippet
    } = $props()
    const original = untrack(() => user)
    const defaultExpiration = new Date()
    defaultExpiration.setDate(defaultExpiration.getDate() + 1)
    const initialDate = localDateTime(original?.expireAt ?? defaultExpiration)
    const units = new Map([
        ['B', 1],
        ['MiB', 1048576],
        ['GiB', 1073741824],
        ['TiB', 1099511627776]
    ])
    const initialBytes = original?.trafficLimitBytes ?? 0
    const initialUnit = initialBytes % 1073741824 === 0 ? 'GiB' : 'B'
    const touched = new Set<'trafficLimit' | 'trafficLimitStrategy' | 'description' | 'tag'>()
    let trafficUnit = $state(initialUnit)
    let trafficStrategy = $state(original?.trafficLimitStrategy ?? 'NO_RESET')
    let expireAt = $state(initialDate)
    let expirationOpen = $state(false)
    let tag = $state(original?.tag ?? '')
    let squadSearch = $state('')
    let editingSquad = $state<string>()
    let noDeviceLimit = $state(original?.hwidDeviceLimit === 0)
    const strategyLabels = {
        NO_RESET: 'reset-data.constants.never-reset',
        DAY: 'reset-data.constants.reset-daily',
        WEEK: 'reset-data.constants.reset-weekly',
        MONTH: 'reset-data.constants.reset-monthly',
        MONTH_ROLLING: 'reset-data.constants.reset-monthly-by-creation-date'
    } as const
    let dateText = $derived.by(() => {
        const date = new Date(expireAt)
        if (!Number.isFinite(date.getTime())) return ''
        const format = new Intl.DateTimeFormat(locale.language, { dateStyle: 'long' })
        return `${format.format(date)} - ${expireAt.slice(11, 16)}`
    })
    let error = $state('')
    let chosenSquads = $state(original?.activeInternalSquads.map(({ uuid }) => uuid) ?? [])
    let t = $derived(locale.t)
    const hwidDescription = $derived(
        t('create-user-modal.widget.hwid-user-limit-description').split(
            /(<\/?(?:highlight|anchor)>)/
        )
    )
    const fieldPrefix = $props.id()

    const squads = createQuery(() => squadsQuery)
    const external = createQuery(() => externalSquadsQuery)
    const tags = createQuery(() => userTagsQuery)
    let options = $derived({
        squads: squads.data?.internalSquads,
        external: external.data?.externalSquads
    })
    let choices = $derived(options.squads ?? original?.activeInternalSquads ?? [])
    let visibleSquads = $derived(
        choices.filter(({ name }) => name.toLowerCase().includes(squadSearch.trim().toLowerCase()))
    )
    const squadInfo = $derived(new Map(options.squads?.map(({ uuid, info }) => [uuid, info])))
    const externalChoices = $derived.by(() => {
        const choices = (options.external ?? []).map(({ uuid, name }) => ({
            value: uuid,
            label: name
        }))
        const id = original?.externalSquadUuid
        if (id && !choices.some(({ value }) => value === id))
            choices.unshift({ value: id, label: id })
        return choices
    })
    const save = createMutation(() => ({
        mutationFn: (body: unknown) =>
            original
                ? submitCommand(UpdateUserCommand, body)
                : submitCommand(CreateUserCommand, body),
        onSuccess: async (updated) => {
            queryClient.setQueryData(userQuery(String(updated.id)).queryKey, updated)
            await queryClient.invalidateQueries({ queryKey: usersQueryKey })
            onclose()
        },
        onError: (reason) => {
            error =
                reason instanceof ZodError
                    ? reason.issues
                          .map((issue) => `${issue.path.join('.')}: ${issue.message}`)
                          .join('\n')
                    : reason.message
        }
    }))

    function submit(event: SubmitEvent): void {
        event.preventDefault()
        if (!(event.currentTarget instanceof HTMLFormElement) || save.isPending) return
        error = ''
        const data = new FormData(event.currentTarget)
        const status = textField(data, 'status')
        const expire = textField(data, 'expireAt')
        const externalId = textField(data, 'externalSquadUuid')
        const deviceLimit = numberField(data, 'hwidDeviceLimit')
        try {
            const multiplier = units.get(textField(data, 'trafficUnit')) ?? 1
            const traffic = Number(textField(data, 'trafficLimit')) * multiplier
            if (!Number.isFinite(traffic) || traffic < 0) throw new Error(t('common.message.error'))
            save.mutate({
                id: original?.id,
                username: textField(data, 'username').trim(),
                status: status === original?.status ? undefined : status,
                expireAt: original && expire === initialDate ? undefined : isoDateTime(expire),
                trafficLimitBytes:
                    !original || touched.has('trafficLimit') ? Math.round(traffic) : undefined,
                trafficLimitStrategy:
                    !original || touched.has('trafficLimitStrategy') ? trafficStrategy : undefined,
                description:
                    !original || touched.has('description')
                        ? textField(data, 'description') || (original ? null : '')
                        : undefined,
                email: textField(data, 'email').trim() || null,
                telegramId: numberField(data, 'telegramId') ?? null,
                tag:
                    !original || touched.has('tag')
                        ? textField(data, 'tag').trim() || null
                        : undefined,
                hwidDeviceLimit: noDeviceLimit ? 0 : (deviceLimit ?? (original ? null : undefined)),
                activeInternalSquads: chosenSquads.slice(),
                externalSquadUuid: options.external
                    ? externalId || null
                    : (original?.externalSquadUuid ?? null)
            })
        } catch (reason) {
            error = reason instanceof Error ? reason.message : t('common.message.error')
        }
    }
</script>

{#snippet inputField(
    label: string,
    description: string,
    icon: IconName,
    content: Snippet<[string]>,
    required = false,
    fieldClass = ''
)}
    {@const controlId = `${fieldPrefix}-${encodeURIComponent(label)}`}
    <label class="form-field {fieldClass}" for={controlId}>
        <span
            >{label}
            {#if required}
                <span class="required"> *</span>
            {/if}</span
        >
        {#if description}
            <small>{description}</small>
        {/if}
        <span class="input-wrap">
            <span class="leading"><Icon name={icon} size={16} /></span>
            {@render content(controlId)}
        </span>
    </label>
{/snippet}

<Overlay
    title={original?.username ?? t('create-user-modal.widget.create-user')}
    icon="TbUser"
    tone={original ? 'cyan' : 'teal'}
    width={1000}
    close={onclose}
    preventClose={save.isPending}
    actions={headerActions}
    inlineActions
>
    <form onsubmit={submit} class:editing={!!original}>
        <div class="form-columns">
            <div class="form-column">
                {#if original && identity}
                    {@render identity()}
                    <input type="hidden" name="username" value={original.username}>
                    <input type="hidden" name="status" value={original.status}>
                {:else}
                    <FormCard
                        class="identity"
                        title={t('user-identity-creation-card.user-identity')}
                        icon="HiIdentification"
                        tone="blue"
                    >
                        {#snippet children()}
                            {@render inputField(
                                t('common.field.username'),
                                t('create-user-modal.widget.username-cannot-be-changed-later'),
                                'PiUserDuotone',
                                username,
                                true
                            )}
                            {#snippet username(controlId: string)}
                                <input
                                    id={controlId}
                                    name="username"
                                    value={original?.username ?? ''}
                                    required
                                >
                            {/snippet}
                            <input type="hidden" name="status" value="ACTIVE">
                        {/snippet}
                    </FormCard>
                {/if}

                <FormCard
                    class="contact"
                    title={t('contact-information-card.contact-information')}
                    icon="TbMail"
                    tone="teal"
                >
                    {#snippet children()}
                        {@render inputField('Telegram ID', '', 'PiTelegramLogoDuotone', telegram)}
                        {#snippet telegram(controlId: string)}
                            <input
                                id={controlId}
                                type="number"
                                name="telegramId"
                                step="1"
                                value={original?.telegramId ?? ''}
                                placeholder="Enter user's Telegram ID (optional)"
                            >
                        {/snippet}
                        {@render inputField('Email', '', 'PiEnvelopeDuotone', email)}
                        {#snippet email(controlId: string)}
                            <input
                                id={controlId}
                                type="email"
                                name="email"
                                value={original?.email ?? ''}
                                placeholder="Enter user's email (optional)"
                            >
                        {/snippet}
                    {/snippet}
                </FormCard>

                <FormCard
                    class="device"
                    title={t('device-tag-settings-card.device-and-tag-settings')}
                    icon="TbSettings"
                    tone="orange"
                >
                    {#snippet children()}
                        <div class="form-field device-limit">
                            <label class="hwid-title" for="device-limit"
                                >{t('create-user-modal.widget.hwid-device-limit')}</label
                            >
                            <small class="hwid-description">
                                {#each hwidDescription as part, index (index)}
                                    {#if part && !part.startsWith('<')}
                                        {#if hwidDescription[index - 1] === '<anchor>'}
                                            <a
                                                href="https://docs.rw/docs/features/hwid-device-limit"
                                                target="_blank"
                                                rel="noreferrer"
                                                >{part}</a
                                            >
                                        {:else if hwidDescription[index - 1] === '<highlight>'}
                                            <code>{part}</code>
                                        {:else}
                                            {part}
                                        {/if}
                                    {/if}
                                {/each}
                            </small>
                            <label class="checkbox hwid-checkbox">
                                <input type="checkbox" bind:checked={noDeviceLimit}>
                                {t('create-user-modal.widget.disable-hwid-limit')}
                            </label>
                            <span class="input-wrap">
                                <span class="leading"><Icon name="TbDevices2" size={16} /></span>
                                <input
                                    id="device-limit"
                                    type="number"
                                    name="hwidDeviceLimit"
                                    min="0"
                                    step="1"
                                    value={original?.hwidDeviceLimit || ''}
                                    disabled={noDeviceLimit}
                                    placeholder="Fallback Device Limit in use"
                                >
                            </span>
                        </div>
                        <SelectField
                            name="tag"
                            label={t('common.field.tag')}
                            description={t('create-user-modal.widget.create-or-select-tag')}
                            leading="PiTagDuotone"
                            placeholder="EXAMPLE_TAG_1"
                            autocomplete="create"
                            pattern={'[A-Z0-9_]{1,16}'}
                            preferAbove
                            options={(tags.data?.tags ?? []).map((value) => ({
                                value,
                                label: value
                            }))}
                            value={tag}
                            onchoose={(value) => {
                                tag = value
                                touched.add('tag')
                            }}
                        />
                        <label class="form-field description-field">
                            <span>{t('common.field.description')}</span>
                            <small>{t('create-user-modal.widget.user-description')}</small>
                            <textarea
                                name="description"
                                oninput={() => touched.add('description')}
                                rows="2"
                                value={original?.description ?? ''}
                            ></textarea>
                        </label>
                    {/snippet}
                </FormCard>
            </div>

            <div class="form-column">
                <FormCard
                    class="traffic"
                    title={t('traffic-limits-card.traffic-and-limits')}
                    icon="TbChartLine"
                    tone="violet"
                >
                    {#snippet children()}
                        <div class="traffic-limit">
                            {@render inputField(
                                t('traffic-limits-card.traffic-limit'),
                                t('traffic-limits-card.traffic-limit-description'),
                                'TbChartLine',
                                amount
                            )}
                            {#snippet amount(controlId: string)}
                                <input
                                    id={controlId}
                                    type="number"
                                    name="trafficLimit"
                                    oninput={() => touched.add('trafficLimit')}
                                    min="0"
                                    step="any"
                                    value={initialBytes / (units.get(initialUnit) ?? 1)}
                                >
                            {/snippet}
                            <SelectField
                                name="trafficUnit"
                                label="Traffic unit"
                                placeholder=""
                                options={Array.from(units.keys(), (unit) => ({
                                    value: unit,
                                    label: unit
                                }))}
                                value={trafficUnit}
                                onchoose={(unit) => {
                                    trafficUnit = unit
                                    touched.add('trafficLimit')
                                }}
                                searchable={false}
                                clearable={false}
                                hideLabel
                            />
                        </div>
                        <SelectField
                            name="trafficLimitStrategy"
                            label={t('create-user-modal.widget.traffic-reset-strategy')}
                            description={t(
                                'create-user-modal.widget.traffic-reset-strategy-description'
                            )}
                            placeholder={t('create-user-modal.widget.pick-value')}
                            leading="PiClockDuotone"
                            options={trafficStrategies.map((value) => ({
                                value,
                                label: t(strategyLabels[value])
                            }))}
                            value={trafficStrategy}
                            onchoose={(value) => {
                                const strategy = trafficStrategies.find(
                                    (strategy) => strategy === value
                                )
                                if (!strategy) return
                                trafficStrategy = strategy
                                touched.add('trafficLimitStrategy')
                            }}
                            searchable={false}
                            clearable={false}
                        />
                    {/snippet}
                </FormCard>

                <FormCard
                    class="access"
                    title={t('access-settings-card.access-settings')}
                    icon="TbShield"
                    tone="indigo"
                >
                    {#snippet children()}
                        {@render inputField(
                            t('create-user-modal.widget.expiry-date'),
                            t('create-user-modal.widget.expire-at-description'),
                            'PiCalendarDuotone',
                            expiration
                        )}
                        {#snippet expiration(controlId: string)}
                            <input type="hidden" name="expireAt" value={expireAt}>
                            <button
                                class="mantine-UnstyledButton-root date"
                                type="button"
                                id={controlId}
                                aria-haspopup="dialog"
                                aria-expanded={expirationOpen}
                                onclick={() => (expirationOpen = true)}
                            >
                                {dateText}
                            </button>
                        {/snippet}
                        <fieldset class="squads">
                            <legend>{t('common.field.internal-squads')}</legend>
                            <small
                                >{t('create-user-modal.widget.internal-squads-description')}</small
                            >
                            <span class="input-wrap">
                                <span class="leading"
                                    ><Icon name="TbCirclesRelation" size={16} /></span
                                >
                                <input
                                    type="search"
                                    aria-label={t(
                                        'internal-squads-list.widget.search-internal-squads'
                                    )}
                                    placeholder={t(
                                        'internal-squads-list.widget.search-internal-squads'
                                    )}
                                    bind:value={squadSearch}
                                >
                            </span>
                            <div
                                class="squad-list"
                                style:height={`${Math.min(200, Math.max(1, visibleSquads.length) * 80)}px`}
                            >
                                {#each visibleSquads as squad (squad.uuid)}
                                    {@const info = squadInfo.get(squad.uuid)}
                                    <div
                                        class="squad"
                                        class:selected={chosenSquads.includes(squad.uuid)}
                                    >
                                        <label class="checkbox">
                                            <input
                                                type="checkbox"
                                                class="mantine-Checkbox-input"
                                                bind:group={chosenSquads}
                                                value={squad.uuid}
                                            >
                                            <strong>{squad.name}</strong>
                                        </label>
                                        {#if info}
                                            <span class="squad-count members">
                                                <Icon name="PiUsers" size={16} />
                                                {info.membersCount.toLocaleString('en-US')}
                                            </span>
                                            <span class="squad-count inbounds">
                                                <Icon name="PiTag" size={16} />
                                                {info.inboundsCount}
                                            </span>
                                        {/if}
                                        <ActionIcon
                                            icon="TbEdit"
                                            label={`${t('common.action.edit')} ${squad.name}`}
                                            size={34}
                                            iconSize={16}
                                            onclick={() => (editingSquad = squad.uuid)}
                                        />
                                    </div>
                                {:else}
                                    <div class="squad-empty">
                                        <Icon name="PiEmpty" size={48} />
                                        <span
                                            >{t(
                                                'internal-squads-list.widget.no-squads-found'
                                            )}</span
                                        >
                                    </div>
                                {/each}
                            </div>
                        </fieldset>
                        <SelectField
                            name="externalSquadUuid"
                            label={t('access-settings-card.external-squad')}
                            description={t(
                                'access-settings-card.select-an-external-squad-to-apply-custom-settings-to-this-user'
                            )}
                            placeholder={t('access-settings-card.select-an-external-squad')}
                            leading="TbWebhook"
                            options={externalChoices}
                            value={original?.externalSquadUuid ?? ''}
                            disabled={!options.external}
                            searchable={false}
                        />
                    {/snippet}
                </FormCard>
            </div>
        </div>
        {#if error}
            <p class="error" role="alert">{error}</p>
        {/if}
        <div class="user-footer">
            {@render actions?.()}
            <Button
                type="submit"
                icon="PiFloppyDiskDuotone"
                size="md"
                variant={original ? 'soft' : 'light'}
                color="teal"
                disabled={save.isPending}
            >
                {t(original ? 'common.action.save' : 'common.action.create')}
            </Button>
        </div>
    </form>
    {#if expirationOpen}
        <ExpirationPicker
            value={expireAt}
            close={() => (expirationOpen = false)}
            onchoose={(value) => {
                expireAt = value
                expirationOpen = false
            }}
        />
    {/if}
    {@render overlays?.()}
    {#if editingSquad}
        <SquadInbounds uuid={editingSquad} close={() => (editingSquad = undefined)} />
    {/if}
</Overlay>

<style>
    .form-columns,
    .form-column {
        display: grid;
        min-width: 0;
        gap: var(--mantine-spacing-md);
        align-content: start;
    }
    .form-columns {
        grid-template-columns: repeat(2, minmax(0, 1fr));
        align-items: start;
    }
    .form-column {
        --select-label-weight: 500;
    }
    form {
        margin: 0;
    }
    .form-field,
    .squads {
        min-width: 0;
        display: grid;
        gap: 2px;
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
    }
    .device-limit {
        gap: 0;
    }
    .hwid-title {
        font-weight: 600;
    }
    small {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        font-weight: 400;
        line-height: var(--mantine-line-height-xs);
    }
    .description-field small {
        line-height: 1.2;
    }
    .description-field {
        margin-block-start: 2px;
        gap: 1px;
    }
    .hwid-description {
        line-height: 1;
    }
    small code {
        padding: 2px calc(var(--mantine-spacing-xs) / 2);
        margin: 0;
        border-radius: var(--mantine-radius-sm);
        background: light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-6));
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height);
    }
    .input-wrap {
        position: relative;
        display: grid;
        min-width: 0;
        margin-block-start: 2px;
    }
    .device-limit .input-wrap {
        margin-block-start: 0;
    }
    .leading {
        display: grid;
        width: 34px;
        place-items: center;
        position: absolute;
        inset-block: 0;
        inset-inline-start: 0;
        z-index: 1;
        color: var(--mantine-color-dimmed);
        pointer-events: none;
    }
    @media (prefers-reduced-motion: no-preference) {
        .form-column > :global(.form-card) {
            animation: form-card-enter 300ms ease-out both;
        }
        .form-column > :global(.form-card:nth-child(2)) {
            animation-delay: 100ms;
        }
        .form-column > :global(.form-card:nth-child(3)) {
            animation-delay: 200ms;
        }
    }

    @keyframes form-card-enter {
        from {
            opacity: 0;
            transform: translateY(20px);
        }
        to {
            opacity: 1;
            transform: none;
        }
    }

    .input-wrap :is(input:not([type="hidden"]), .date) {
        min-width: 0;
        width: 100%;
        min-height: 36px;
        height: 36px;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-dark-6);
        padding-block: 0;
        padding-inline-start: 34px;
    }
    .form-field input::placeholder,
    .squads input::placeholder {
        color: var(--mantine-color-placeholder);
        opacity: 1;
    }
    .date {
        position: relative;
        display: flex;
        align-items: center;
        font-weight: 400;
    }
    .date:focus-visible {
        border-color: var(--mantine-primary-color-filled);
    }
    .traffic-limit {
        display: grid;
        grid-template-columns: minmax(0, 1fr) 92px;
        gap: var(--mantine-spacing-xs);
        align-items: end;
    }
    .required {
        color: var(--mantine-color-error);
    }
    .squads {
        padding: 2.4px 0 0;
        margin: 0;
        border: 0;
        gap: 1px;
    }
    .squads legend {
        padding: 0;
        transform: translateY(2.4px);
        font-weight: 600;
    }
    .squads > small {
        line-height: 1.2;
        margin-block-start: 1.2px;
    }
    .squads > .input-wrap {
        margin-block-start: 4px;
    }
    .checkbox {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .device-limit .hwid-checkbox {
        gap: 12px;
        margin-block: var(--mantine-spacing-xs);
        font-weight: 400;
        line-height: 20px;
    }
    .squad-list {
        max-height: 200px;
        overflow: auto;
        padding: 8px;
        margin-block-start: 15px;
        border: 1px solid var(--mantine-color-gray-7);
        border-radius: var(--mantine-radius-md);
    }
    .squad {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        min-height: 48px;
        padding: 8px 12px;
        border: 1px solid light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-4));
        border-radius: var(--mantine-radius-md);
    }
    .squad + .squad {
        margin-block-start: 8px;
    }
    .squad-empty {
        display: flex;
        height: 100%;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: var(--mantine-spacing-md);
        padding-block-start: 80px;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        text-align: center;
    }
    .squad.selected {
        border-color: var(--mantine-primary-color-filled);
        background: light-dark(var(--mantine-primary-color-light), transparent);
    }
    .squad .checkbox {
        flex: 1;
        min-width: 0;
        cursor: pointer;
    }
    .squad strong {
        overflow: hidden;
        color: var(--mantine-color-bright);
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-xs);
        font-weight: 600;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .squad-count {
        display: inline-flex;
        align-items: center;
        gap: 5px;
        padding-inline: 9px;
        border-radius: 100vw;
        color: var(--mantine-color-teal-light-color);
        background: var(--mantine-color-teal-light);
        font-size: var(--mantine-font-size-xs);
        font-weight: 700;
    }
    .squad-count.inbounds {
        color: var(--mantine-color-blue-light-color);
        background: var(--mantine-color-blue-light);
    }
    @media (max-width: 48em) {
        .squad-count.members {
            display: none;
        }
    }
    textarea {
        padding: 5.5px 12px;
        resize: vertical;
    }
    .description-field > textarea {
        margin-block-start: 4px;
    }
    .user-footer {
        position: sticky;
        bottom: -1px;
        z-index: 1;
        display: flex;
        justify-content: end;
        gap: var(--mantine-spacing-md);
        padding: var(--mantine-spacing-sm) var(--mantine-spacing-md);
        margin: var(--mantine-spacing-md) calc(-1 * var(--mantine-spacing-md))
            calc(-1 * var(--mantine-spacing-md));
        border-block-start: 1px solid rgba(var(--gw-fg-rgb), 0.06);
        background: color-mix(in srgb, var(--mantine-color-body) 91%, transparent);
    }
    .error {
        white-space: pre-wrap;
    }
    @media (width > 64rem) {
        .tag-field > span:first-child {
            position: relative;
            inset-block-start: 1px;
        }
        :global(.select-field > span:first-child) {
            position: relative;
            inset-block-start: 0.7px;
        }
        form:not(.editing) .tag-field > span:first-child {
            inset-block-start: 3px;
        }
        form:not(.editing) .description-field > span:first-child {
            position: relative;
            inset-block-start: 2px;
        }
        .form-field:not(.tag-field):not(.description-field):not(.device-limit) > span:first-child {
            position: relative;
            inset-block-start: 2.5px;
        }
    }
    @media (max-width: 64rem) {
        .form-columns {
            grid-template-columns: minmax(0, 1fr);
        }
        .squads > .input-wrap {
            margin-block-start: 0;
        }
        .form-field.tag-field:not(.device-limit) > .input-wrap {
            margin-block-start: 4px;
        }
        .form-column > :global(.access) :global(.card-body) {
            padding-block-start: 2px;
        }
        .form-field:not(.tag-field):not(.device-limit) > .input-wrap {
            margin-block-start: 1px;
        }
        form:not(.editing) .form-column > :global(.form-card.identity) :global(.card-body) {
            padding-block-start: 2px;
        }
        form:not(.editing) .form-column > :global(.form-card.identity) {
            margin-block-start: 1px;
        }
        form:not(.editing) .form-column > :global(.traffic .form-field > .input-wrap) {
            margin-block-start: 1.7px;
        }
        form:not(.editing) .form-column > :global(.form-card.identity .form-field > .input-wrap) {
            margin-block-start: -0.3px;
        }
        form.editing .form-column > :global(.identity.identity-card) {
            margin-block-start: 1px;
            padding-block-start: calc(var(--mantine-spacing-md) - 1px);
            padding-block-end: calc(var(--mantine-spacing-md) - 0.7px);
        }
        form.editing .form-column > :global(.form-card.traffic .card-body) {
            padding-block-start: 0.7px;
        }
        .form-column {
            display: contents;
        }
        .form-columns :global(.identity) {
            order: 0;
        }
        .form-columns :global(.traffic) {
            order: 1;
        }
        .form-columns :global(.access) {
            order: 2;
        }
        .form-columns :global(.contact) {
            order: 3;
        }
        .form-columns :global(.device) {
            order: 4;
        }
        .user-footer :global(.mantine-Button-root:last-child) {
            flex: 1 1 auto;
        }
        .editing .user-footer > :global(.menu-trigger) {
            flex: 1 1 auto;
        }
        .editing .user-footer > :global(.menu-trigger .mantine-Button-root) {
            width: 100%;
        }
    }
</style>
