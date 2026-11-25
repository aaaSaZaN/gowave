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
        copy
    }: {
        node: SessionNode
        ips: SessionIp[]
        search: string
        now: number
        copy: (ip: string) => void
    } = $props()
    let countryFlag = $derived(flag(node.countryCode))
</script>

<section aria-label={node.name}>
    <div class="node-header">
        <h3>
            {#if countryFlag}
                <span role="img" aria-label={node.countryCode}>{countryFlag}</span>
            {/if}
            <a href={p('/dashboard/open/node/:id', { params: { id: node.uuid } })}>{node.name}</a>
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
        z-index: 1;
        inset-block-start: 0;
        justify-content: space-between;
        padding-block: var(--mantine-spacing-xs);
        background: var(--gw-raised-bg);
    }
    h3 {
        min-width: 0;
        margin: 0;
        font-size: var(--mantine-font-size-sm);
    }
    h3 a {
        color: var(--mantine-color-bright);
        overflow-wrap: anywhere;
        text-decoration: none;
    }
    h3 a:hover {
        text-decoration: underline;
    }
    .node-count {
        flex: none;
        min-width: 1.8rem;
        padding: 0.15rem 0.4rem;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        font-size: var(--mantine-font-size-xs);
        font-weight: 700;
        text-align: center;
    }
</style>
