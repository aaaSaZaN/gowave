<script lang="ts">
    import { type IconName, icons } from './icons'

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
    const icon = $derived(icons[name])
</script>

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
