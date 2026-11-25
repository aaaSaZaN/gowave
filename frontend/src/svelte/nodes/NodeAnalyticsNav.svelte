<script lang="ts">
    import { ROUTES } from '@shared/constants/routes'
    import { locale } from '../i18n.svelte'
    import { p } from '../router'

    let { active }: { active: 'list' | 'usage' | 'metrics' } = $props()
    let t = $derived(locale.t)
</script>

<nav class="button-row" aria-label={t('constants.nodes')}>
    <a
        aria-current={active === 'list' ? 'page' : undefined}
        href={p(ROUTES.DASHBOARD.MANAGEMENT.NODES)}
        >{t('constants.nodes')}</a
    >
    <a
        aria-current={active === 'usage' ? 'page' : undefined}
        href={p(ROUTES.DASHBOARD.MANAGEMENT.NODES_STATS)}
        >{t('constants.nodes-statistics')}</a
    >
    <a
        aria-current={active === 'metrics' ? 'page' : undefined}
        href={p(ROUTES.DASHBOARD.MANAGEMENT.NODES_METRICS)}
        >{t('constants.nodes-metrics')}</a
    >
</nav>

<style>
    nav {
        margin-block: 0.75rem 1rem;
    }
    a {
        padding: 0.5rem 0.625rem;
    }
    a[aria-current] {
        border-block-end: 2px solid var(--gw-accent);
    }
</style>
