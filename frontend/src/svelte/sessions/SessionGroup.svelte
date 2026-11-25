<script lang="ts">
    import { locale } from '../i18n.svelte'
    import { p } from '../router'
    import SessionIps from './SessionIps.svelte'
    import type { SessionIp, SessionNode } from './scan'

    let {
        node,
        ips,
        search,
        copy
    }: {
        node: SessionNode
        ips: SessionIp[]
        search: string
        copy: (ip: string) => void
    } = $props()
</script>

<section aria-label={node.name}>
    <h3>
        <a href={p('/dashboard/open/node/:id', { params: { id: node.uuid } })}>{node.name}</a>
        {#if node.countryCode && node.countryCode !== 'XX'}
            <small>{node.countryCode}</small>
        {/if}
        <small>{ips.length.toLocaleString(locale.language)}</small>
    </h3>
    <SessionIps {ips} {search} {copy} />
</section>

<style>
    h3 {
        display: flex;
        align-items: baseline;
        gap: 0.5rem;
        margin: 0 0 0.4rem;
        font-size: 1rem;
    }
    small {
        font-weight: normal;
    }
</style>
