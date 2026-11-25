<script lang="ts">
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Icon from '../ui/Icon.svelte'
    import { type NodeMetricsRow, visibleTraffic } from './analytics'

    let { node, open }: { node: NodeMetricsRow; open: () => void } = $props()
    let t = $derived(locale.t)
    let hideZero = $state(true)
    let inbounds = $derived(visibleTraffic(node.inboundsStats, hideZero))
    let outbounds = $derived(visibleTraffic(node.outboundsStats, hideZero))
</script>

<article>
    <header>
        <ActionIcon icon="TbServer2" tone="indigo" label={node.nodeName} onclick={open} size={44} />
        <div class="identity">
            <h2 title={node.nodeName}>
                <span aria-hidden="true">{node.countryEmoji}</span>
                <span>{node.nodeName}</span>
            </h2>
            {#if node.providerName !== 'unknown'}
                <p title={node.providerName}>{node.providerName}</p>
            {/if}
        </div>
        <ActionIcon
            icon={hideZero ? 'TbEyeOff' : 'TbEye'}
            label={hideZero
                ? t('node-details-card.show-zero-values')
                : t('node-details-card.hide-zero-values')}
            tone="indigo"
            size={28}
            iconSize={16}
            class="hide-zero-values-toggle"
            onclick={() => (hideZero = !hideZero)}
        />
        <span class="online" title="Users online">
            <Icon name="PiUsersDuotone" size={15} />
            <span class="visually-hidden">Users online</span>
            {node.usersOnline.toLocaleString(locale.language)}
        </span>
    </header>

    <div class="traffic-panels">
        {@render traffic(
            'Inbound Traffic',
            inbounds,
            t('node-details-card.no-inbound-traffic-data'),
            true
        )}
        {@render traffic(
            'Outbound Traffic',
            outbounds,
            t('node-details-card.no-outbound-traffic-data'),
            false
        )}
    </div>
</article>

{#snippet traffic(
    heading: string,
    rows: NodeMetricsRow['inboundsStats'],
    empty: string,
    inbound: boolean
)}
    <section class="traffic" class:inbound aria-label={heading}>
        <h3>
            <Icon name={inbound ? 'PiArrowDownDuotone' : 'PiArrowUpDuotone'} size={16} />
            {heading}
        </h3>
        {#if rows.length}
            <ul>
                {#each rows as row (row.tag)}
                    <li>
                        <span class="tag" title={row.tag}>
                            <Icon name="PiTag" size={13} />
                            <span>{row.tag}</span>
                        </span>
                        <dl>
                            {@render direction('Up', row.upload, true)}
                            {@render direction('Down', row.download, false)}
                        </dl>
                    </li>
                {/each}
            </ul>
        {:else}
            <p class="empty">
                <span class="empty-icon"><Icon name="PiProhibitDuotone" size={16} /></span>
                {empty}
            </p>
        {/if}
    </section>
{/snippet}

{#snippet direction(label: string, value: string, upload: boolean)}
    <div class="direction" class:upload>
        <dt>
            <Icon name={upload ? 'PiArrowUpDuotone' : 'PiArrowDownDuotone'} size={12} />
            {label}
        </dt>
        <dd>{value === '0' ? '0 B' : value}</dd>
    </div>
{/snippet}

<style>
    article {
        min-width: 0;
        padding: var(--mantine-spacing-md);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        container-type: inline-size;
        animation: enter 0.2s ease-out;
    }

    header {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        padding-block-end: var(--mantine-spacing-md);
        border-block-end: 1px solid rgba(var(--gw-fg-rgb), 0.08);
    }

    h2,
    h3,
    p,
    dl,
    dd {
        margin: 0;
    }

    .identity {
        flex: 1;
        min-width: 0;
        margin-inline-start: 6px;
    }

    h2 {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        color: var(--mantine-color-bright);
        font-size: var(--mantine-font-size-md);
        font-weight: 600;
    }

    h2 span:last-child,
    .identity p,
    .tag span {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .identity p {
        color: var(--mantine-color-gray-4);
        font-size: var(--mantine-font-size-sm);
    }

    .identity {
        min-height: calc(
            var(--mantine-line-height-md) *
            var(--mantine-font-size-md) +
            var(--mantine-line-height-sm) *
            var(--mantine-font-size-sm) +
            0.25rem
        );
    }

    .online,
    .tag {
        display: inline-flex;
        align-items: center;
        gap: 0.25rem;
        border-radius: 100vw;
        padding-inline: 0.5rem;
        font-size: var(--mantine-font-size-xs);
        font-weight: 600;
    }

    .online {
        flex: none;
        min-height: var(--badge-height-lg, 1.625rem);
        min-width: 3.334rem;
        justify-content: center;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-teal-light);
        color: var(--mantine-color-teal-light-color);
    }

    .visually-hidden {
        position: absolute;
        width: 1px;
        height: 1px;
        overflow: hidden;
        clip-path: inset(50%);
        white-space: nowrap;
    }

    .traffic-panels {
        display: grid;
        align-items: start;
        gap: var(--mantine-spacing-md);
        margin-block-start: var(--mantine-spacing-md);
    }

    .traffic {
        --traffic-color: var(--mantine-color-blue-4);
        --traffic-tint: var(--mantine-color-blue-6);
        min-width: 0;
        padding: var(--mantine-spacing-md);
        border: 1px solid color-mix(in srgb, var(--traffic-tint) 15%, transparent);
        border-radius: var(--mantine-radius-md);
        background: color-mix(in srgb, var(--traffic-tint) 5%, transparent);
    }

    .traffic.inbound {
        --traffic-color: var(--mantine-color-teal-4);
        --traffic-tint: var(--mantine-color-teal-6);
    }

    h3 {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        color: var(--traffic-color);
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        line-height: var(--mantine-line-height-sm);
    }

    ul {
        display: grid;
        gap: 0;
        margin: var(--mantine-spacing-xs) 0 0;
        padding: 0;
        list-style: none;
    }

    .tag {
        display: flex;
        width: fit-content;
        max-width: 100%;
        height: 18px;
        gap: 0.5rem;
        line-height: 16px;
        border: 1px solid var(--traffic-color);
        color: var(--traffic-color);
        font-size: 10px;
        font-weight: 700;
        text-transform: uppercase;
    }

    li {
        margin-block-end: var(--mantine-spacing-sm);
    }

    dl {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: var(--mantine-spacing-xs);
        margin-block-start: var(--mantine-spacing-xs);
    }

    .direction {
        display: grid;
        gap: 1px;
    }

    dt {
        display: flex;
        align-items: flex-start;
        gap: 0.25rem;
        color: var(--mantine-color-green-4);
        font-size: 9px;
        font-weight: 500;
        line-height: 1;
        height: 9px;
        text-transform: uppercase;
    }

    dt :global(svg) {
        margin-block-start: 2px;
    }

    .upload dt {
        color: var(--mantine-color-orange-4);
    }

    dd {
        overflow-wrap: anywhere;
        padding-inline-start: 1rem;
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }

    .empty {
        display: grid;
        justify-items: center;
        gap: var(--mantine-spacing-xs);
        margin-block-start: var(--mantine-spacing-xs);
        color: var(--mantine-color-gray-5);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
        text-align: center;
    }
    .empty-icon {
        display: grid;
        width: 22px;
        height: 22px;
        place-items: center;
        border: 0;
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-gray-light);
        color: var(--mantine-color-gray-light-color);
    }

    :global(.action-icon.hide-zero-values-toggle.tinted) {
        border-color: transparent;
        background: var(--mantine-color-indigo-light);
    }

    @container (min-width: 26rem) {
        .traffic-panels {
            grid-template-columns: repeat(2, minmax(0, 1fr));
        }
    }

    @keyframes enter {
        from {
            opacity: 0;
            transform: scale(0.95);
        }
    }

    @media (prefers-reduced-motion: reduce) {
        article {
            animation: none;
        }
    }
</style>
