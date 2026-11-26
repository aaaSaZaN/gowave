<script lang="ts">
    import type { Snippet } from 'svelte'
    import type { ClassValue } from 'svelte/elements'
    import Icon from './Icon.svelte'
    import type { IconName } from './icons'

    let {
        id,
        label,
        description,
        error,
        required,
        leftIcon,
        rightSection,
        children,
        class: className
    }: {
        id: string
        label?: string | undefined
        description?: string | undefined
        error?: string | undefined
        required?: boolean | undefined
        leftIcon?: IconName | undefined
        rightSection?: Snippet | undefined
        children: Snippet
        class?: ClassValue | undefined
    } = $props()
</script>

<div class={['field', className]}>
    {#if label}
        <label for={id}>
            {label}
            {#if required}
                <span class="required" aria-hidden="true"> *</span>
            {/if}
        </label>
    {/if}
    {#if description}
        <div class="description" id={`${id}-description`}>{description}</div>
    {/if}
    <div class="control" class:with-left={!!leftIcon} class:with-right={!!rightSection}>
        {#if leftIcon}
            <span class="left" aria-hidden="true"><Icon name={leftIcon} size={16} /></span>
        {/if}
        {@render children()}
        {#if rightSection}
            <div class="right">{@render rightSection()}</div>
        {/if}
    </div>
    {#if error}
        <div class="error" id={`${id}-error`}>{error}</div>
    {/if}
</div>

<style>
    .field {
        min-width: 0;
        font-size: var(--mantine-font-size-sm);
    }

    label {
        font-weight: 600;
    }

    .description,
    .error {
        font-size: var(--mantine-font-size-xs);
    }

    .description {
        color: var(--mantine-color-dimmed);
    }

    .required,
    .error {
        color: var(--mantine-color-error);
    }

    .control {
        position: relative;
        min-width: 0;
    }

    .control :global(.field-input) {
        width: 100%;
    }

    .control :global(.field-input:focus) {
        border-color: var(--mantine-primary-color-filled);
    }

    .control :global(.field-input[aria-invalid="true"]) {
        border-color: var(--mantine-color-error);
    }

    .control :global(.field-input:disabled) {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .with-left :global(.field-input) {
        padding-inline-start: 36px;
    }

    .with-right :global(.field-input) {
        padding-inline-end: var(--field-right-width, 36px);
    }

    .left,
    .right {
        position: absolute;
        inset-block: 0;
        display: grid;
        place-items: center;
        color: var(--mantine-color-dimmed);
    }

    .left {
        inset-inline-start: 0;
        width: 36px;
        pointer-events: none;
    }

    .right {
        inset-inline-end: 0;
        width: var(--field-right-width, 36px);
    }
</style>
