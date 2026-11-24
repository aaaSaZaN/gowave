<script lang="ts">
    import {
        CreateInfraBillingNodeCommand,
        CreateInfraBillingRecordCommand,
        CreateInfraProviderCommand,
        UpdateInfraBillingNodeCommand,
        UpdateInfraProviderCommand
    } from '@remnawave/backend-contract'
    import { toUtcDayISO } from '@shared/utils/time-utils/to-utc-day-iso/to-utc-day-iso.util'
    import dayjs from 'dayjs'
    import { untrack } from 'svelte'

    import { submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { flag } from '../nodes/analytics'
    import { queryClient } from '../query-client'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import Calendar from '../ui/Calendar.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import Overlay from '../ui/Overlay.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import TablePopup from '../ui/TablePopup.svelte'
    import { type AvailableNode, type BillingNode, billingKey, type Provider } from './queries'

    type Mode =
        | { kind: 'provider'; provider?: Provider }
        | { kind: 'node' }
        | { kind: 'record' }
        | { kind: 'date'; nodes: BillingNode[] }

    let {
        mode,
        providers,
        available,
        close
    }: {
        mode: Mode
        providers: Provider[]
        available: AvailableNode[]
        close: () => void
    } = $props()

    const id = $props.id()
    let busy = $state(false)
    let error = $state('')
    let nodeType = $state('existing')
    let nodeUuid = $state('')
    let providerUuid = $state('')
    let t = $derived(locale.t)
    const today = dayjs()
    let provider = $derived(mode.kind === 'provider' ? mode.provider : undefined)
    const currentDate = $derived(
        mode.kind === 'date' && mode.nodes.length === 1
            ? mode.nodes[0]?.nextBillingAt.toISOString().slice(0, 10)
            : undefined
    )
    let date = $state(untrack(() => currentDate ?? today.format('YYYY-MM-DD')))
    let calendarOpen = $state(false)
    let dateTrigger = $state<HTMLButtonElement>()
    const providerLink = $derived(providers.find((item) => item.uuid === providerUuid)?.loginUrl)
    let title = $derived(
        mode.kind === 'date'
            ? t('update-billing-date-modal.widget.update-billing-date')
            : mode.kind === 'node'
              ? t('common.field.billing-node')
              : mode.kind === 'record'
                ? t('create-infra-billing-record.modal.widget.bill-record')
                : t('view-infra-provider.drawer.widget.infra-provider')
    )
    const icon = $derived(
        mode.kind === 'date' ? 'TbCalendar' : mode.kind === 'record' ? 'TbInvoice' : 'TbServer'
    )

    function dateText(value: string, comma = false): string {
        return dayjs(value)
            .locale(locale.language)
            .format(comma ? 'D MMMM, YYYY' : 'D MMMM YYYY')
    }

    async function save(event: SubmitEvent): Promise<void> {
        event.preventDefault()
        if (busy || !(event.currentTarget instanceof HTMLFormElement)) return
        const form = new FormData(event.currentTarget)
        const value = (name: string) => String(form.get(name) ?? '').trim()
        const billingAt = toUtcDayISO(date)
        if (
            mode.kind === 'node' &&
            (!providerUuid || (nodeType === 'existing' ? !nodeUuid : !value('name')))
        ) {
            error = t(
                'create-infra-billing-node.modal.widget.please-select-a-provider-and-billing-node'
            )
            return
        }
        if (mode.kind === 'record' && !providerUuid) {
            error = t('create-infra-billing-record.modal.widget.please-select-a-provider')
            return
        }
        busy = true
        error = ''
        try {
            switch (mode.kind) {
                case 'provider': {
                    const name = value('name')
                    const faviconLink = value('faviconLink')
                    const loginUrl = value('loginUrl')
                    if (mode.provider) {
                        await submitCommand(UpdateInfraProviderCommand, {
                            uuid: mode.provider.uuid,
                            name,
                            faviconLink: faviconLink || null,
                            loginUrl: loginUrl || null
                        })
                    } else {
                        await submitCommand(CreateInfraProviderCommand, {
                            name,
                            faviconLink: faviconLink || undefined,
                            loginUrl: loginUrl || undefined
                        })
                    }
                    break
                }
                case 'node':
                    await submitCommand(CreateInfraBillingNodeCommand, {
                        providerUuid,
                        nodeUuid: nodeType === 'existing' ? nodeUuid : null,
                        name: nodeType === 'existing' ? null : value('name'),
                        nextBillingAt: billingAt
                    })
                    break
                case 'record':
                    await submitCommand(CreateInfraBillingRecordCommand, {
                        providerUuid,
                        billedAt: billingAt,
                        amount: Number(value('amount').replaceAll(',', ''))
                    })
                    break
                case 'date':
                    await submitCommand(UpdateInfraBillingNodeCommand, {
                        uuids: mode.nodes.map((node) => node.uuid),
                        nextBillingAt: billingAt
                    })
                    break
            }
            await queryClient.invalidateQueries({ queryKey: billingKey })
            close()
        } catch (failure) {
            error = failure instanceof Error ? failure.message : t('common.message.error')
        } finally {
            busy = false
        }
    }
</script>

{#snippet textField(
    name: string,
    label: string,
    options: {
        value?: string
        placeholder?: string
        description?: string
        icon?: IconName
        iconSize?: number
        required?: boolean
        readonly?: boolean
        minLength?: number
        maxLength?: number
        amount?: boolean
        focus?: boolean
    }
)}
    {@const fieldId = `${id}-${name}`}
    <div class="field">
        <label for={fieldId} class="m_8fdc1311"
            >{label}
            {#if options.required}
                <span class="required" aria-hidden="true">*</span>
            {/if}</label
        >
        {#if options.description}
            <div id={`${fieldId}-description`} class="m_fe47ce59">{options.description}</div>
        {/if}
        <div
            class="m_6c018570 control"
            data-variant="default"
            data-with-left-section={options.icon ? true : undefined}
        >
            {#if options.icon}
                <span class="m_82577fc2" data-position="left"
                    ><Icon name={options.icon} size={options.iconSize ?? 16} /></span
                >
            {/if}
            <input
                id={fieldId}
                class="m_8fb7ebe7 mantine-Input-input"
                {name}
                value={options.value ?? ''}
                placeholder={options.placeholder}
                required={options.required}
                readonly={options.readonly}
                minlength={options.minLength}
                maxlength={options.maxLength}
                inputmode={options.amount ? 'decimal' : undefined}
                pattern={options.amount
                    ? '(?:[0-9]+(?:,[0-9]{3})*(?:[.][0-9]{1,2})?|[.][0-9]{1,2})'
                    : undefined}
                aria-describedby={options.description ? `${fieldId}-description` : undefined}
                data-autofocus={options.focus ? '' : undefined}
                onblur={(event) => {
                    if (
                        !options.amount ||
                        !event.currentTarget.validity.valid ||
                        !event.currentTarget.value
                    )
                        return
                    const parsed = Number(event.currentTarget.value.replaceAll(',', ''))
                    if (Number.isFinite(parsed))
                        event.currentTarget.value = parsed.toLocaleString('en-US', {
                            minimumFractionDigits: 2,
                            maximumFractionDigits: 2
                        })
                }}
            >
        </div>
    </div>
{/snippet}

{#snippet billingProviders()}
    <div class="providers">
        <SelectField
            name="provider"
            label={t('select-infra-provider.shared.infrastructure-provider')}
            description={t('select-infra-provider.shared.select-the-infrastructure-provider')}
            placeholder={t(
                providers.length
                    ? 'select-infra-provider.shared.select-provider'
                    : 'select-infra-provider.shared.no-providers-available'
            )}
            options={providers.map(({ uuid, name }) => ({ value: uuid, label: name }))}
            bind:value={providerUuid}
        />
        <ActionIcon
            icon="TbExternalLink"
            label={t('common.action.open')}
            size={36}
            iconSize={18}
            disabled={!providerLink || !/^https?:\/\//i.test(providerLink)}
            onclick={() => {
                if (providerLink && /^https?:\/\//i.test(providerLink))
                    window.open(providerLink, '_blank', 'noopener,noreferrer')
            }}
        />
    </div>
{/snippet}

{#snippet dateField(label: string, description: string, min?: string, max?: string)}
    <div class="field">
        <label for={`${id}-date-trigger`} class="m_8fdc1311"
            >{label} <span class="required" aria-hidden="true">*</span></label
        >
        <div id={`${id}-date-description`} class="m_fe47ce59">{description}</div>
        <div class="m_6c018570 control" data-pointer data-variant="default" data-with-left-section>
            <span class="m_82577fc2" data-position="left"
                ><Icon name="HiCalendar" size={16} /></span
            >
            <button
                bind:this={dateTrigger}
                id={`${id}-date-trigger`}
                class="m_8fb7ebe7 mantine-Input-input date-trigger"
                data-autofocus={mode.kind === 'node' && nodeType === 'existing' ? '' : undefined}
                type="button"
                aria-expanded={calendarOpen}
                aria-haspopup="dialog"
                aria-describedby={`${id}-date-description`}
                aria-controls={calendarOpen ? `${id}-calendar` : undefined}
                onclick={() => (calendarOpen = !calendarOpen)}
            >
                {dateText(date, true)}
            </button>
        </div>
    </div>
    {#if calendarOpen && dateTrigger}
        <TablePopup
            anchor={dateTrigger}
            {label}
            id={`${id}-calendar`}
            width={269}
            role="dialog"
            close={() => (calendarOpen = false)}
        >
            <Calendar
                bind:value={date}
                {min}
                {max}
                onselect={() => {
                    calendarOpen = false
                    dateTrigger?.focus({ preventScroll: true })
                }}
            />
        </TablePopup>
    {/if}
{/snippet}

<Overlay
    {title}
    {icon}
    tone="teal"
    width={mode.kind === 'date' ? 407 : 440}
    class="billing-dialog"
    preventClose={busy}
    {close}
>
    <form
        data-autofocus={mode.kind === 'date' || mode.kind === 'provider' ? '' : undefined}
        tabindex="-1"
        onsubmit={save}
    >
        <fieldset class:date-form={mode.kind === 'date'} disabled={busy}>
            {#if mode.kind === 'provider'}
                {@render textField('name', t('common.field.name'), {
                    value: provider?.name ?? '',
                    required: true,
                    placeholder: t('view-infra-provider.drawer.widget.enter-provider-name'),
                    description: t('view-infra-provider.drawer.widget.name-description'),
                    minLength: 2,
                    maxLength: 30
                })}
                {@render textField(
                    'faviconLink',
                    t('view-infra-provider.drawer.widget.favicon-link'),
                    {
                        value: provider?.faviconLink ?? '',
                        required: !provider,
                        placeholder: provider
                            ? t('view-infra-provider.drawer.widget.enter-favicon-link')
                            : 'https://hetzner.com',
                        description: t('view-infra-provider.drawer.widget.favicon-link-description')
                    }
                )}
                {@render textField('loginUrl', t('view-infra-provider.drawer.widget.login-url'), {
                    value: provider?.loginUrl ?? '',
                    required: !provider,
                    placeholder: provider
                        ? t('view-infra-provider.drawer.widget.enter-login-url')
                        : 'https://cloud.hetzner.com',
                    description: t('view-infra-provider.drawer.widget.login-url-description')
                })}
            {:else}
                <input type="hidden" name="date" value={date}>
                {#if mode.kind === 'date'}
                    {#if currentDate}
                        {@render textField(
                            'currentDate',
                            t('update-billing-date-modal.widget.current-date'),
                            {
                                value: dateText(currentDate),
                                readonly: true
                            }
                        )}
                    {/if}
                    {@render textField('newDate', t('update-billing-date-modal.widget.new-date'), {
                        value: dateText(date),
                        readonly: true
                    })}
                    <Calendar
                        bind:value={date}
                        layout="billing"
                        max={today.add(2, 'year').format('YYYY-MM-DD')}
                    >
                        {#snippet shortcuts(choose)}
                            <button
                                class="mantine-UnstyledButton-root"
                                type="button"
                                onclick={() => choose(today)}
                            >
                                {t('common.field.today')}
                            </button>
                            <button
                                class="mantine-UnstyledButton-root"
                                type="button"
                                onclick={() => choose(today.add(1, 'day'))}
                            >
                                {t('update-billing-date-modal.widget.tomorrow')}
                            </button>
                            <button
                                class="mantine-UnstyledButton-root"
                                type="button"
                                disabled={dayjs(date)
                                    .add(1, 'month')
                                    .isAfter(today.add(2, 'year'), 'day')}
                                onclick={() => choose(dayjs(date).add(1, 'month'))}
                            >
                                {t('update-billing-date-modal.widget.next-month')}
                            </button>
                        {/snippet}
                    </Calendar>
                {:else if mode.kind === 'node'}
                    <fieldset class="node-types">
                        <legend class="sr-only">{t('common.field.billing-node')}</legend>
                        {#each [
                            {
                                value: 'existing',
                                icon: 'TbServer',
                                label: t('create-infra-billing-node.modal.widget.existing')
                            },
                            {
                                value: 'custom',
                                icon: 'TbCursorText',
                                label: t('create-infra-billing-node.modal.widget.custom')
                            }
                        ] as const as option (option.value)}
                            <label class:active={nodeType === option.value}>
                                <input
                                    type="radio"
                                    class="sr-only"
                                    name={`${id}-node-type`}
                                    value={option.value}
                                    bind:group={nodeType}
                                    onchange={() => {
                                        if (option.value === 'custom') nodeUuid = ''
                                    }}
                                >
                                <Icon name={option.icon} size={16} />{option.label}
                            </label>
                        {/each}
                    </fieldset>
                    {#if nodeType === 'existing'}
                        <SelectField
                            name="node"
                            label={t('common.field.billing-node')}
                            description={available.length
                                ? t('select-billing-node.shared.select-the-billing-node')
                                : undefined}
                            placeholder={t(
                                available.length
                                    ? 'select-billing-node.shared.select-billing-node'
                                    : 'select-billing-node.shared.no-billing-nodes-available'
                            )}
                            options={available.map(({ uuid, name, countryCode }) => ({
                                value: uuid,
                                label: name,
                                flag: flag(countryCode)
                            }))}
                            bind:value={nodeUuid}
                        />
                    {:else}
                        {@render textField(
                            'name',
                            t('create-infra-billing-node.modal.widget.custom'),
                            {
                                placeholder: 'Management Server',
                                description: t(
                                    'create-infra-billing-node.modal.widget.custom-name-description'
                                ),
                                icon: 'TbCursorText',
                                required: true,
                                maxLength: 255,
                                focus: true
                            }
                        )}
                    {/if}
                    {@render billingProviders()}
                    {@render dateField(
                        t('create-infra-billing-node.modal.widget.next-billing-at'),
                        t('create-infra-billing-node.modal.widget.next-billing-at-description'),
                        today.subtract(1, 'day').format('YYYY-MM-DD')
                    )}
                {:else}
                    {@render billingProviders()}
                    {@render dateField(
                        t('create-infra-billing-record.modal.widget.billed-at'),
                        t(
                            'create-infra-billing-record.modal.widget.the-date-and-time-when-the-bill-was-paid'
                        ),
                        undefined,
                        today.add(1, 'day').format('YYYY-MM-DD')
                    )}
                    {@render textField(
                        'amount',
                        t('create-infra-billing-record.modal.widget.amount'),
                        {
                            description: t(
                                'create-infra-billing-record.modal.widget.payment-amount-usd'
                            ),
                            amount: true,
                            icon: 'HiCurrencyDollar',
                            iconSize: 20,
                            required: true,
                            focus: true
                        }
                    )}
                {/if}
            {/if}
            {#if error}
                <p class="error" role="alert">{error}</p>
            {/if}
            {#if mode.kind === 'date'}
                <div class="date-submit">
                    <Button type="submit" variant="light" disabled={busy}
                        >{t('common.action.update')}</Button
                    >
                </div>
            {:else}
                <Button
                    type="submit"
                    block
                    variant={mode.kind === 'provider' && !provider ? 'light' : 'soft'}
                    disabled={busy}
                >
                    {t(provider ? 'common.action.save' : 'common.action.create')}
                </Button>
            {/if}
        </fieldset>
    </form>
</Overlay>

<style>
    form:focus {
        outline: none;
    }
    fieldset {
        display: grid;
        min-width: 0;
        gap: var(--mantine-spacing-md);
        border: 0;
        margin: 0;
        padding: 0;
    }
    .date-form {
        gap: var(--mantine-spacing-xs);
        grid-template-columns: minmax(0, 1fr);
    }
    .control {
        --input-fz: var(--mantine-font-size-sm);
    }
    .field:has(.m_fe47ce59) .control {
        margin-block-start: 5px;
    }
    .required,
    .error {
        color: var(--mantine-color-error);
    }
    .error {
        margin: 0;
        font-size: var(--mantine-font-size-sm);
    }
    .date-submit {
        display: flex;
        justify-content: flex-end;
        margin-block-start: 26px;
    }
    .date-trigger {
        line-height: normal;
    }
    .providers {
        display: flex;
        align-items: flex-end;
        gap: var(--mantine-spacing-xs);
    }
    .node-types {
        display: flex;
        padding: 4px;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-dark-6);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
    }
    .node-types label {
        display: flex;
        flex: 1;
        align-items: center;
        justify-content: center;
        gap: var(--mantine-spacing-xs);
        padding: 2px;
        min-height: 28px;
        border: 1px solid transparent;
        border-radius: var(--mantine-radius-sm);
        cursor: pointer;
        font-weight: 500;
    }
    .node-types .active {
        border-color: rgb(21 170 191 / 30%);
        background: var(--mantine-primary-color-light);
        color: var(--mantine-color-cyan-4);
    }
    .node-types label:has(input:focus-visible) {
        outline: 2px solid var(--mantine-primary-color-filled);
        outline-offset: -2px;
    }
    .sr-only {
        position: absolute;
        width: 1px;
        height: 1px;
        overflow: hidden;
        clip-path: inset(50%);
    }
    :global(dialog.billing-dialog .overlay-header) {
        padding-block-start: calc(var(--mantine-spacing-sm) + 1px);
    }
    @media (max-width: 64rem) {
        :global(dialog.panel-overlay.billing-dialog) {
            inset: 0;
            width: 100%;
            height: 100dvh;
            max-height: none;
            margin: 0;
            border-radius: 0;
        }
    }
</style>
