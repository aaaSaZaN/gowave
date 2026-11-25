<script lang="ts">
    import {
        SetExternalSquadTagsCommand,
        SUBSCRIPTION_TEMPLATE_TYPE,
        UpdateExternalSquadCommand
    } from '@remnawave/backend-contract'
    import { createQuery } from '@tanstack/svelte-query'
    import { onDestroy, type Snippet } from 'svelte'
    import { submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import HwidFields from '../subscriptions/HwidFields.svelte'
    import { removedHeaderNames } from '../subscriptions/headers'
    import {
        subpageConfigsQuery,
        subscriptionSettingsQuery,
        subscriptionTemplatesQuery
    } from '../subscriptions/queries'
    import RemarksFields from '../subscriptions/RemarksFields.svelte'
    import ResponseHeaderFields from '../subscriptions/ResponseHeaderFields.svelte'
    import {
        readHeaders,
        readHwidSettings,
        readRemarks,
        subscriptionOptions,
        textValue
    } from '../subscriptions/settings-fields'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import FormCard from '../ui/FormCard.svelte'
    import HelpDrawer from '../ui/HelpDrawer.svelte'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import Switch from '../ui/Switch.svelte'
    import { type ExternalSquad, externalSquadQuery, externalSquadsQuery } from './queries'

    const tabs = [
        ['templates', 'external-squads.drawer.widget.templates', 'TbFolder'],
        ['overrides', 'external-squads.drawer.widget.settings', 'TbSettings'],
        ['hosts', 'constants.hosts', 'PiListChecks'],
        ['headers', 'external-squads-response-headers.widget.response-headers', 'TbPrescription'],
        ['hwid', 'HWID', 'PiIdentificationBadge'],
        ['remarks', 'external-squads.drawer.widget.remarks', 'TbListLetters'],
        ['page', 'constants.subscription-page', 'TbPalette']
    ] as const
    type Section = (typeof tabs)[number][0]
    type SubscriptionOverride = keyof typeof subscriptionOptions
    type HostOverride = 'serverDescription' | 'vlessRouteId'

    const types = SUBSCRIPTION_TEMPLATE_TYPE
    const templateFields = [
        [types.XRAY_JSON, 'Xray JSON', 'XrayLogo'],
        [types.MIHOMO, 'Mihomo', 'MihomoLogo'],
        [types.STASH, 'Stash', 'StashLogo'],
        [types.SINGBOX, 'Sing-box', 'SingboxLogo'],
        [types.CLASH, 'Clash', 'MihomoLogo']
    ] as const
    const hostOptions = {
        serverDescription: 'base-host-form.server-description-header',
        vlessRouteId: 'base-host-form.vless-route-id'
    } as const
    const subscriptionKeys = Object.keys(subscriptionOptions) as SubscriptionOverride[]
    const hostKeys = Object.keys(hostOptions) as HostOverride[]
    const tabsId = $props.id()

    let { squad, editIdentity }: { squad: ExternalSquad; editIdentity: boolean } = $props()
    let t = $derived(locale.t)
    let section = $state<Section>('templates')
    let error = $state('')
    let saveError = $state('')
    let saving = $state(false)
    let copied = $state(false)
    let copyTimer: ReturnType<typeof setTimeout> | undefined
    let useDeviceSettings = $state(false)
    let useRemarks = $state(false)
    let headersHelp = $state(false)
    let subscriptionOverrides = $state<SubscriptionOverride[]>([])
    let hostOverrides = $state<HostOverride[]>([])
    const active = $derived(squad.info.membersCount > 0)

    const templates = createQuery(() => ({
        ...subscriptionTemplatesQuery,
        enabled: !editIdentity && section === 'templates'
    }))
    const configs = createQuery(() => ({ ...subpageConfigsQuery, enabled: section === 'page' }))
    const globalSettings = createQuery(() => ({
        ...subscriptionSettingsQuery,
        enabled: section === 'headers'
    }))

    async function save(event: SubmitEvent): Promise<void> {
        event.preventDefault()
        if (saving || !(event.currentTarget instanceof HTMLFormElement)) return
        const data = new FormData(event.currentTarget)
        saving = true
        saveError = ''
        try {
            if (editIdentity) {
                const name = textValue(data, 'name').trim()
                const tags = [
                    ...new Set(
                        textValue(data, 'tags')
                            .toUpperCase()
                            .split(/[\s,]+/)
                            .filter(Boolean)
                    )
                ]
                const tagBody = SetExternalSquadTagsCommand.RequestBodySchema.parse({
                    uuid: squad.uuid,
                    tags
                })
                let updated = squad
                if (name !== updated.name) {
                    updated = await submitCommand(UpdateExternalSquadCommand, {
                        uuid: updated.uuid,
                        name
                    })
                    cache(updated)
                }
                if (JSON.stringify(tags) !== JSON.stringify(updated.tags)) {
                    const result = await submitCommand(SetExternalSquadTagsCommand, tagBody)
                    cache({ ...updated, tags: result.tags })
                }
            } else {
                const patch = changes(data)
                cache(
                    await submitCommand(UpdateExternalSquadCommand, { uuid: squad.uuid, ...patch })
                )
            }
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
        } catch (cause) {
            saveError = cause instanceof Error ? cause.message : String(cause)
        } finally {
            saving = false
        }
    }

    function cache(updated: ExternalSquad): void {
        queryClient.setQueryData(externalSquadQuery(updated.uuid).queryKey, updated)
        void queryClient.invalidateQueries({ queryKey: externalSquadsQuery.queryKey })
    }

    function choicesWithSelection(
        rows: readonly { uuid: string; name: string }[],
        selected: string | null | undefined
    ): { value: string; label: string }[] {
        const options = rows.map(({ uuid, name }) => ({ value: uuid, label: name }))
        if (selected && !options.some(({ value }) => value === selected)) {
            options.unshift({ value: selected, label: selected })
        }
        return options
    }

    function changes(data: FormData): Omit<UpdateExternalSquadCommand.RequestBody, 'uuid'> {
        switch (section) {
            case 'templates':
                return {
                    templates: [
                        ...squad.templates.filter(
                            ({ templateType }) => templateType === types.XRAY_BASE64
                        ),
                        ...templateFields.flatMap(([templateType]) => {
                            const templateUuid = textValue(data, templateType)
                            return templateUuid ? [{ templateType, templateUuid }] : []
                        })
                    ]
                }
            case 'overrides':
                return {
                    subscriptionSettings: Object.fromEntries(
                        subscriptionOverrides.map((key) => [key, data.has(key)])
                    )
                }
            case 'hosts':
                return {
                    hostOverrides: {
                        serverDescription: hostOverrides.includes('serverDescription')
                            ? textValue(data, 'description').trim() || null
                            : undefined,
                        vlessRouteId: hostOverrides.includes('vlessRouteId')
                            ? textValue(data, 'routeId') === ''
                                ? null
                                : Number(textValue(data, 'routeId'))
                            : undefined
                    }
                }
            case 'headers':
                return {
                    responseHeadersAdd: readHeaders(data),
                    responseHeadersRemove: removedHeaderNames(textValue(data, 'removeHeaders'))
                }
            case 'hwid':
                return { hwidSettings: useDeviceSettings ? readHwidSettings(data) : null }
            case 'remarks':
                return { customRemarks: useRemarks ? readRemarks(data) : null }
            case 'page':
                return { subpageConfigUuid: textValue(data, 'subpageConfigUuid') || null }
        }
    }

    function open(next: Section): void {
        if (section === next || saving) return
        error = ''
        saveError = ''
        useDeviceSettings = squad.hwidSettings !== null
        useRemarks = squad.customRemarks !== null
        subscriptionOverrides = subscriptionKeys.filter(
            (key) => squad.subscriptionSettings?.[key] !== undefined
        )
        hostOverrides = hostKeys.filter((key) => squad.hostOverrides?.[key] !== undefined)
        section = next
    }

    function moveTab(event: KeyboardEvent, index: number): void {
        const direction = document.documentElement.dir === 'rtl' ? -1 : 1
        const last = tabs.length - 1
        const next = {
            ArrowLeft: (index - direction + tabs.length) % tabs.length,
            ArrowRight: (index + direction + tabs.length) % tabs.length,
            Home: 0,
            End: last
        }[event.key]
        if (next === undefined || saving) return
        const tab = tabs[next]
        if (!tab) return
        event.preventDefault()
        open(tab[0])
        document.getElementById(`${tabsId}-${tab[0]}`)?.focus()
    }

    function copyUuid(): void {
        error = ''
        void navigator.clipboard.writeText(squad.uuid).then(
            () => {
                copied = true
                clearTimeout(copyTimer)
                copyTimer = setTimeout(() => (copied = false), 2000)
            },
            (reason: unknown) => (error = reason instanceof Error ? reason.message : String(reason))
        )
    }

    onDestroy(() => clearTimeout(copyTimer))
</script>

<section class="squad-card summary">
    <span class="squad-icon" class:active><Icon name="TbWebhook" size={28} /></span>
    <div class="identity">
        <h3 title={squad.name}>{squad.name}</h3>
        <div class="counts">
            <span class="member-count" class:active title={t('external-squad-card.widget.users')}>
                <Icon name="PiUsers" size={12} />{squad.info.membersCount.toLocaleString('en-US')}
            </span>
            <ActionIcon
                icon={copied ? 'PiCheck' : 'PiCopy'}
                {...copied ? { tone: 'teal' } : {}}
                size={34}
                iconSize={18}
                subtle
                label={t('common.action.copy-uuid')}
                onclick={copyUuid}
            />
        </div>
    </div>
</section>

{#if !editIdentity}
    <div
        role="tablist"
        class="squad-tabs"
        aria-label={t('external-squads.drawer.widget.edit-external-squad')}
    >
        {#each tabs as [id, label, icon], index (id)}
            <button
                type="button"
                class="m_4ec4dce6 mantine-UnstyledButton-root squad-tab"
                role="tab"
                id={`${tabsId}-${id}`}
                disabled={saving}
                aria-selected={section === id}
                aria-controls={`${tabsId}-panel-${id}`}
                tabindex={section === id ? 0 : -1}
                onclick={() => open(id)}
                onkeydown={(event) => moveTab(event, index)}
            >
                <Icon name={icon} size="1.2rem" />{label === 'HWID' ? label : t(label)}
            </button>
        {/each}
    </div>
    {#each tabs as [id] (id)}
        <div
            role="tabpanel"
            id={`${tabsId}-panel-${id}`}
            aria-labelledby={`${tabsId}-${id}`}
            hidden={id !== section}
        >
            {#if id === section}
                {@render editor()}
            {/if}
        </div>
    {/each}
{:else}
    {@render editor()}
{/if}

{#snippet editor()}
    <form
        class="squad-card editor"
        class:headers={section === 'headers'}
        class:compact-save={section !== 'templates' || editIdentity}
        onsubmit={save}
    >
        <fieldset disabled={saving}>
            {#if editIdentity}
                <label>
                    {t('common.field.name')}
                    <input name="name" value={squad.name} required minlength="2" maxlength="30">
                </label>
                <label>
                    {t('common.field.tags')}
                    <input name="tags" value={squad.tags.join(', ')} spellcheck="false">
                </label>
            {:else if section === 'templates'}
                <h3>{t('external-squads.drawer.widget.subscription-templates')}</h3>
                <p class="description">
                    {t('external-squads.drawer.widget.subscription-templates-description-line-1')}
                    <br>
                    {t('external-squads.drawer.widget.subscription-templates-description-line-2')}
                </p>
                {#if templates.error}
                    <p role="alert">{templates.error.message}</p>
                {:else if templates.isPending}
                    <RouteLoading compact />
                {:else}
                    <div class="template-fields">
                        {#each templateFields as [type, label, logo] (type)}
                            {@const selected = squad.templates.find(
                                ({ templateType }) => templateType === type
                            )?.templateUuid}
                            {@const choices = templates.data.templates.filter(
                                (template) =>
                                    template.templateType === type &&
                                    (template.name.toLowerCase() !== 'default' ||
                                        template.uuid === selected)
                            )}
                            <SelectField
                                name={type}
                                {label}
                                leading={logo}
                                options={choicesWithSelection(choices, selected)}
                                value={selected ?? ''}
                                placeholder={t(
                                    choices.length
                                        ? 'external-squads.drawer.widget.select-a-template'
                                        : 'external-squads.drawer.widget.default-template-in-use'
                                )}
                                searchable={false}
                            />
                        {/each}
                    </div>
                {/if}
            {:else if section === 'overrides'}
                <h3>{t('subscription-settings.widget.main-settings')}</h3>
                <p class="description">
                    {t(
                        'external-squads-settings.tab.widget.override-subscription-settings-for-this-external-squad'
                    )}
                </p>
                {#each subscriptionOverrides as key (key)}
                    <div class="override">
                        <Switch
                            label={t(subscriptionOptions[key])}
                            showLabel
                            name={key}
                            checked={squad.subscriptionSettings?.[key] ?? false}
                        />
                        <ActionIcon
                            icon="TbTrash"
                            tone="red"
                            size={36}
                            iconSize={20}
                            subtle
                            label={t('common.action.delete')}
                            onclick={() =>
                                (subscriptionOverrides = subscriptionOverrides.filter(
                                    (item) => item !== key
                                ))}
                        />
                    </div>
                {/each}
                {@render addOverride(
                    subscriptionKeys.filter((key) => !subscriptionOverrides.includes(key)),
                    (key) => t(subscriptionOptions[key as SubscriptionOverride]),
                    (key) =>
                        (subscriptionOverrides = [
                            ...subscriptionOverrides,
                            key as SubscriptionOverride
                        ])
                )}
            {:else if section === 'hosts'}
                <h3>{t('external-squads-hosts.tab.widget.host-overrides')}</h3>
                <p class="description">
                    {t(
                        'external-squads-hosts.tab.widget.override-host-settings-for-this-external-squad'
                    )}
                </p>
                {#each hostOverrides as key (key)}
                    <div class="override">
                        <label for={`${tabsId}-override-${key}`}>
                            {t(hostOptions[key])}
                            {#if key === 'serverDescription'}
                                <input
                                    id={`${tabsId}-override-${key}`}
                                    name="description"
                                    maxlength="30"
                                    value={squad.hostOverrides?.serverDescription ?? ''}
                                >
                            {:else}
                                <input
                                    id={`${tabsId}-override-${key}`}
                                    name="routeId"
                                    type="number"
                                    min="0"
                                    max="65535"
                                    step="1"
                                    value={squad.hostOverrides?.vlessRouteId ?? ''}
                                >
                            {/if}
                        </label>
                        <ActionIcon
                            icon="TbTrash"
                            tone="red"
                            size={36}
                            iconSize={20}
                            subtle
                            label={t('common.action.delete')}
                            onclick={() =>
                                (hostOverrides = hostOverrides.filter((item) => item !== key))}
                        />
                    </div>
                {/each}
                {@render addOverride(
                    hostKeys.filter((key) => !hostOverrides.includes(key)),
                    (key) => t(hostOptions[key as HostOverride]),
                    (key) => (hostOverrides = [...hostOverrides, key as HostOverride])
                )}
            {:else if section === 'headers'}
                <FormCard
                    title={t('external-squads-response-headers.widget.response-headers')}
                    icon="TbPrescription"
                    tone="cyan"
                >
                    {#snippet actions()}
                        <ActionIcon
                            icon="TbQuestionMark"
                            tone="lime"
                            label={t('help-action-icon.shared.help-article')}
                            onclick={() => (headersHelp = true)}
                        />
                        {@render saveButton(false)}
                    {/snippet}
                    <p class="description">
                        {t('external-squads-response-headers.widget.headers-inherited-description')}
                    </p>
                    <h4>{t('external-squads-response-headers.widget.add-headers')}</h4>
                    <p class="description">
                        {t('external-squads-response-headers.widget.add-headers-description')}
                    </p>
                    <ResponseHeaderFields compact headers={squad.responseHeadersAdd}>
                        {#snippet children(fields, addHeader)}
                            <div class="response-fields">{@render fields()}</div>
                            <div class="add-header">
                                <Button variant="soft" icon="TbPlus" onclick={addHeader}>
                                    {t('headers-manager.widget.add-header')}
                                </Button>
                            </div>
                        {/snippet}
                    </ResponseHeaderFields>
                    <h4>{t('external-squads-response-headers.widget.remove-headers')}</h4>
                    <p class="description">
                        {t('external-squads-response-headers.widget.remove-headers-description')}
                    </p>
                    <input
                        name="removeHeaders"
                        aria-label={t('external-squads-response-headers.widget.remove-headers')}
                        placeholder={t('external-squads-response-headers.widget.enter-header-key')}
                        value={squad.responseHeadersRemove.join(', ')}
                        spellcheck="false"
                    >
                    {#if globalSettings.data?.customResponseHeaders &&
                        Object.keys(globalSettings.data.customResponseHeaders).length}
                        <p class="description">
                            {Object.keys(globalSettings.data.customResponseHeaders).join(', ')}
                        </p>
                    {/if}
                </FormCard>
            {:else if section === 'hwid'}
                <div class="override-intro">
                    <h3>{t('external-squads-hwid-settings.tab.widget.hwid-settings-override')}</h3>
                    <p class="description">
                        {t('external-squads-hwid-settings.tab.widget.override-hwid-description')}
                    </p>
                </div>
                {#snippet hwidToggle()}
                    <Switch
                        label={t('external-squads-hwid-settings.tab.widget.enable-hwid-override')}
                        tone="cyan"
                        bind:checked={useDeviceSettings}
                    />
                {/snippet}
                {@render toggle(
                    t('external-squads-hwid-settings.tab.widget.enable-hwid-override'),
                    hwidToggle
                )}
                {#if useDeviceSettings}
                    <div class="hwid-fields">
                        <HwidFields value={squad.hwidSettings} defaultLimit={999} />
                    </div>
                {/if}
            {:else if section === 'remarks'}
                <div class="override-intro">
                    <h3>
                        <Icon name="TbListLetters" size={20} />
                        {t('subscription-settings.widget.custom-remarks')}
                    </h3>
                    <p class="description">
                        {t('external-squads-custom-remarks.widget.override-remarks-description')}
                    </p>
                </div>
                {#snippet remarksToggle()}
                    <Switch
                        label={t('external-squads-custom-remarks.widget.enable-override')}
                        tone="cyan"
                        bind:checked={useRemarks}
                    />
                {/snippet}
                {@render toggle(
                    t('external-squads-custom-remarks.widget.enable-override'),
                    remarksToggle
                )}
                {#if useRemarks}
                    <RemarksFields remarks={squad.customRemarks} />
                {/if}
            {:else if section === 'page'}
                <h3>{t('external-squads-subpage-config.tab.widget.subscription-page-config')}</h3>
                <p class="description">
                    {t(
                        'external-squads-subpage-config.tab.widget.subscription-page-config-description'
                    )}
                </p>
                {#if configs.error}
                    <p role="alert">{configs.error.message}</p>
                {:else if configs.isPending}
                    <RouteLoading compact />
                {:else}
                    <SelectField
                        name="subpageConfigUuid"
                        label={t('external-squads-subpage-config.tab.widget.subpage-config')}
                        leading="TbPalette"
                        options={choicesWithSelection(
                            configs.data.configs,
                            squad.subpageConfigUuid
                        )}
                        value={squad.subpageConfigUuid ?? ''}
                        placeholder={t('external-squads-subpage-config.tab.widget.select-config')}
                        searchable={false}
                    />
                {/if}
            {/if}
        </fieldset>
        {#if section !== 'headers'}
            {@render saveButton(true)}
        {/if}
    </form>
{/snippet}

{#snippet saveButton(block: boolean)}
    <Button
        type="submit"
        variant="soft"
        color="teal"
        size="md"
        {block}
        icon="TbDeviceFloppy"
        iconSize={19.2}
        disabled={saving ||
            (!editIdentity && section === 'templates' && !templates.isSuccess) ||
            (section === 'page' && !configs.isSuccess)}
    >
        {t('common.action.save')}
    </Button>
{/snippet}

{#snippet toggle(label: string, control: Snippet)}
    <div class="override toggle">
        <span>{label}</span>
        {@render control()}
    </div>
{/snippet}

{#snippet addOverride(keys: string[], label: (key: string) => string, add: (key: string) => void)}
    {#if keys.length}
        {@const placeholder = t('external-squads-settings.tab.widget.add-override')}
        <SelectField
            label={placeholder}
            {placeholder}
            leading="TbPlus"
            options={keys.map((key) => ({ value: key, label: label(key) }))}
            searchable={false}
            hideLabel
            onchoose={add}
        />
    {/if}
{/snippet}

{#if headersHelp}
    <HelpDrawer screen="PAGE_RESPONSE_HEADERS" close={() => (headersHelp = false)} />
{/if}
{#if saveError}
    <p role="alert">{saveError}</p>
{/if}
{#if error}
    <p role="alert">{error}</p>
{/if}

<style>
    .squad-card {
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .summary {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-md);
        margin-block-end: var(--mantine-spacing-md);
    }
    .squad-icon {
        display: grid;
        place-items: center;
        flex: 0 0 44px;
        height: 44px;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-dark-6);
        color: var(--mantine-color-gray-light-color);
        box-shadow: 0 4px 6px -1px rgb(0 0 0 / 20%);
    }
    .squad-icon.active,
    .member-count.active {
        background: var(--mantine-color-teal-light);
        color: var(--mantine-color-teal-light-color);
    }
    .identity {
        min-width: 0;
    }
    .identity h3 {
        overflow: hidden;
        margin: 0 0 6px;
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-lg);
        line-height: var(--mantine-line-height-lg);
        white-space: nowrap;
        text-overflow: ellipsis;
    }
    .counts,
    .member-count {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .member-count {
        gap: 6px;
        height: 26px;
        padding-inline: 13px;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-gray-light);
        color: var(--mantine-color-gray-light-color);
        font-size: var(--mantine-font-size-xs);
        font-weight: 700;
    }
    .squad-tabs {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
        margin-block-end: var(--mantine-spacing-xl);
    }
    .squad-tab {
        gap: var(--mantine-spacing-xs);
        background: var(--gw-raised-bg);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        font-weight: 500;
    }
    .squad-tab:hover {
        background: var(--mantine-color-default-hover);
        border-color: var(--mantine-color-default-border);
    }
    .squad-tab[aria-selected="true"] {
        color: var(--mantine-primary-color-4);
        background: color-mix(in srgb, var(--mantine-primary-color-6) 12%, transparent);
        border-color: color-mix(in srgb, var(--mantine-primary-color-6) 30%, transparent);
    }
    .editor,
    fieldset,
    label,
    .template-fields {
        display: grid;
    }
    .editor {
        gap: var(--mantine-spacing-xl);
    }
    .compact-save {
        gap: var(--mantine-spacing-md);
    }
    .headers {
        padding: 0;
        border: 0;
        background: none;
    }
    .headers h4 {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        margin: 0;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        font-weight: 400;
    }
    .headers h4::after {
        height: 1px;
        flex: 1;
        background: var(--mantine-color-default-border);
        content: "";
    }
    .response-fields {
        display: grid;
        gap: var(--mantine-spacing-xs);
    }
    .add-header {
        justify-self: end;
    }
    fieldset {
        min-width: 0;
        gap: var(--mantine-spacing-md);
        margin: 0;
        padding: 0;
        border: 0;
    }
    .editor h3,
    .editor p {
        margin: 0;
    }
    .editor h3 {
        font-size: var(--mantine-font-size-md);
        font-weight: 600;
        line-height: var(--mantine-line-height-md);
    }
    .description {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
    }
    .template-fields {
        gap: var(--mantine-spacing-sm);
    }
    label {
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        line-height: var(--mantine-line-height-sm);
    }
    .override {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        justify-content: space-between;
        padding: var(--mantine-spacing-sm);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-dark-7);
    }
    .override > label {
        flex: 1;
    }
    .override-intro {
        display: grid;
        gap: 4px;
    }
    .override-intro h3 {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .toggle,
    .hwid-fields > :global(div) {
        padding: var(--mantine-spacing-md);
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
    }
    .hwid-fields {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .hwid-fields > :global(div) {
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-dark-7);
    }
</style>
