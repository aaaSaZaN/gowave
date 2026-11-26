<script lang="ts">
    import Checkbox from './Checkbox.svelte'
    import type { IconName } from './icons'
    import ThemeIcon from './ThemeIcon.svelte'

    let {
        label,
        icon,
        tone = 'cyan',
        checked,
        disabled = false,
        onchange
    }: {
        label: string
        icon: IconName
        tone?: string
        checked: boolean
        disabled?: boolean
        onchange: (checked: boolean) => void
    } = $props()

    const id = $props.id()
</script>

<label for={id} class:checked>
    <ThemeIcon {icon} {tone} />
    <span>{label}</span>
    <Checkbox
        {id}
        {checked}
        {disabled}
        onchange={(event) => onchange(event.currentTarget.checked)}
    />
</label>

<style>
    label {
        display: grid;
        grid-template-columns: auto minmax(0, 1fr) auto;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        min-height: 56px;
        padding: var(--mantine-spacing-xs) var(--mantine-spacing-sm);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        color: var(--mantine-color-bright);
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
        cursor: pointer;
    }

    .checked {
        border-color: var(--mantine-primary-color-filled);
        background: var(--mantine-primary-color-light);
    }

    label:has(:disabled) {
        opacity: 0.6;
        cursor: not-allowed;
    }
</style>
