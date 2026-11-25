<script lang="ts">
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

    let { id, title, icon }: { id?: string; title: string; icon: IconName } = $props()
</script>

<main class="stage">
    <div class="stage-card">
        <span class="stage-icon"><Icon name={icon} size={32} /></span>
        <div>
            <p>{title}</p>
            {#if id}
                <code>{id}</code>
            {/if}
        </div>
    </div>
</main>

<style>
    .stage {
        display: grid;
        place-items: center;
        min-height: calc(100dvh - var(--header-height, 116px) - 48px);
    }
    .stage-card {
        display: grid;
        justify-items: center;
        gap: var(--mantine-spacing-lg);
        max-width: 90vw;
        min-width: min(320px, 90vw);
        padding: var(--mantine-spacing-xl);
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-card-bg);
        text-align: center;
    }
    .stage-icon {
        display: grid;
        place-items: center;
        width: 64px;
        height: 64px;
        border: 1px solid rgb(134 142 150 / 30%);
        border-radius: 50%;
        background: linear-gradient(135deg, rgb(134 142 150 / 15%), rgb(73 80 87 / 10%));
        color: var(--mantine-color-gray-4);
    }
    p {
        margin: 0 0 var(--mantine-spacing-xs);
        color: var(--mantine-color-dimmed);
        font-weight: 600;
    }
    code {
        padding: 2px 4px;
        border-radius: var(--mantine-radius-sm);
        background: var(--mantine-color-dark-5);
        font-family: var(--mantine-font-family-monospace);
        font-size: var(--mantine-font-size-xs);
        overflow-wrap: anywhere;
    }
</style>
