<script lang="ts">
    import { getIcon, type IconName } from './icons'

    let {
        name,
        size = 20,
        class: className,
        color
    }: {
        name: IconName
        size?: number | string
        class?: string | undefined
        color?: string
    } = $props()

    const dimensions = $derived(typeof size === 'number' ? String(size) : size)
    const icon = $derived(getIcon(name))
</script>

{#snippet glyph(icon: Awaited<ReturnType<typeof getIcon>>)}
    <svg
        xmlns="http://www.w3.org/2000/svg"
        fill="currentColor"
        {...icon.attrs}
        width={dimensions}
        height={dimensions}
        {color}
        class={className}
        aria-hidden="true"
    >
        {@html icon.content}
    </svg>
{/snippet}

{#if icon instanceof Promise}
    {#await icon}
        <svg width={dimensions} height={dimensions} class={className} aria-hidden="true"></svg>
    {:then ready}
        {@render glyph(ready)}
    {/await}
{:else}
    {@render glyph(icon)}
{/if}
