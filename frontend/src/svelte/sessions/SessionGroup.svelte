<script lang="ts">
    import { locale } from '../i18n.svelte'
    import { flag } from '../nodes/analytics'
    import { p } from '../router'
    import SessionIps from './SessionIps.svelte'
    import type { SessionIp, SessionNode } from './scan'

    let {
        node,
        ips,
        search,
        now,
        copy,
        onnode
    }: {
        node: SessionNode
        ips: SessionIp[]
        search: string
        now: number
        copy?: (ip: string) => void
        onnode?: (uuid: string) => void
    } = $props()
    let countryFlag = $derived(flag(node.countryCode))
</script>

<section aria-label={node.name}>
    <div class="node-header">
        <h3>
            {#if countryFlag}
                <span class="node-flag" role="img" aria-label={node.countryCode}
                    >{countryFlag}</span
                >
            {/if}
            {#if onnode}
                <button
                    type="button"
                    class="mantine-UnstyledButton-root node-link"
                    onclick={() => onnode(node.uuid)}
                >
                    {node.name}
                </button>
            {:else}
                <a href={p('/dashboard/open/node/:id', { params: { id: node.uuid } })}
                    >{node.name}</a
                >
            {/if}
        </h3>
        <span class="node-count" title={locale.t('sessions-explorer.widget.total-ips')}>
            {ips.length.toLocaleString(locale.language)}
        </span>
    </div>
    <SessionIps {ips} {search} {copy} {now} compact />
</section>

<style>
    .node-header,
    h3 {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .node-header {
        position: sticky;
        z-index: 2;
        inset-block-start: 0;
        justify-content: space-between;
        padding: 8px 4px;
        background:
            linear-gradient(rgba(var(--gw-fg-rgb), 0.02), rgba(var(--gw-fg-rgb), 0.02)),
            var(--mantine-color-body);
        box-shadow:
            1px 0 0 0 var(--mantine-color-body),
            -1px 0 0 0 var(--mantine-color-body),
            0 1px 0 0 rgba(var(--gw-fg-rgb), 0.06);
    }
    h3 {
        min-width: 0;
        margin: 0;
        font-size: var(--mantine-font-size-sm);
    }
    .node-flag {
        font-size: 1.25em;
        line-height: 1;
        flex: none;
    }
    h3 a,
    .node-link {
        color: var(--mantine-color-bright);
        overflow-wrap: anywhere;
        text-decoration: none;
        font-weight: 600;
        text-align: start;
    }
    h3 a:hover,
    .node-link:hover {
        text-decoration: underline;
    }
    .node-count {
        flex: none;
        min-width: 1.9rem;
        padding: 0.2rem 0.5rem;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-default);
        font-size: var(--mantine-font-size-xs);
        font-weight: 700;
        text-align: center;
    }
</style>
