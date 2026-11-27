<script lang="ts">
    import {
        ALPN,
        FINGERPRINTS,
        INTERNAL_SQUADS_MODE,
        MIHOMO_IP_VERSION,
        SECURITY_LAYERS,
        SUBSCRIPTION_TEMPLATE_TYPE
    } from '@remnawave/backend-contract'
    import { createQuery } from '@tanstack/svelte-query'
    import { type Snippet, untrack } from 'svelte'
    import { SvelteMap, SvelteSet } from 'svelte/reactivity'
    import { type Profile, profilesQuery } from '../config-profiles/queries'
    import { locale } from '../i18n.svelte'
    import { squadsQuery } from '../internal-squads/queries'
    import { flag } from '../nodes/analytics'
    import { nodesQuery } from '../nodes/queries'
    import RouteLoading from '../RouteLoading.svelte'
    import { subscriptionSettingsQuery, subscriptionTemplatesQuery } from '../subscriptions/queries'
    import TemplateVariables from '../subscriptions/TemplateVariables.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import { nextComboboxOption } from '../ui/combobox'
    import Icon from '../ui/Icon.svelte'
    import MultiSelect from '../ui/MultiSelect.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import Switch from '../ui/Switch.svelte'
    import TablePopup from '../ui/TablePopup.svelte'
    import Tooltip from '../ui/Tooltip.svelte'
    import HostInboundPicker from './HostInboundPicker.svelte'
    import HostJsonInput from './HostJsonInput.svelte'
    import HostTags from './HostTags.svelte'
    import { type HostField, jsonOptions, switches, textOptions } from './host-fields'
    import { type Host, hostTagsQuery } from './queries'

    let {
        host,
        bulk,
        onfieldchange
    }: {
        host: Host | undefined
        bulk: boolean
        onfieldchange: (key: string) => void
    } = $props()

    const profiles = createQuery(() => ({ ...profilesQuery, refetchOnMount: 'always' }))
    const nodes = createQuery(() => nodesQuery)
    const squads = createQuery(() => squadsQuery)
    const templates = createQuery(() => subscriptionTemplatesQuery)
    const settings = createQuery(() => subscriptionSettingsQuery)
    const tags = createQuery(() => hostTagsQuery)

    const applied = new SvelteSet<HostField>()
    const cleared = new SvelteSet<HostField>()
    const selectValues = new SvelteMap<HostField, string>()
    let squadMode = $state(untrack(() => host?.internalSquads.mode ?? INTERNAL_SQUADS_MODE.EXCLUDE))
    let assignedSquads = $state<string[]>(untrack(() => [...(host?.internalSquads.squads ?? [])]))
    let hostNodes = $state<string[]>(untrack(() => [...(host?.nodes ?? [])]))
    let excludedSubscriptions = $state<string[]>(
        untrack(() => [...(host?.excludeFromSubscriptionTypes ?? [])])
    )
    let responseRules = $state<string[]>(untrack(() => [...(host?.includeInSrrRules ?? [])]))
    const assignmentFields = {
        tags: 'common.field.tags',
        nodes: 'constants.nodes',
        internalSquads: 'constants.internal-squads'
    } as const
    const securityFields = {
        sni: textOptions.sni,
        overrideSniFromAddress: switches.overrideSniFromAddress,
        keepSniBlank: switches.keepSniBlank,
        securityLayer: 'base-host-form.security-layer',
        fingerprint: textOptions.fingerprint
    } as const
    const transportFields = {
        vlessRouteId: 'base-host-form.vless-route-id',
        host: textOptions.host,
        path: textOptions.path,
        alpn: 'base-host-form.alpn'
    } as const
    const subscriptionFields = {
        serverDescription: textOptions.serverDescription,
        excludeFromSubscriptionTypes: 'base-host-form.exclude-from-subscription-type',
        includeInSrrRules: 'constants.response-rules',
        isHidden: switches.isHidden,
        shuffleHost: switches.shuffleHost
    } as const
    const advancedFields = {
        xrayJsonTemplateUuid: 'base-host-form.xray-json-template',
        ...jsonOptions
    } as const
    const clientFields = {
        pinnedPeerCertSha256: textOptions.pinnedPeerCertSha256,
        verifyPeerCertByName: textOptions.verifyPeerCertByName,
        mihomoX25519: switches.mihomoX25519,
        mihomoIpVersion: 'base-host-form.mihomo-ip-version'
    } as const
    const subscriptionTypeItems = [
        { value: SUBSCRIPTION_TEMPLATE_TYPE.XRAY_JSON, label: 'Xray JSON', icon: 'XrayLogo' },
        { value: SUBSCRIPTION_TEMPLATE_TYPE.XRAY_BASE64, label: 'Xray Base64', icon: 'XrayLogo' },
        { value: SUBSCRIPTION_TEMPLATE_TYPE.MIHOMO, label: 'Mihomo', icon: 'MihomoLogo' },
        { value: SUBSCRIPTION_TEMPLATE_TYPE.STASH, label: 'Stash', icon: 'StashLogo' },
        { value: SUBSCRIPTION_TEMPLATE_TYPE.SINGBOX, label: 'Singbox', icon: 'SingboxLogo' },
        { value: SUBSCRIPTION_TEMPLATE_TYPE.CLASH, label: 'Clash', icon: 'MihomoLogo' }
    ] as const
    const optionHelpDocs: Partial<Record<HostField, string>> = {
        vlessRouteId: 'https://xtls.github.io/config/routing.html',
        pinnedPeerCertSha256: 'https://xtls.github.io/ru/config/transports/tls.html#tlsobject',
        verifyPeerCertByName: 'https://xtls.github.io/ru/config/transports/tls.html#tlsobject',
        mihomoX25519:
            'https://wiki.metacubex.one/en/config/proxies/tls/#reality-optssupport-x25519mlkem768',
        mihomoIpVersion: 'https://wiki.metacubex.one/ru/config/proxies/#ip-version'
    }
    const core = new Set<HostField>(['remark', 'inbound', 'address', 'port', 'isDisabled'])
    const optional = Object.entries({
        ...assignmentFields,
        ...securityFields,
        ...transportFields,
        ...subscriptionFields,
        ...advancedFields,
        ...clientFields
    }).filter(([key]) => !core.has(key as HostField))
    let addOpen = $state(false)
    let inboundOpen = $state(false)
    let optionSearch = $state('')
    let highlightedOption = $state<string>()
    let hostTags = $state(untrack(() => [...(host?.tags ?? [])]))
    let fingerprint = $state(untrack(() => host?.fingerprint ?? ''))
    let hostEnabled = $state(untrack(() => host?.isDisabled === false))
    const activeOptions = new SvelteSet<HostField>(
        untrack(() => {
            if (!host) return []
            return optional.flatMap(([field]) => {
                const key = field as keyof Host & HostField
                const value = host[key]
                if (key === 'securityLayer' && value === SECURITY_LAYERS.DEFAULT) return []
                if (
                    key === 'internalSquads' &&
                    host.internalSquads.squads.length === 0 &&
                    host.internalSquads.mode === INTERNAL_SQUADS_MODE.EXCLUDE
                )
                    return []
                const present =
                    typeof value === 'object' && value !== null
                        ? Object.keys(value).length > 0
                        : value !== null && value !== undefined && value !== false && value !== ''
                return present ? [key] : []
            })
        })
    )
    const hasOption = (fields: object) =>
        Object.keys(fields).some((key) => activeOptions.has(key as HostField))
    const id = $props.id()
    let t = $derived(locale.t)
    const optionLabels = {
        ...assignmentFields,
        ...securityFields,
        ...transportFields,
        ...subscriptionFields,
        ...advancedFields,
        ...clientFields
    }
    const groups = [
        ['base-host-form.group-assignment', ['tags', 'nodes', 'internalSquads']],
        [
            'base-host-form.group-security',
            ['sni', 'overrideSniFromAddress', 'keepSniBlank', 'securityLayer', 'fingerprint']
        ],
        ['base-host-form.group-transport', ['vlessRouteId', 'host', 'path', 'alpn']],
        [
            'base-host-form.group-subscription',
            [
                'serverDescription',
                'excludeFromSubscriptionTypes',
                'includeInSrrRules',
                'isHidden',
                'shuffleHost'
            ]
        ],
        [
            'base-host-form.group-advanced',
            [
                'xrayJsonTemplateUuid',
                'mapper',
                'muxParams',
                'finalMask',
                'xhttpExtraParams',
                'sockoptParams'
            ]
        ],
        [
            'base-host-form.group-client',
            ['pinnedPeerCertSha256', 'verifyPeerCertByName', 'mihomoX25519', 'mihomoIpVersion']
        ]
    ] as const
    let trigger = $state<HTMLButtonElement>()
    let availableOptions = $derived(
        groups
            .map(([group, fields]) => ({
                group,
                options: fields.flatMap((key) => {
                    if (activeOptions.has(key)) return []
                    const label = optionLabels[key]
                    const title = t(label)
                    return title.toLowerCase().includes(optionSearch.trim().toLowerCase())
                        ? [{ key: key as HostField, title }]
                        : []
                })
            }))
            .filter(({ options }) => options.length)
    )
    function chooseOption(key: HostField): void {
        activeOptions.add(key)
        addOpen = false
        trigger?.focus({ preventScroll: true })
    }

    let port = $state<number | undefined>()
    let inbound = $state<string | undefined>()
    let chosenInbound = $derived(
        inbound ??
            (host?.inbound.configProfileUuid && host?.inbound.configProfileInboundUuid
                ? `${host.inbound.configProfileUuid}/${host.inbound.configProfileInboundUuid}`
                : '')
    )
    const activeProfile = $derived(
        profiles.data?.configProfiles.find(({ uuid }) => uuid === chosenInbound.split('/')[0])
    )
    let knownInbound = $derived.by(() => {
        const [profileId, inboundId] = chosenInbound.split('/')
        return profiles.data?.configProfiles
            .find(({ uuid }) => uuid === profileId)
            ?.inbounds.find(({ uuid }) => uuid === inboundId)
    })

    function apply(key: HostField, enabled: boolean): void {
        if (enabled) applied.add(key)
        else applied.delete(key)
    }

    function optionValue<Key extends HostField & keyof Host>(key: Key): Host[Key] | undefined {
        return cleared.has(key) ? undefined : host?.[key]
    }

    function resetOption(key: HostField): void {
        cleared.add(key)
        selectValues.delete(key)
        if (key === 'tags') hostTags = []
        if (key === 'fingerprint') fingerprint = ''
        if (key === 'nodes') hostNodes = []
        if (key === 'excludeFromSubscriptionTypes') excludedSubscriptions = []
        if (key === 'includeInSrrRules') responseRules = []
        if (key === 'internalSquads') {
            squadMode = INTERNAL_SQUADS_MODE.EXCLUDE
            assignedSquads = []
        }
        activeOptions.delete(key)
        if (!bulk) onfieldchange(key)
        trigger?.focus({ preventScroll: true })
    }

    function changeInbound(profile: Profile, item: Profile['inbounds'][number]): void {
        inbound = `${profile.uuid}/${item.uuid}`
        onfieldchange('inbound')
        if (item.port && !bulk) {
            port = item.port
            onfieldchange('port')
        }
        inboundOpen = false
    }

    function jsonText(value: unknown): string {
        return value === null || value === undefined ? '' : JSON.stringify(value, null, 2)
    }

    function namedOptions(
        items: readonly { uuid: string; name: string }[],
        previous: readonly string[]
    ) {
        const known = new Map(items.map(({ uuid, name }) => [uuid, name]))
        return [
            ...items.map(({ uuid, name }) => ({ value: uuid, label: name })),
            ...previous
                .filter((value) => !known.has(value))
                .map((value) => ({ value, label: value }))
        ]
    }

    function nodeSelectOptions(
        items: readonly {
            uuid: string
            name: string
            countryCode: string
            provider?: { name: string } | null
        }[],
        previous: readonly string[]
    ) {
        const known = new Set(items.map(({ uuid }) => uuid))
        return [
            ...items.map((node) => {
                const emoji = flag(node.countryCode)
                const providerName = node.provider?.name
                return {
                    value: node.uuid,
                    label: `${emoji ? `${emoji} ` : ''}${node.name}${providerName ? ` (${providerName})` : ''}`,
                    name: node.name,
                    flag: emoji || undefined,
                    badge: providerName ?? undefined
                }
            }),
            ...previous
                .filter((value) => !known.has(value))
                .map((value) => ({ value, label: value }))
        ]
    }

    function helpMessage(key: HostField): string | undefined {
        switch (key) {
            case 'tags':
                return t(
                    'host-tags-input.tags-are-not-visible-to-end-users-tag-will-be-sent-with-raw-subscription-only'
                )
            case 'nodes':
                return t(
                    'base-host-form.pick-nodes-which-resolved-from-this-host-only-visual-assignment'
                )
            case 'internalSquads':
                return squadMode === INTERNAL_SQUADS_MODE.ALLOW_ONLY
                    ? t('base-host-form.allow-this-host-only-for-specific-internal-squads')
                    : t('base-host-form.exclude-this-host-from-specific-internal-squads')
            case 'sni':
                return `${t('base-host-form.sni-description-line-1')}\n\n${t('base-host-form.sni-description-line-2')}`
            case 'overrideSniFromAddress':
                return t('base-host-form.override-sni-from-address-description')
            case 'keepSniBlank':
                return t('base-host-form.keep-sni-blank-description')
            case 'vlessRouteId':
                return t('base-host-form.vless-route-description')
            case 'includeInSrrRules':
                return t('base-host-form.response-rules-description')
            case 'isHidden':
                return t('base-host-form.hidden-host-description')
            case 'shuffleHost':
                return t('base-host-form.shuffled-hosts-hover-card')
            default:
                return undefined
        }
    }

    function setOption(key: HostField, value: string): void {
        selectValues.set(key, value)
        onfieldchange(key)
    }
</script>

{#snippet entry(
    key: HostField,
    label: string,
    field: Snippet,
    inline = false,
    headerControl?: Snippet
)}
    {#if !bulk && core.has(key)}
        {@render field()}
    {:else if (bulk && core.has(key)) || activeOptions.has(key)}
        {@const coreBulkField = bulk && core.has(key)}
        {@const expanded = !coreBulkField || applied.has(key)}
        {@const helpText = helpMessage(key)}
        {@const docsUrl = optionHelpDocs[key]}
        <div class="field optional-field" class:inline={inline}>
            <div class="option-field-label">
                {#if coreBulkField}
                    <label class="apply">
                        <input
                            id={`${id}-apply-${key}`}
                            type="checkbox"
                            name={`apply:${key}`}
                            checked={expanded}
                            onchange={(event) => apply(key, event.currentTarget.checked)}
                        >
                        {label}
                    </label>
                {:else}
                    <span>{label}</span>
                    {#if bulk}
                        <input type="hidden" name={`apply:${key}`} value="">
                    {/if}
                {/if}
                {#if key === 'serverDescription' || helpText}
                    <Tooltip multiline width={280}>
                        {#snippet tip()}
                            {#if key === 'serverDescription'}
                                <div class="help-popover">
                                    <p>{t('base-host-form.server-description-1')}</p>
                                    <p>{t('base-host-form.server-description-2')}</p>
                                    <strong class="help-popover-title"
                                        >{t('base-host-form.supported-clients')}</strong
                                    >
                                    <p>
                                        Mihomo: FlClash X, Flowvy, prizrak-box, Koala Clash<br>
                                        Xray: Happ, Incy
                                    </p>
                                </div>
                            {:else}
                                <span class="help-pre">{helpText}</span>
                            {/if}
                        {/snippet}
                        <button
                            type="button"
                            class="mantine-UnstyledButton-root option-help"
                            aria-label={label}
                        >
                            <Icon name="HiQuestionMarkCircle" size={16} />
                        </button>
                    </Tooltip>
                {/if}
                {#if docsUrl}
                    <Tooltip text="Docs">
                        <a
                            class="option-docs"
                            href={docsUrl}
                            target="_blank"
                            rel="noreferrer"
                            aria-label={`${label} Docs`}
                        >
                            <Icon name="TbExternalLink" size={14} />
                        </a>
                    </Tooltip>
                {/if}
                {#if headerControl}
                    <div class="option-header-control">{@render headerControl()}</div>
                {/if}
            </div>
            {#if expanded}
                <div class="option-control">{@render field()}</div>
                {#if !core.has(key)}
                    <div class="reset-option">
                        <ActionIcon
                            icon="TbArrowBackUp"
                            label={bulk
                                ? t('base-host-form.leave-field-untouched')
                                : t('common.action.remove')}
                            size={30}
                            iconSize={16}
                            subtle
                            onclick={() => resetOption(key)}
                        />
                    </div>
                {/if}
            {/if}
        </div>
    {/if}
{/snippet}

{#snippet scalar(
    key: HostField,
    label: string,
    value: string,
    maxLength?: number,
    required = false
)}
    {@render entry(key, label, textControl)}
    {#snippet textControl()}
        {#snippet textInput()}
            <input
                class="gw-input"
                id={`${id}-${key}`}
                name={key}
                aria-label={label}
                {value}
                maxlength={maxLength}
                {required}
                placeholder={key === 'sni' || (!bulk && key === 'address')
                    ? 'example.com'
                    : undefined}
            >
        {/snippet}
        {#if !bulk && key === 'remark'}
            <div class="remark-field">
                <label for={`${id}-${key}`}>
                    {label}
                    {#if required}
                        <span class="required"> *</span>
                    {/if}
                </label>
                <span class="input-with-icon">
                    <span class="field-info remark-info"><TemplateVariables /></span>
                    {@render textInput()}
                </span>
            </div>
        {:else if !core.has(key)}
            {@render textInput()}
        {:else}
            <label for={`${id}-${key}`}>
                <span hidden={bulk}>
                    {label}
                    {#if required}
                        <span class="required"> *</span>
                    {/if}
                </span>
                {#if !bulk && key === 'address'}
                    <span class="input-with-icon address-input">
                        <span
                            class="field-info"
                            title={t('base-host-form.address-description-line-1')}
                            ><Icon name="TbInfoCircle" size={16} /></span
                        >
                        {@render textInput()}
                        <span
                            class="pattern-info"
                            title={t('base-host-form.address-description-line-2')}
                        >
                            <Icon name="HiQuestionMarkCircle" size={16} />
                        </span>
                    </span>
                {:else}
                    {@render textInput()}
                {/if}
            </label>
        {/if}
    {/snippet}
{/snippet}

{#snippet selectOption(
    key: HostField,
    label: string,
    values: readonly string[],
    value: string,
    blank = true
)}
    {@render entry(key, label, selectControl)}
    {#snippet selectControl()}
        <SelectField
            name={key}
            {label}
            hideLabel
            searchable={false}
            clearable={blank}
            placeholder={t('common.message.not-set')}
            options={values.map((option) => ({ value: option, label: option }))}
            value={selectValues.get(key) ?? value}
            onchoose={(next) => setOption(key, next)}
        />
    {/snippet}
{/snippet}

<div class="vitals">
    {#if bulk}
        <h3>{t('base-host-form.vital-parameters')}</h3>
        {@render hostSwitch('isDisabled', t('use-hosts-table-widget.disabled'))}
    {:else}
        <div class="host-visibility">
            <span class="visibility-icon" class:enabled={hostEnabled}>
                <Icon name={hostEnabled ? 'PiPulse' : 'PiProhibit'} size={22} />
            </span>
            <div class="visibility-label">
                <label for={id}>{t('base-host-form.host-visibility')}</label>
                <span class:enabled={hostEnabled}
                    >{t(
                        hostEnabled
                            ? 'use-hosts-table-widget.enabled'
                            : 'use-hosts-table-widget.disabled'
                    )}</span
                >
            </div>
            <Switch
                {id}
                switchSize="lg"
                roundedSquare
                label={t('base-host-form.host-visibility')}
                bind:checked={hostEnabled}
                onchange={() => onfieldchange('isDisabled')}
            />
            {#if !hostEnabled}
                <input type="hidden" name="isDisabled" value="on">
            {/if}
        </div>
    {/if}
    <div class="essential-fields">
        {@render scalar('remark', t('base-host-form.remark'), host?.remark ?? '', 100, true)}
        {@render entry('inbound', t('common.field.inbounds'), inboundControl)}
        {#snippet inboundControl()}
            <input type="hidden" name="inbound" value={chosenInbound}>
            <button
                type="button"
                class="mantine-UnstyledButton-root inbound-select"
                aria-label={knownInbound
                    ? t('common.action.change')
                    : t('host-select-inbound.feature.choose-an-inbound-to-apply-to-the-host')}
                onclick={() => (inboundOpen = true)}
            >
                <span class="inbound-icon" class:enabled={!!knownInbound}
                    ><Icon name={knownInbound ? 'XrayLogo' : 'TbAlertTriangle'} size={22} /></span
                >
                <span class="inbound-label">
                    <strong
                        >{activeProfile?.name ?? t('common.message.no-inbound-selected')}</strong
                    >
                    {#if knownInbound}
                        <small>{knownInbound.tag}</small>
                    {:else}
                        <small
                            >{t(
                                'host-select-inbound.feature.choose-an-inbound-to-apply-to-the-host'
                            )}</small
                        >
                    {/if}
                </span>
                <Icon name="TbChevronRight" size={18} />
            </button>
            {#if inboundOpen}
                <HostInboundPicker
                    profiles={profiles.data?.configProfiles ?? []}
                    current={chosenInbound}
                    onselect={changeInbound}
                    close={() => (inboundOpen = false)}
                />
            {/if}
            {#if profiles.isPending}
                <RouteLoading compact />
            {/if}
            {#if profiles.error}
                <p role="alert">{profiles.error.message}</p>
            {/if}
        {/snippet}
        <div class="address-fields">
            {@render scalar(
                'address',
                t('common.field.address'),
                host?.address ?? '',
                undefined,
                true
            )}
            {@render entry('port', t('common.field.port'), portControl)}
            {#snippet portControl()}
                <label>
                    {#if !bulk}
                        <span>{t('common.field.port')}<span class="required"> *</span></span>
                    {/if}
                    <span class="input-with-icon">
                        <span class="field-info" title={t('base-host-form.port-description-line-1')}
                            ><Icon name="TbInfoCircle" size={16} /></span
                        >
                        <input
                            class="gw-input"
                            name="port"
                            aria-label={t('common.field.port')}
                            value={port ?? host?.port ?? 0}
                            type="number"
                            min="1"
                            max="65535"
                            step="1"
                            placeholder="443"
                            required
                        >
                    </span>
                </label>
            {/snippet}
        </div>
    </div>
</div>

<section class="options">
    <header class="option-header">
        <span class="visibility-icon enabled"><Icon name="PiListChecks" size={20} /></span>
        <h3>{t('base-host-form.options')}</h3>
        <Button
            icon="TbPlus"
            size="sm"
            variant="soft"
            color="gray"
            disabled={activeOptions.size >= optional.length}
            aria-haspopup="dialog"
            aria-expanded={addOpen}
            onclick={(event) => {
                trigger = event.currentTarget
                addOpen = !addOpen
                optionSearch = ''
                highlightedOption = undefined
            }}
        >
            {t('common.action.add')}
        </Button>
    </header>
    {#if addOpen && trigger}
        <TablePopup
            anchor={trigger}
            id={`${id}-host-picker`}
            role="dialog"
            label={t('base-host-form.options')}
            width={320}
            maxHeight={325}
            alignEnd
            focusOnOpen
            close={() => (addOpen = false)}
        >
            <div class="option-picker">
                <input
                    class="gw-input"
                    aria-label={t('common.action.search')}
                    placeholder={t('common.action.search')}
                    bind:value={optionSearch}
                    data-popup-autofocus
                    role="combobox"
                    aria-expanded="true"
                    aria-controls={`${id}-host-options`}
                    aria-activedescendant={highlightedOption
                        ? `${id}-host-option-${highlightedOption}`
                        : undefined}
                    oninput={() => (highlightedOption = undefined)}
                    onkeydown={(event) => {
                        const options = availableOptions.flatMap(({ options }) => options)
                        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
                            event.preventDefault()
                            highlightedOption = nextComboboxOption(
                                event.key,
                                highlightedOption,
                                options.map(({ key }) => ({ value: key }))
                            )
                            document
                                .getElementById(`${id}-host-option-${highlightedOption}`)
                                ?.scrollIntoView({ block: 'nearest' })
                        } else if (event.key === 'Enter') {
                            event.preventDefault()
                            const option = options.find(({ key }) => key === highlightedOption)
                            if (option) chooseOption(option.key)
                        } else if (event.key === 'Escape') {
                            event.preventDefault()
                            event.stopPropagation()
                            trigger?.focus({ preventScroll: true })
                            addOpen = false
                        }
                    }}
                >
                <div
                    class="option-results"
                    id={`${id}-host-options`}
                    role="listbox"
                    aria-label={t('base-host-form.options')}
                >
                    {#each availableOptions as { group, options } (group)}
                        <div class="picker-group">{t(group)}</div>
                        {#each options as { key, title } (key)}
                            <button
                                type="button"
                                id={`${id}-host-option-${key}`}
                                role="option"
                                aria-selected={key === highlightedOption}
                                tabindex="-1"
                                onclick={() => chooseOption(key)}
                            >
                                {title}
                            </button>
                        {/each}
                    {:else}
                        <div class="picker-group">{t('common.message.nothing-found')}</div>
                    {/each}
                </div>
            </div>
        </TablePopup>
    {/if}
    {#if !activeOptions.size}
        <div class="empty-options">
            <span><Icon name="TbAdjustmentsHorizontal" size={24} /></span>
            <strong>{t('base-host-form.no-options-title')}</strong>
            <p>{t('base-host-form.no-options-description')}</p>
        </div>
    {/if}
    <div class="option-group" hidden={!hasOption(assignmentFields)}>
        <div class="option-label">{t('base-host-form.group-assignment')}</div>
        <div class="fields">
            {@render entry('tags', t('common.field.tags'), tagsControl)}
            {#snippet tagsControl()}
                <HostTags
                    bind:values={hostTags}
                    label={t('common.field.tags')}
                    suggestions={tags.data?.tags ?? []}
                    onchange={() => onfieldchange('tags')}
                />
            {/snippet}
            {@render entry('nodes', t('constants.nodes'), assignedNodes)}
            {#snippet assignedNodes()}
                <MultiSelect
                    name="nodes"
                    label={t('constants.nodes')}
                    hideLabel
                    options={nodeSelectOptions(nodes.data ?? [], hostNodes)}
                    bind:values={hostNodes}
                    onchange={() => onfieldchange('nodes')}
                />
                {#if nodes.error}
                    <p role="alert">{nodes.error.message}</p>
                {/if}
            {/snippet}
            {#snippet squadModeControl()}
                <fieldset class="squad-mode" aria-label={t('base-host-form.access-mode')}>
                    {#each Object.values(INTERNAL_SQUADS_MODE) as mode (mode)}
                        <label>
                            <input
                                type="radio"
                                name="internalSquads:mode"
                                value={mode}
                                checked={squadMode === mode}
                                onchange={() => (squadMode = mode)}
                            >
                            <span class="choice"
                                >{t(
                                    mode === INTERNAL_SQUADS_MODE.EXCLUDE
                                        ? 'base-host-form.internal-squads-mode-exclude'
                                        : 'base-host-form.internal-squads-mode-allow-only'
                                )}</span
                            >
                        </label>
                    {/each}
                </fieldset>
            {/snippet}
            {@render entry(
                'internalSquads',
                t('constants.internal-squads'),
                access,
                false,
                squadModeControl
            )}
            {#snippet access()}
                <MultiSelect
                    name="internalSquads:squads"
                    label={t('constants.internal-squads')}
                    options={namedOptions(squads.data?.internalSquads ?? [], assignedSquads)}
                    bind:values={assignedSquads}
                    onchange={() => onfieldchange('internalSquads')}
                />
                {#if squads.error}
                    <p role="alert">{squads.error.message}</p>
                {/if}
            {/snippet}
        </div>
    </div>

    <div class="option-group" hidden={!hasOption(securityFields)}>
        <div class="option-label">{t('base-host-form.group-security')}</div>
        <div class="fields">
            {@render scalar('sni', t('base-host-form.sni'), optionValue('sni') ?? '')}
            {@render hostSwitch(
                'overrideSniFromAddress',
                t('base-host-form.override-sni-from-address')
            )}
            {@render hostSwitch('keepSniBlank', t('base-host-form.keep-sni-blank'))}
            {@render selectOption(
                'securityLayer',
                t('base-host-form.security-layer'),
                Object.values(SECURITY_LAYERS),
                optionValue('securityLayer') ?? SECURITY_LAYERS.DEFAULT,
                false
            )}
            {@render entry('fingerprint', t('base-host-form.fingerprint'), fingerprintControl)}
            {#snippet fingerprintControl()}
                <SelectField
                    name="fingerprint"
                    label={t('base-host-form.fingerprint')}
                    hideLabel
                    placeholder="chrome"
                    autocomplete="suggest"
                    options={FINGERPRINTS.map((value) => ({ value, label: value }))}
                    value={fingerprint}
                    onchoose={(value) => {
                        fingerprint = value
                        onfieldchange('fingerprint')
                    }}
                />
            {/snippet}
        </div>
    </div>

    <div class="option-group" hidden={!hasOption(transportFields)}>
        <div class="option-label">{t('base-host-form.group-transport')}</div>
        <div class="fields">
            {@render entry('vlessRouteId', t('base-host-form.vless-route-id'), routeId)}
            {#snippet routeId()}
                <input
                    class="gw-input"
                    name="vlessRouteId"
                    aria-label={t('base-host-form.vless-route-id')}
                    type="number"
                    min="0"
                    max="65535"
                    step="1"
                    value={optionValue('vlessRouteId') ?? ''}
                >
            {/snippet}
            {@render scalar('host', t('base-host-form.host'), optionValue('host') ?? '')}
            {@render scalar('path', t('base-host-form.path'), optionValue('path') ?? '')}
            {@render selectOption(
                'alpn',
                t('base-host-form.alpn'),
                Object.values(ALPN),
                optionValue('alpn') ?? ''
            )}
        </div>
    </div>

    <div class="option-group" hidden={!hasOption(subscriptionFields)}>
        <div class="option-label">{t('base-host-form.group-subscription')}</div>
        <div class="fields">
            {@render scalar(
                'serverDescription',
                t('base-host-form.server-description-header'),
                optionValue('serverDescription') ?? '',
                30
            )}
            {@render entry(
                'excludeFromSubscriptionTypes',
                t('base-host-form.exclude-from-subscription-type'),
                excludedTypes
            )}
            {#snippet excludedTypes()}
                <fieldset
                    class="subscription-types"
                    aria-label={t('base-host-form.exclude-from-subscription-type')}
                >
                    <div class="chip-grid">
                        {#each subscriptionTypeItems as item (item.value)}
                            {@const checked = excludedSubscriptions.includes(item.value)}
                            <label class="chip-card" class:checked>
                                <span class="chip-left">
                                    <input
                                        type="checkbox"
                                        name="excludeFromSubscriptionTypes"
                                        value={item.value}
                                        {checked}
                                        onchange={(event) => {
                                            excludedSubscriptions = event.currentTarget.checked
                                                ? [...excludedSubscriptions, item.value]
                                                : excludedSubscriptions.filter(
                                                      (val) => val !== item.value
                                                  )
                                            onfieldchange('excludeFromSubscriptionTypes')
                                        }}
                                    >
                                    <span class="chip-label">{item.label}</span>
                                </span>
                                <span class="chip-icon" aria-hidden="true">
                                    <Icon name={item.icon} size={16} />
                                </span>
                            </label>
                        {/each}
                    </div>
                </fieldset>
            {/snippet}
            {@render entry('includeInSrrRules', t('constants.response-rules'), rules)}
            {#snippet rules()}
                <MultiSelect
                    name="includeInSrrRules"
                    label={t('constants.response-rules')}
                    hideLabel
                    create
                    placeholder={t('base-host-form.response-rules-placeholder')}
                    options={[
                        ...new Set(settings.data?.responseRules?.rules.map(({ name }) => name))
                    ].map((value) => ({
                        value,
                        label: value
                    }))}
                    bind:values={responseRules}
                    onchange={() => onfieldchange('includeInSrrRules')}
                />
            {/snippet}
            {@render hostSwitch('isHidden', t('base-host-form.hide-host'))}
            {@render hostSwitch('shuffleHost', t('base-host-form.shuffle-host'))}
        </div>
    </div>

    <div class="option-group" hidden={!hasOption(advancedFields)}>
        <div class="option-label">{t('base-host-form.group-advanced')}</div>
        <div class="fields">
            {@render entry(
                'xrayJsonTemplateUuid',
                t('base-host-form.xray-json-template'),
                template
            )}
            {#snippet template()}
                {@const savedTemplate = optionValue('xrayJsonTemplateUuid')}
                <SelectField
                    name="xrayJsonTemplateUuid"
                    label={t('base-host-form.xray-json-template')}
                    hideLabel
                    placeholder={t('base-host-form.select-a-xray-json-template')}
                    options={namedOptions(
                        templates.data?.templates.filter(
                            (item) => item.templateType === SUBSCRIPTION_TEMPLATE_TYPE.XRAY_JSON
                        ) ?? [],
                        savedTemplate ? [savedTemplate] : []
                    )}
                    value={selectValues.get('xrayJsonTemplateUuid') ?? savedTemplate ?? ''}
                    onchoose={(value) => setOption('xrayJsonTemplateUuid', value)}
                />
                {#if templates.error}
                    <p role="alert">{templates.error.message}</p>
                {/if}
            {/snippet}
            {#each Object.entries(jsonOptions) as [field, label] (field)}
                {@const key = field as keyof typeof jsonOptions}
                {@render entry(key, t(label), jsonControl, true)}
                {#snippet jsonControl()}
                    <HostJsonInput
                        field={key}
                        label={t(label)}
                        value={jsonText(optionValue(key))}
                        rawInbound={knownInbound?.rawInbound}
                        disabled={key === 'xhttpExtraParams' &&
                            (!bulk || Boolean(chosenInbound)) &&
                            knownInbound?.network !== 'xhttp'}
                        onchange={() => onfieldchange(key)}
                    />
                {/snippet}
            {/each}
        </div>
    </div>

    <div class="option-group" hidden={!hasOption(clientFields)}>
        <div class="option-label">{t('base-host-form.group-client')}</div>
        <div class="fields">
            {@render scalar(
                'pinnedPeerCertSha256',
                t('base-host-form.pinned-peer-cert-sha256'),
                optionValue('pinnedPeerCertSha256') ?? ''
            )}
            {@render scalar(
                'verifyPeerCertByName',
                t('base-host-form.verify-peer-cert-by-name'),
                optionValue('verifyPeerCertByName') ?? ''
            )}
            {@render hostSwitch('mihomoX25519', t('base-host-form.enable-x25519mlkem768'))}
            {@render selectOption(
                'mihomoIpVersion',
                t('base-host-form.mihomo-ip-version'),
                Object.values(MIHOMO_IP_VERSION),
                optionValue('mihomoIpVersion') ?? ''
            )}
        </div>
    </div>
</section>

{#snippet hostSwitch(key: keyof typeof switches, label: string)}
    {@render entry(key, label, switchControl, !core.has(key))}
    {#snippet switchControl()}
        {#if core.has(key)}
            <label class="switch">
                <input type="checkbox" name={key} checked={host?.[key] ?? !bulk}>
                {bulk ? t('common.action.enable') : label}
            </label>
        {:else}
            <Switch name={key} {label} checked={optionValue(key) ?? false} />
        {/if}
    {/snippet}
{/snippet}

<style>
    .vitals,
    .options {
        min-width: 0;
        margin: 0;
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
    }
    .host-visibility,
    .option-header {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        margin: 0 var(--mantine-spacing-md);
        padding: var(--mantine-spacing-md) 0;
        border-block-end: 1px solid rgba(var(--gw-fg-rgb), 0.06);
    }
    .visibility-icon,
    .inbound-icon {
        display: grid;
        place-items: center;
        flex: none;
        width: 34px;
        height: 34px;
        color: var(--mantine-color-gray-4);
        border-radius: var(--mantine-radius-md);
        border: 1px solid rgb(134 142 150 / 30%);
        background: linear-gradient(135deg, rgb(134 142 150 / 15%), rgb(73 80 87 / 10%));
    }
    .visibility-icon.enabled,
    .inbound-icon.enabled {
        color: var(--mantine-color-teal-4);
        border-color: rgb(18 184 134 / 30%);
        background: linear-gradient(135deg, rgb(18 184 134 / 15%), rgb(12 166 120 / 10%));
    }
    .visibility-label {
        display: grid;
        flex: 1;
    }
    .visibility-label span {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        font-weight: 600;
        line-height: var(--mantine-line-height-xs);
    }
    .visibility-label .enabled {
        color: var(--mantine-color-teal-4);
    }
    .essential-fields {
        padding: var(--mantine-spacing-md);
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    .address-fields {
        display: grid;
        grid-template-columns: minmax(0, 2.13fr) minmax(0, 1fr);
        gap: var(--mantine-spacing-xs);
    }
    label {
        display: grid;
        min-width: 0;
        gap: 2px;
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
    }
    .essential-fields label:has(.input-with-icon) {
        gap: 0;
    }
    .essential-fields label > span:first-child:not([hidden], .input-with-icon),
    .remark-field > label {
        min-height: calc(var(--mantine-font-size-md) * var(--mantine-line-height-md));
    }
    .remark-field > label {
        display: block;
        font-weight: 600;
    }
    .visibility-label label {
        gap: 0;
        cursor: pointer;
        font-weight: 600;
        line-height: var(--mantine-line-height-sm);
    }
    .required {
        color: var(--mantine-color-error);
    }
    .input-with-icon {
        position: relative;
        display: grid;
        min-width: 0;
    }
    .field-info {
        position: absolute;
        inset-inline-start: 10px;
        inset-block-start: 50%;
        transform: translateY(-50%);
        color: var(--mantine-color-dimmed);
        display: flex;
    }
    .remark-info {
        inset-inline-start: 3px;
    }
    .pattern-info {
        position: absolute;
        inset-inline-end: 10px;
        inset-block-start: 50%;
        transform: translateY(-50%);
        display: flex;
        color: var(--mantine-color-gray-light-color);
    }
    .input-with-icon input {
        padding-inline-start: 36px;
        min-width: 0;
    }
    .address-input input {
        padding-inline-end: 36px;
    }
    :is(label, .remark-field) input:not([type="checkbox"], [type="radio"]) {
        min-height: 36px;
        height: 36px;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-dark-6);
        font-size: var(--mantine-font-size-sm);
    }
    .essential-fields input::placeholder {
        color: var(--mantine-color-placeholder);
    }
    .squad-mode {
        min-width: 0;
        margin: 0;
        border: 0;
        display: flex;
        flex-wrap: wrap;
        gap: 4px;
        width: fit-content;
        padding: 3px;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-default);
    }
    .subscription-types {
        min-width: 0;
        margin: 0;
        border: 0;
        padding: 0;
        display: grid;
        gap: 5px;
        font-size: var(--mantine-font-size-sm);
    }
    .squad-mode label {
        position: relative;
    }
    .squad-mode input {
        position: absolute;
        width: 1px;
        height: 1px;
        opacity: 0;
        pointer-events: none;
    }
    .choice {
        padding: 3px var(--mantine-spacing-xs);
        border-radius: var(--mantine-radius-sm);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        font-weight: 500;
        cursor: pointer;
    }
    .squad-mode input:focus-visible + .choice {
        outline: 2px solid var(--mantine-color-cyan-6);
    }
    .squad-mode input:checked + .choice {
        background: var(--mantine-color-body);
        color: var(--mantine-color-text);
    }
    .chip-grid {
        display: grid;
        grid-template-columns: repeat(3, minmax(0, 1fr));
        gap: var(--mantine-spacing-xs);
        margin-top: 4px;
    }
    @media (max-width: 40em) {
        .chip-grid {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }
    @media (max-width: 30em) {
        .chip-grid {
            grid-template-columns: minmax(0, 1fr);
        }
    }
    .chip-card {
        --gw-checkbox-size: 18px;
        --gw-checkbox-fill: var(--mantine-color-red-filled);
        position: relative;
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--mantine-spacing-xs);
        padding: 8px 12px;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-dark-6);
        border: 1px solid var(--mantine-color-dark-3);
        cursor: pointer;
        transition: all 150ms ease;
    }
    .chip-card:has(input:focus-visible) {
        outline: 2px solid var(--mantine-primary-color-filled);
        outline-offset: 2px;
    }
    .chip-card.checked {
        border-color: var(--mantine-color-red-9);
    }
    .chip-left {
        display: inline-flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        min-width: 0;
    }
    .chip-label {
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
        color: var(--mantine-color-bright);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }
    .chip-icon {
        display: inline-flex;
        flex: none;
        color: var(--mantine-color-text);
    }
    .inbound-select {
        display: flex;
        align-items: center;
        min-width: 0;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-md);
        border-radius: var(--mantine-radius-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        background: var(--gw-raised-bg);
        font-size: var(--mantine-font-size-sm);
        cursor: pointer;
    }
    .inbound-select .inbound-icon {
        border: 0;
        background: var(--mantine-color-gray-light);
    }
    .inbound-select .inbound-icon.enabled {
        background: var(--mantine-color-teal-light);
    }
    .inbound-select:hover {
        border-color: rgba(var(--gw-fg-rgb), 0.14);
        background: rgba(var(--gw-fg-rgb), 0.035);
    }
    .inbound-label {
        flex: 1;
        min-width: 0;
        display: grid;
        line-height: var(--mantine-line-height-sm);
    }
    .inbound-label strong {
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
        font-weight: 600;
        font-family: var(--mantine-font-family-monospace);
    }
    .inbound-select:has(.inbound-icon:not(.enabled)) .inbound-label strong {
        font-family: inherit;
    }
    .inbound-label small {
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
        color: var(--mantine-color-dimmed);
        font-weight: 500;
        font-size: var(--mantine-font-size-xs);
    }
    .option-header h3 {
        margin: 0;
        flex: 1;
        color: var(--mantine-color-bright);
        font-size: var(--mantine-h5-font-size);
        font-weight: var(--mantine-h5-font-weight);
    }
    .option-header :global(button) {
        width: min(200px, 61%);
    }
    .option-group {
        padding: var(--mantine-spacing-sm) var(--mantine-spacing-md) var(--mantine-spacing-md);
        min-width: 0;
    }
    .option-group[hidden] {
        display: none;
    }

    .option-label {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        margin-block-end: var(--mantine-spacing-sm);
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    .option-label::after {
        height: 1px;
        flex: 1;
        content: "";
        background: var(--mantine-color-default-border);
    }
    .fields,
    .field {
        display: grid;
        gap: var(--mantine-spacing-xs);
        min-width: 0;
    }
    .option-picker {
        display: grid;
        min-width: 0;
    }
    .option-picker > input {
        border: 0;
        border-block-end: 1px solid var(--mantine-color-default-border);
        border-radius: 0;
    }
    .option-results {
        display: grid;
        max-height: 280px;
        overflow-y: auto;
        padding: 4px;
    }
    .option-results button {
        text-align: start;
        border: 0;
        background: transparent;
        color: var(--mantine-color-text);
        font-weight: 400;
    }
    .option-results button:hover,
    .option-results button[aria-selected="true"] {
        background: var(--mantine-color-default-hover);
    }
    .picker-group::after {
        content: "";
        flex: 1;
        border-block-start: 1px solid var(--gw-border);
    }
    .picker-group {
        display: flex;
        align-items: center;
        gap: 6px;
        font-weight: 600;
        padding: 4px 8px;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    .optional-field {
        display: grid;
        grid-template-columns: minmax(0, 1fr) auto;
        align-items: center;
        column-gap: var(--mantine-spacing-xs);
    }
    .option-field-label {
        display: flex;
        align-items: center;
        gap: 4px;
        min-height: 30px;
        min-width: 0;
        font-size: var(--mantine-font-size-sm);
        font-weight: var(--mantine-font-weight-medium);
    }
    .option-help {
        display: inline-grid;
        color: var(--mantine-color-gray-light-color);
        cursor: help;
    }
    .option-docs {
        display: inline-grid;
        color: var(--mantine-color-dimmed);
    }
    .option-docs:hover {
        color: var(--mantine-color-bright);
    }
    .option-header-control {
        margin-inline-start: auto;
        flex-shrink: 0;
    }
    .help-pre {
        white-space: pre-line;
    }
    .help-popover {
        display: grid;
        gap: 6px;
        font-size: var(--mantine-font-size-xs);
    }
    .help-popover-title {
        color: var(--mantine-color-white);
    }
    .option-control {
        display: grid;
        gap: var(--mantine-spacing-xs);
        grid-column: 1 / -1;
        grid-row: 2;
        min-width: 0;
    }
    .reset-option {
        grid-column: 2;
        grid-row: 1;
        opacity: 0.45;
        transition: opacity 120ms ease;
    }
    .inline .reset-option {
        grid-column: 3;
    }
    .optional-field:hover .reset-option,
    .optional-field:focus-within .reset-option {
        opacity: 1;
    }
    @media (hover: none) {
        .reset-option {
            opacity: 1;
        }
    }
    .optional-field.inline {
        grid-template-columns: minmax(0, 1fr) auto auto;
    }
    .inline .option-control {
        grid-column: 2;
        grid-row: 1;
    }
    .empty-options {
        display: grid;
        justify-items: center;
        padding: 44px 16px;
        gap: 6px;
        font-size: var(--mantine-font-size-sm);
        text-align: center;
    }
    .empty-options span {
        display: grid;
        width: 48px;
        height: 48px;
        place-items: center;
        border-radius: 50%;
        margin-block-end: 4px;
        color: var(--mantine-color-gray-light-color);
        background: var(--mantine-color-gray-light);
    }
    p {
        margin: 0;
    }
    .empty-options p {
        max-width: 360px;
        color: var(--mantine-color-dimmed);
    }
    label.apply,
    label.switch {
        display: flex;
        align-items: center;
        gap: 6px;
    }
</style>
