<script lang="ts">
    import { NodeIpSchema, type TNodeIpStatus } from '@remnawave/backend-contract'
    import { onDestroy } from 'svelte'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import ActionMenu from '../ui/ActionMenu.svelte'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'

    let {
        ip = $bindable(),
        status,
        focus,
        showError,
        reorderable,
        onstatus,
        onremove,
        orderKey,
        onreorder,
        onmove
    }: {
        ip: string
        status: TNodeIpStatus
        focus: boolean
        showError: boolean
        reorderable: boolean
        onstatus: (status: TNodeIpStatus) => void
        onremove: () => void
        orderKey: string
        onreorder: (target: string) => void
        onmove: (offset: -1 | 1) => void
    } = $props()

    const statuses = {
        INBOUND: { tone: 'green', icon: 'TbArrowDownLeft', label: 'node-ip-statuses.inbound' },
        OUTBOUND: { tone: 'blue', icon: 'TbArrowUpRight', label: 'node-ip-statuses.outbound' },
        MANAGEMENT: { tone: 'violet', icon: 'TbTerminal2', label: 'node-ip-statuses.management' },
        TRANSIT: { tone: 'cyan', icon: 'TbArrowsExchange', label: 'node-ip-statuses.transit' },
        MONITORING: {
            tone: 'teal',
            icon: 'TbActivityHeartbeat',
            label: 'node-ip-statuses.monitoring'
        },
        RESERVE: { tone: 'gray', icon: 'TbBookmark', label: 'node-ip-statuses.reserve' },
        BLOCKED: { tone: 'red', icon: 'TbBan', label: 'node-ip-statuses.blocked' },
        FLAGGED: { tone: 'orange', icon: 'TbAlertTriangle', label: 'node-ip-statuses.flagged' },
        DEPRECATED: { tone: 'grape', icon: 'TbArchive', label: 'node-ip-statuses.deprecated' },
        UNKNOWN: { tone: 'gray', icon: 'TbQuestionMark', label: 'node-ip-statuses.unknown' }
    } as const satisfies Record<TNodeIpStatus, { tone: string; icon: IconName; label: string }>
    const statusOrder = Object.keys(statuses) as TNodeIpStatus[]

    const id = $props.id()
    let t = $derived(locale.t)
    let meta = $derived(statuses[status])
    let address = $derived(ip.trim())
    let valid = $derived(NodeIpSchema.shape.ip.safeParse(address).success)
    let error = $derived(
        showError && !valid ? t(address ? 'node-ips.invalid-ip' : 'node-ips.ip-is-required') : ''
    )
    let input = $state<HTMLInputElement>()
    let copied = $state(false)
    let timer: ReturnType<typeof setTimeout> | undefined

    $effect(() => {
        if (focus) input?.focus()
    })
    onDestroy(() => clearTimeout(timer))

    async function copy(): Promise<void> {
        try {
            await navigator.clipboard.writeText(address)
            copied = true
            clearTimeout(timer)
            timer = setTimeout(() => (copied = false), 2000)
        } catch {
            copied = false
        }
    }
</script>

<li class="ip-row" data-reorder-key={orderKey}>
    <ActionMenu
        icon={meta.icon}
        label={t(meta.label)}
        tone={meta.tone}
        subtle={false}
        hover={false}
        alignEnd={false}
        width={220}
        size={36}
        iconSize={20}
    >
        {#each statusOrder as value (value)}
            {@const { tone, icon, label } = statuses[value]}
            <button
                role="menuitem"
                type="button"
                tabindex="-1"
                class:active={value === status}
                onclick={() => onstatus(value)}
            >
                <span style:color={`var(--mantine-color-${tone}-5)`}
                    ><Icon name={icon} size={16} /></span
                >
                {t(label)}
            </button>
        {/each}
        <hr>
        <button role="menuitem" type="button" tabindex="-1" data-tone="red" onclick={onremove}>
            <Icon name="TbTrash" size={16} />
            {t('node-ips.remove-ip')}
        </button>
    </ActionMenu>
    <div class="address">
        <input
            bind:this={input}
            bind:value={ip}
            name="ips"
            aria-label={t('common.field.ip-address')}
            aria-invalid={!!error}
            aria-describedby={error ? id : undefined}
            placeholder="1.1.1.1"
        >
        <ActionIcon
            class={copied ? 'field-action copy copied' : 'field-action copy'}
            icon={copied ? 'PiCheck' : 'PiCopy'}
            label={t(copied ? 'common.message.copied' : 'common.action.copy')}
            size={28}
            iconSize={16}
            subtle
            disabled={!address}
            onclick={() => void copy()}
        />
        <ActionIcon
            class="field-action external"
            icon="TbExternalLink"
            label={t('common.action.open')}
            size={28}
            iconSize={16}
            subtle
            href={valid ? `https://ipinfo.io/${address}` : undefined}
            target="_blank"
            rel="noopener noreferrer"
            disabled={!valid}
        />
        {#if error}
            <small {id}>{error}</small>
        {/if}
    </div>
    <ActionIcon
        class="grip"
        icon="RiDraggable"
        label={t('node-ips.reorder-ip')}
        size={22}
        iconSize={18}
        subtle
        reorder={reorderable ? { drop: onreorder } : undefined}
        disabled={!reorderable}
        onkeydown={(event) => {
            if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return
            event.preventDefault()
            onmove(event.key === 'ArrowUp' ? -1 : 1)
        }}
    />
</li>

<style>
    .ip-row {
        display: grid;
        grid-template-columns: auto minmax(0, 1fr) 22px;
        align-items: start;
        gap: 6px;
    }
    .address {
        position: relative;
        min-width: 0;
    }
    input {
        width: 100%;
        height: 36px;
        padding-inline: 33px;
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-sm);
    }
    input::placeholder {
        color: var(--mantine-color-placeholder);
    }
    input:focus {
        outline: none;
        border-color: var(--mantine-primary-color-filled);
    }
    input[aria-invalid="true"] {
        border-color: var(--mantine-color-error);
    }
    small {
        display: block;
        color: var(--mantine-color-error);
        font-size: var(--mantine-font-size-xs);
    }
    .address :global(.field-action) {
        position: absolute;
        inset-block-start: 4px;
    }
    .ip-row :global(.field-action),
    .ip-row :global(.grip) {
        color: var(--mantine-color-dimmed);
    }
    .ip-row :global(.field-action:disabled),
    .ip-row :global(.grip:disabled) {
        background: transparent;
    }
    .address :global(.copy) {
        inset-inline-start: 3px;
    }
    .address :global(.copy.copied) {
        color: var(--mantine-color-teal-6);
    }
    .address :global(.external) {
        inset-inline-end: 3px;
    }
    .address :global(a.external) {
        color: var(--mantine-color-cyan-4);
    }
    .ip-row :global(.grip) {
        height: 36px;
    }
    button.active {
        font-weight: 700;
    }
    hr {
        border: 0;
        border-block-start: 1px solid var(--mantine-color-default-border);
    }
</style>
