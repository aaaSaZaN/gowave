<script lang="ts">
    import { locale } from '../i18n.svelte'
    import UsageReport from './UsageReport.svelte'

    let t = $derived(locale.t)
</script>

<svelte:head><title>{t('constants.nodes-statistics')} | Gowave</title></svelte:head>
<main>
    <UsageReport />
</main>

<style>
    main {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
</style>
