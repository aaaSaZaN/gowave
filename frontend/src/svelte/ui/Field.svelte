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
        multiline = false,
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
        multiline?: boolean
        children: Snippet
        class?: ClassValue | undefined
    } = $props()
</script>

<div class={['mantine-InputWrapper-root field', className]}>
    {#if label}
        <label for={id} class="mantine-InputWrapper-label">
            {label}
            {#if required}
                <span class="mantine-InputWrapper-required" aria-hidden="true"> *</span>
            {/if}
        </label>
    {/if}
    {#if description}
        <div class="mantine-InputWrapper-description" id={`${id}-description`}>{description}</div>
    {/if}
    <div
        class="mantine-Input-wrapper control"
        data-variant="default"
        data-with-left-section={!!leftIcon || undefined}
        data-with-right-section={!!rightSection || undefined}
        data-multiline={multiline || undefined}
        data-error={!!error || undefined}
    >
        {#if leftIcon}
            <span class="mantine-Input-section" data-position="left" aria-hidden="true"
                ><Icon name={leftIcon} size={16} /></span
            >
        {/if}
        {@render children()}
        {#if rightSection}
            <div class="mantine-Input-section" data-position="right">{@render rightSection()}</div>
        {/if}
    </div>
    {#if error}
        <div class="mantine-InputWrapper-error" id={`${id}-error`}>{error}</div>
    {/if}
</div>

<style>
    .field {
        min-width: 0;
    }

    .control {
        --input-fz: var(--mantine-font-size-sm);
        --input-radius: var(--mantine-radius-md);
        --input-right-section-width: var(--field-right-width, 36px);
        --input-right-section-pointer-events: auto;
        --input-left-section-pointer-events: none;
    }

    .mantine-InputWrapper-description + .control {
        margin-block-start: 5px;
    }

    .control :global(.mantine-Input-input[aria-invalid="true"]) {
        --input-bd: var(--mantine-color-error);
    }

    .control[data-multiline] {
        --input-padding-y: var(--input-padding-y-sm);
        --input-resize: vertical;
    }
</style>
