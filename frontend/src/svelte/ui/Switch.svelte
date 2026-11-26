<script lang="ts">
    import type { HTMLInputAttributes } from 'svelte/elements'

    let {
        label,
        description,
        showLabel = false,
        switchSize = 'md',
        tone = 'teal',
        roundedSquare = false,
        checked = $bindable(false),
        ...input
    }: HTMLInputAttributes & {
        label: string
        description?: string
        showLabel?: boolean
        switchSize?: 'sm' | 'md' | 'lg'
        tone?: 'cyan' | 'teal'
        roundedSquare?: boolean
    } = $props()
</script>

<label
    class="mantine-Switch-root gw-switch-layout switch"
    class:rounded-square={roundedSquare}
    data-label-position="right"
    title={description ? undefined : label}
    style:--label-lh={`var(--label-lh-${switchSize})`}
    style:--switch-height={`var(--switch-height-${switchSize})`}
    style:--switch-width={`var(--switch-width-${switchSize})`}
    style:--switch-thumb-size={`var(--switch-thumb-size-${switchSize})`}
    style:--switch-track-label-padding={`var(--switch-track-label-padding-${switchSize})`}
    style:--switch-color={tone === 'teal'
        ? 'var(--mantine-color-teal-8)'
        : 'var(--mantine-primary-color-filled)'}
>
    <input
        class="mantine-Switch-input"
        {...input}
        bind:checked
        type="checkbox"
        role="switch"
        aria-checked={!!checked}
        aria-label={description || showLabel ? undefined : label}
        aria-describedby={input['aria-describedby']}
    >
    <span
        class="mantine-Switch-track"
        data-without-labels
        data-label-position="right"
        aria-hidden="true"
    >
        <span class="mantine-Switch-thumb" data-with-thumb-indicator></span>
    </span>
    {#if description || showLabel}
        <span class="gw-switch-copy">
            <span class="gw-switch-label">{label}</span>
            {#if description}
                <span class="mantine-InputWrapper-description gw-switch-description"
                    >{description}</span
                >
            {/if}
        </span>
    {/if}
</label>

<style>
    .switch {
        display: inline-flex;
        align-items: flex-start;
        flex: none;
        cursor: pointer;
    }
    .rounded-square {
        --switch-radius: var(--mantine-radius-md);
    }
    .rounded-square :global(.mantine-Switch-thumb)::before {
        border-radius: 50%;
    }
</style>
