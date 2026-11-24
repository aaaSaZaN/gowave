<script lang="ts">
    import {
        BulkNodesProfileModificationCommand,
        BulkNodesUpdateCommand,
        CreateNodeCommand,
        NODE_IP_STATUSES,
        UpdateNodeCommand
    } from '@remnawave/backend-contract'
    import {
        TRAFFIC_TRACKING_START,
        UpdateNodeBodySchema,
        UpdateNodeResponseSchema
    } from '@shared/api/schemas/node.schema'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { type Snippet, untrack } from 'svelte'
    import { SvelteSet } from 'svelte/reactivity'
    import { submitCommand, submitVoidCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { nodePluginsQuery } from '../plugins/queries'
    import { queryClient } from '../query-client'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import FormCard from '../ui/FormCard.svelte'
    import Icon from '../ui/Icon.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import Switch from '../ui/Switch.svelte'
    import { COUNTRIES } from './countries'
    import {
        gibibyte,
        type NodeField,
        type NodePatch,
        readConfigProfile,
        readNodePatch
    } from './fields'
    import NodeInstall from './NodeInstall.svelte'
    import NodeProviderField from './NodeProviderField.svelte'
    import ProfileFields from './ProfileFields.svelte'
    import {
        type NodeDetail,
        nodeQuery,
        nodeSecretQuery,
        nodesKey,
        providersQuery
    } from './queries'

    let {
        node,
        ids,
        profileOnly = false,
        details,
        system,
        actions,
        onsaved
    }: {
        node?: NodeDetail
        ids?: string[]
        profileOnly?: boolean
        details?: Snippet
        system?: Snippet
        actions?: Snippet
        onsaved: (node?: NodeDetail) => void
    } = $props()
    let initial = $state.raw(untrack(() => node))
    let profileVersion = $state(0)
    let t = $derived(locale.t)
    let mode: 'bulk' | 'edit' | 'create' = $derived(ids ? 'bulk' : node ? 'edit' : 'create')
    const changed = new SvelteSet<string>()
    const enabled = new SvelteSet<NodeField>()
    const providers = createQuery(() => ({ ...providersQuery, enabled: !profileOnly }))
    const plugins = createQuery(() => ({ ...nodePluginsQuery, enabled: !profileOnly }))
    const secret = createQuery(() => ({ ...nodeSecretQuery, enabled: mode === 'edit' }))
    const secretKeyId = $props.id()
    let providerUuid = $state(untrack(() => initial?.providerUuid ?? ''))
    let activePluginUuid = $state(untrack(() => initial?.activePluginUuid ?? ''))
    let ports = $state<number | undefined>(untrack(() => initial?.port ?? undefined))
    let countryCode = $state(untrack(() => initial?.countryCode ?? 'XX'))
    const countryOptions = $derived(
        initial?.countryCode && !COUNTRIES.some(({ value }) => value === initial?.countryCode)
            ? [{ value: initial.countryCode, label: initial.countryCode }, ...COUNTRIES]
            : COUNTRIES
    )
    let ips = $state(untrack(() => initial?.ips.map((ip, index) => ({ ...ip, key: index })) ?? []))
    let nextIp = untrack(() => initial?.ips.length ?? 0)
    let tracking = $state(untrack(() => initial?.isTrafficTrackingActive ?? false))
    let notice = $state('')
    let problem = $state('')

    const save = createMutation(() => ({
        mutationFn: async (patch: NodePatch): Promise<NodeDetail | undefined> => {
            if (ids) {
                if (profileOnly)
                    await submitVoidCommand(BulkNodesProfileModificationCommand, {
                        uuids: ids,
                        ...patch
                    })
                else await submitVoidCommand(BulkNodesUpdateCommand, { uuids: ids, fields: patch })
                return undefined
            }
            if (!node) return submitCommand(CreateNodeCommand, patch)
            return submitCommand(
                {
                    ...UpdateNodeCommand,
                    RequestBodySchema: UpdateNodeBodySchema,
                    ResponseSchema: UpdateNodeResponseSchema
                },
                { uuid: node.uuid, ...patch }
            )
        },
        onSuccess: async (result) => {
            if (result) {
                queryClient.setQueryData(nodeQuery(result.uuid).queryKey, result)
                if (node) {
                    initial = result
                    countryCode = result.countryCode
                    providerUuid = result.providerUuid ?? ''
                    activePluginUuid = result.activePluginUuid ?? ''
                    profileVersion += 1
                }
            }
            await queryClient.invalidateQueries({ queryKey: nodesKey })
            changed.clear()
            notice = t('common.message.operation-completed')
            onsaved(result)
        }
    }))

    let trackingStartOverride = $state<string>()
    let trackingStart = $derived(
        trackingStartOverride ??
            initial?.trafficTrackingStart ??
            (initial?.isTrafficTrackingActive ? 'keep' : 'period')
    )

    function markProfile(dirty: boolean): void {
        if (dirty) changed.add('configProfile')
        else changed.delete('configProfile')
        notice = ''
        problem = ''
    }

    function mark(event: Event): void {
        const target = event.target
        if (
            !(
                target instanceof HTMLInputElement ||
                target instanceof HTMLSelectElement ||
                target instanceof HTMLTextAreaElement
            )
        )
            return
        const key = target.name
        if (key) changed.add(key === 'profile' || key === 'inbounds' ? 'configProfile' : key)
        notice = ''
        problem = ''
    }

    function submit(event: SubmitEvent): void {
        event.preventDefault()
        if (save.isPending || !(event.currentTarget instanceof HTMLFormElement)) return
        save.reset()
        const data = new FormData(event.currentTarget)
        try {
            const patch = profileOnly
                ? { configProfile: readConfigProfile(data) }
                : readNodePatch(
                      data,
                      (key) => {
                          if (mode === 'bulk') return enabled.has(key)
                          if (key === 'trafficTrackingStart')
                              return (
                                  mode === 'edit' &&
                                  tracking &&
                                  (changed.has(key) || !initial?.isTrafficTrackingActive)
                              )
                          return mode === 'create' || changed.has(key)
                      },
                      mode
                  )
            if (!profileOnly && mode !== 'bulk' && (mode === 'create' || changed.has('ips'))) {
                patch.ips = ips
                    .filter(({ ip }) => ip.trim())
                    .map(({ ip, status }) => ({ ip: ip.trim(), status }))
            }
            if (!Object.keys(patch).length) {
                problem = 'Select the fields to update'
                return
            }
            if (
                ids &&
                !window.confirm(
                    `${t('common.action.update')} ${ids.length} ${t('constants.nodes')}?`
                )
            )
                return
            save.mutate(patch)
        } catch (error) {
            problem = error instanceof Error ? error.message : String(error)
        }
    }

    function addIp(): void {
        ips.push({ key: nextIp, ip: '', status: NODE_IP_STATUSES.UNKNOWN })
        nextIp += 1
    }

    function setApplied(key: NodeField, apply: boolean): void {
        if (apply) enabled.add(key)
        else enabled.delete(key)
    }
    function selectAssociation(field: 'providerUuid' | 'activePluginUuid', value: string): void {
        if (field === 'providerUuid') providerUuid = value
        else activePluginUuid = value
        changed.add(field)
        notice = ''
        problem = ''
    }

    function choicesWithSelection(
        items: readonly { uuid: string; name: string }[],
        selection: string | null | undefined
    ) {
        const options = items.map(({ uuid, name }) => ({ value: uuid, label: name }))
        if (selection && !options.some(({ value }) => value === selection))
            options.unshift({ value: selection, label: selection })
        return options
    }
</script>

<form class:single={!ids} onsubmit={submit} oninput={mark} onchange={mark}>
    <fieldset class="form" disabled={save.isPending}>
        {#if ids}
            {#if profileOnly}
                <FormCard
                    title={t('base-node-form.core-configuration')}
                    icon="SiSecurityscorecard"
                    tone="teal"
                >
                    <ProfileFields current={initial?.configProfile} onchange={markProfile} />
                </FormCard>
            {:else}
                <FormCard title={t('base-node-form.node-vitals')} icon="TbServer" tone="blue">
                    {@render input('countryCode', t('base-node-form.country'), '', 2)}
                    {@render input('tags', t('common.field.tags'), '')}
                    {@render input('note', t('node-tracking-and-billing.card.note'), '', 255)}
                    {@render entry(
                        'providerUuid',
                        t('select-infra-provider.shared.infrastructure-provider'),
                        providerField
                    )}
                    {@render entry('activePluginUuid', t('node-vitals.card.plugin'), pluginField)}
                    {@render multipliers()}
                </FormCard>
            {/if}
        {:else}
            <div class="column start">
                {#if details}
                    <div class="details">{@render details()}</div>
                {/if}
                <FormCard
                    class="vitals"
                    title={t('base-node-form.node-vitals')}
                    description={initial?.uuid}
                    icon="HiOutlineServer"
                    tone="blue"
                >
                    <SelectField
                        name="countryCode"
                        label={t('base-node-form.country')}
                        placeholder={t('base-node-form.select-country')}
                        leading="TbMapPin"
                        options={countryOptions}
                        clearable={false}
                        required
                        bind:value={
                            () => countryCode,
                            (value) => {
        countryCode = value
        changed.add('countryCode')
        notice = ''
        problem = ''
    }
                        }
                    />
                    <label>
                        <span class="field-caption"
                            >{t('base-node-form.internal-name')}
                            <span class="required">*</span></span
                        >
                        <span class="node-input">
                            <Icon name="PiUserDuotone" size={16} />
                            <input
                                name="name"
                                required
                                minlength="3"
                                maxlength="30"
                                value={initial?.name ?? ''}
                            >
                        </span>
                    </label>
                    <div class="connection">
                        <label>
                            <span class="field-caption"
                                >{t('common.field.address')} <span class="required">*</span></span
                            >
                            <span class="node-input">
                                <Icon name="TbWorld" size={16} />
                                <input
                                    name="address"
                                    placeholder={t('base-node-form.e-g-example-com')}
                                    required
                                    minlength="2"
                                    value={initial?.address ?? ''}
                                >
                            </span>
                        </label>
                        <label>
                            <span class="field-caption"
                                >Node Port <span class="required">*</span></span
                            >
                            <input
                                name="port"
                                class="node-port"
                                type="number"
                                required
                                min="1"
                                max="65535"
                                placeholder="2222"
                                bind:value={ports}
                            >
                        </label>
                    </div>
                    {#if mode === 'create'}
                        <NodeInstall port={ports} />
                    {:else if mode === 'edit'}
                        <div class="secret-key">
                            <label class="field-caption" for={secretKeyId}
                                >Secret Key (SECRET_KEY)</label
                            >
                            <span class="node-input secret-input">
                                <Icon name="TbCertificate" size={16} />
                                <input
                                    id={secretKeyId}
                                    readonly
                                    value={secret.data?.secretKey ?? 'Error loading...'}
                                    spellcheck="false"
                                >
                                <span class="secret-copy">
                                    <ActionIcon
                                        icon="PiCopy"
                                        label={t('common.action.copy')}
                                        subtle
                                        size={28}
                                        iconSize={16}
                                        disabled={!secret.data}
                                        onclick={() => {
                                            const value = secret.data?.secretKey
                                            if (value) void navigator.clipboard.writeText(value)
                                        }}
                                    />
                                </span>
                            </span>
                        </div>
                    {/if}
                    {@render entry('activePluginUuid', t('node-vitals.card.plugin'), pluginField)}
                    <label>
                        {t('node-vitals.card.proxy-url')}
                        <small class="proxy-description"
                            >{t('node-vitals.card.proxy-url-description')}</small
                        >
                        <span class="node-input">
                            <Icon name="TbNetwork" size={16} />
                            <input
                                name="proxyUrl"
                                placeholder="socks5://user:pass@address:port"
                                value={initial?.proxyUrl ?? ''}
                                autocomplete="off"
                                spellcheck="false"
                            >
                        </span>
                    </label>
                </FormCard>
                <FormCard
                    class="consumption"
                    title={t('base-node-form.consumption')}
                    icon="PiChartBarDuotone"
                    tone="violet"
                >
                    {@render multipliers()}
                </FormCard>
            </div>

            <div class="column end">
                {#if system && node?.system}
                    <div class="system">{@render system()}</div>
                {/if}
                <FormCard
                    class="configuration"
                    title={t('base-node-form.core-configuration')}
                    icon="SiSecurityscorecard"
                    tone="teal"
                >
                    {#key profileVersion}
                        <ProfileFields current={initial?.configProfile} onchange={markProfile} />
                    {/key}
                </FormCard>
                <FormCard
                    class="addresses"
                    title={t('common.field.ip-addresses')}
                    icon="TbNetwork"
                    tone="cyan"
                >
                    {#snippet actions()}
                        <ActionIcon
                            icon="TbPlus"
                            label={t('node-ips.add-ip')}
                            tone="cyan"
                            size={34}
                            iconSize={18}
                            disabled={ips.length >= 64}
                            onclick={addIp}
                        />
                    {/snippet}
                    {#if !ips.length}
                        <div class="empty-ips">
                            <span><Icon name="TbNetwork" size={24} /></span>
                            {t('node-ips.no-ips-yet')}
                        </div>
                    {/if}
                    {#each ips as item, index (item.key)}
                        <div class="ip">
                            <label
                                >{t('common.field.ip-address')}
                                <input bind:value={item.ip} name="ips"></label
                            >
                            <label
                                >{t('common.field.status')}
                                <select bind:value={item.status} name="ips">
                                    {#each Object.values(NODE_IP_STATUSES) as status (status)}
                                        <option value={status}>{status}</option>
                                    {/each}
                                </select>
                            </label>
                            <button
                                type="button"
                                onclick={() => {
                                    ips.splice(index, 1)
                                    changed.add('ips')
                                }}
                            >
                                {t('common.action.remove')}
                            </button>
                        </div>
                    {/each}
                </FormCard>
                <FormCard
                    class="billing"
                    title={t('base-node-form.tracking-and-billing')}
                    icon="PiChartBarDuotone"
                    tone="yellow"
                >
                    {@render entry(
                        'providerUuid',
                        t('select-infra-provider.shared.infrastructure-provider'),
                        providerField
                    )}
                    <div class="tracking">
                        <span
                            ><Icon name="PiChartBarDuotone" size={20} />
                            {t('base-node-form.traffic-tracking')}</span
                        >
                        <Switch
                            name="isTrafficTrackingActive"
                            label={t('base-node-form.traffic-tracking')}
                            bind:checked={tracking}
                        />
                    </div>
                    {#if tracking}
                        {@render numeric(
                            'trafficLimitBytes',
                            'Traffic limit (GiB)',
                            initial?.trafficLimitBytes != null
                                ? initial.trafficLimitBytes / gibibyte
                                : undefined,
                            undefined,
                            'any'
                        )}
                        {@render numeric(
                            'notifyPercent',
                            'Notify at (%)',
                            initial?.notifyPercent ?? undefined,
                            100
                        )}
                        {@render numeric(
                            'trafficResetDay',
                            'Monthly reset day',
                            initial?.trafficResetDay ?? undefined,
                            31,
                            1,
                            1
                        )}
                        {#if node}
                            <SelectField
                                name="trafficTrackingStart"
                                label={t('base-node-form.tracking-start.label')}
                                placeholder={t('base-node-form.tracking-start.label')}
                                value={trackingStart}
                                options={TRAFFIC_TRACKING_START.map((strategy) => ({
                                    value: strategy,
                                    label: t(`base-node-form.tracking-start.${strategy}`)
                                }))}
                                searchable={false}
                                clearable={false}
                                onchoose={(value) => {
                                    trackingStartOverride = value
                                    changed.add('trafficTrackingStart')
                                }}
                            />
                        {/if}
                    {/if}
                    {@render input('tags', t('common.field.tags'), initial?.tags.join(', ') ?? '')}
                    {@render input(
                        'note',
                        t('node-tracking-and-billing.card.note'),
                        initial?.note ?? '',
                        255
                    )}
                </FormCard>
            </div>
        {/if}
    </fieldset>
    {#if problem || save.error}
        <p role="alert">{problem || save.error?.message}</p>
    {/if}
    {#if notice}
        <p role="status">{notice}</p>
    {/if}
    <div class="footer">
        {@render actions?.()}
        <Button
            type="submit"
            size="md"
            color="teal"
            variant="soft"
            icon="PiFloppyDiskDuotone"
            disabled={save.isPending || (mode === 'edit' && changed.size === 0)}
        >
            {t('common.action.save')}
        </Button>
    </div>
</form>

{#snippet multipliers()}
    {@render numeric(
        'consumptionMultiplier',
        t('node-consumption.card.user-consumption-multiplier'),
        initial?.consumptionMultiplier,
        100,
        0.1
    )}
    {@render numeric(
        'nodeConsumptionMultiplier',
        t('node-consumption.card.node-consumption-multiplier'),
        initial?.nodeConsumptionMultiplier,
        100,
        0.1
    )}
{/snippet}

{#snippet providerField()}
    <NodeProviderField
        providers={providers.data?.providers ?? []}
        value={providerUuid}
        onchoose={(value) => selectAssociation('providerUuid', value)}
    />
    {#if providers.error}
        <p role="alert">{providers.error.message}</p>
    {/if}
{/snippet}
{#snippet pluginField()}
    <SelectField
        name="activePluginUuid"
        disabled={false}
        label={t('node-vitals.card.plugin')}
        description={t('node-vitals.card.review-documentation-for-more-information')}
        placeholder={t('node-vitals.card.select-plugin')}
        emptyText={t('node-vitals.card.nothing-found')}
        leading="TbPackage"
        options={choicesWithSelection(plugins.data?.nodePlugins ?? [], initial?.activePluginUuid)}
        value={activePluginUuid}
        onchoose={(value) => selectAssociation('activePluginUuid', value)}
    />
    {#if plugins.error}
        <p role="alert">{plugins.error.message}</p>
    {/if}
{/snippet}
{#snippet entry(key: NodeField, label: string, control: Snippet)}
    {#if ids}
        <label class="check"
            ><input
                type="checkbox"
                checked={enabled.has(key)}
                onchange={(event) => setApplied(key, event.currentTarget.checked)}
            > {label}</label
        >
        {#if enabled.has(key)}
            {@render control()}
        {/if}
    {:else}
        {@render control()}
    {/if}
{/snippet}

{#snippet input(key: NodeField, label: string, value: string, maxlength?: number)}
    {#snippet field()}
        <label>{label} <input name={key} {value} {maxlength}></label>
    {/snippet}
    {@render entry(key, label, field)}
{/snippet}

{#snippet numeric(
    key: NodeField,
    label: string,
    value: number | undefined,
    max?: number,
    step: number | 'any' = 1,
    min = 0
)}
    {#snippet field()}
        <label
            >{label}
            <input name={key} type="number" {value} {min} {max} {step} required={!!ids}></label
        >
    {/snippet}
    {@render entry(key, label, field)}
{/snippet}

<style>
    form,
    .form,
    .column,
    .details,
    label {
        display: grid;
        gap: var(--mantine-spacing-md);
        min-width: 0;
    }
    .form {
        border: 0;
        margin: 0;
        padding: 0;
    }
    .column {
        align-content: start;
    }
    .single .form {
        grid-template-columns: repeat(2, minmax(0, 1fr));
    }
    label {
        gap: 0;
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
    }
    .field-caption {
        min-height: calc(1.55rem * var(--mantine-scale));
    }
    .node-input input {
        height: 36px;
    }
    .proxy-description {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        font-weight: 400;
    }
    .required {
        color: var(--mantine-color-error);
    }
    .node-input {
        position: relative;
        display: grid;
        min-width: 0;
    }
    .node-input > :global(svg) {
        position: absolute;
        inset-inline-start: 10px;
        inset-block-start: 10px;
        color: var(--mantine-color-dimmed);
        pointer-events: none;
    }
    .node-input :is(input, select) {
        padding-inline-start: 36px;
    }
    .secret-key {
        display: grid;
        gap: 0;
        min-width: 0;
    }
    .secret-input input {
        overflow: hidden;
        padding-inline-end: 36px;
        font-family: var(--mantine-font-family-monospace);
        text-overflow: clip;
        white-space: nowrap;
        cursor: copy;
    }
    .secret-input::after {
        position: absolute;
        inset-block: 1px;
        inset-inline-end: 10px;
        width: 80px;
        background: linear-gradient(to right, transparent, var(--mantine-color-default) 70%);
        content: "";
        pointer-events: none;
        z-index: 1;
    }
    .secret-copy {
        position: absolute;
        inset-block: 0;
        inset-inline-end: 6px;
        display: grid;
        place-items: center;
        z-index: 10;
    }
    .secret-copy :global(svg) {
        position: static;
        pointer-events: auto;
    }
    .connection {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        align-items: end;
        gap: var(--mantine-spacing-xs);
    }
    .node-port {
        appearance: textfield;
    }
    .node-port::-webkit-inner-spin-button,
    .node-port::-webkit-outer-spin-button {
        appearance: none;
    }
    .connection input {
        height: 36px;
        min-width: 0;
    }
    .check,
    .tracking,
    .tracking > span,
    .ip {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
    }
    .tracking {
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .tracking {
        justify-content: space-between;
        padding: var(--mantine-spacing-xs) var(--mantine-spacing-sm);
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
    }
    .tracking > span :global(svg) {
        color: var(--mantine-color-indigo-4);
    }
    .empty-ips {
        padding: var(--mantine-spacing-xs) 0;
        display: grid;
        justify-items: center;
        gap: var(--mantine-spacing-xs);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
        font-weight: 600;
    }
    .empty-ips span {
        width: 48px;
        height: 48px;
        display: grid;
        place-items: center;
        border-radius: 50%;
        border: 1px solid color-mix(in srgb, var(--mantine-color-gray-6) 30%, transparent);
        background: transparent;
        color: var(--mantine-color-gray-4);
    }
    .ip {
        flex-wrap: wrap;
        align-items: end;
    }
    .footer {
        display: flex;
        gap: var(--mantine-spacing-md);
        justify-content: end;
    }
    .single .footer {
        position: sticky;
        bottom: -1px;
        z-index: 1;
        margin: 0 calc(-1 * var(--mantine-spacing-sm)) calc(-1 * var(--mantine-spacing-sm));
        padding: var(--mantine-spacing-sm);
        border-block-start: 1px solid rgba(var(--gw-fg-rgb), 0.06);
        background: rgba(var(--gw-surface-rgb), 0.95);
    }
    @media (max-width: 64rem) {
        .single .form {
            display: flex;
            flex-direction: column;
            gap: var(--mantine-spacing-lg);
        }
        .column {
            display: contents;
        }
        .details {
            gap: var(--mantine-spacing-lg);
        }
        .details,
        .system {
            order: 0;
        }
        .form :global(.vitals) {
            order: 1;
        }
        .form :global(.addresses) {
            order: 2;
        }
        .form :global(.configuration) {
            order: 3;
        }
        .form :global(.billing) {
            order: 4;
        }
        .form :global(.consumption) {
            order: 5;
        }
        .single .footer {
            flex-wrap: wrap;
        }
        .single .footer > :global(*) {
            flex: 1 1 auto;
            min-width: 0;
        }
    }
</style>
