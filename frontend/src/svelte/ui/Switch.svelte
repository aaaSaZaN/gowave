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
    class="m_5f93f3bb m_5f75b09e mantine-Switch-root switch"
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
        class="m_926b4011 mantine-Switch-input"
        {...input}
        bind:checked
        type="checkbox"
        role="switch"
        aria-checked={!!checked}
        aria-label={description || showLabel ? undefined : label}
        aria-describedby={input['aria-describedby']}
    >
    <span
        class="m_9307d992 mantine-Switch-track"
        data-without-labels
        data-label-position="right"
        aria-hidden="true"
    >
        <span class="m_93039a1d mantine-Switch-thumb" data-with-thumb-indicator></span>
    </span>
    {#if description || showLabel}
        <span class="m_d3ea56bb">
            <span class="m_8ee546b8">{label}</span>
            {#if description}
                <span class="m_fe47ce59 m_328f68c0">{description}</span>
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
