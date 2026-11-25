<script lang="ts">
    import type { HTMLInputAttributes } from 'svelte/elements'

    interface Props extends HTMLInputAttributes {
        label: string
        fieldSize?: 'sm' | 'md'
        secret?: boolean
        value: string
        error?: string | undefined
    }

    let {
        label,
        fieldSize = 'sm',
        secret = false,
        value = $bindable(),
        error,
        class: className,
        id: fieldId,
        required = true,
        ...attrs
    }: Props = $props()

    const generatedId = $props.id()
    const id = $derived(fieldId ?? generatedId)
    let show = $state(false)
</script>

<div
    class={[
        'm_46b77525 mantine-InputWrapper-root',
        secret && 'm_f61ca620 mantine-PasswordInput-root',
        className
    ]}
    data-size={fieldSize}
    style:--psi-icon-size={`var(--psi-icon-size-${fieldSize})`}
    style:--psi-button-size={`var(--psi-button-size-${fieldSize})`}
>
    <label
        for={id}
        class="m_8fdc1311 mantine-InputWrapper-label"
        data-size={fieldSize}
        data-required={required || undefined}
        style:--input-label-size={`var(--mantine-font-size-${fieldSize})`}
    >
        {label}
        {#if required}
            <span class="m_78a94662 mantine-InputWrapper-required" aria-hidden="true"> *</span>
        {/if}
    </label>
    <div
        class="m_6c018570 mantine-Input-wrapper"
        data-variant="default"
        data-size={fieldSize}
        data-with-right-section={secret || undefined}
        style:--input-height={`var(--input-height-${fieldSize})`}
        style:--input-fz={`var(--mantine-font-size-${fieldSize})`}
        style:--input-radius="var(--mantine-radius-md)"
    >
        {#if secret}
            <div
                class="m_ccf8da4c m_8fb7ebe7 mantine-Input-input mantine-PasswordInput-input"
                data-variant="default"
            >
                <input
                    {...attrs}
                    {id}
                    {required}
                    bind:value
                    class="m_f2d85dd2 mantine-PasswordInput-innerInput"
                    type={show ? 'text' : 'password'}
                    aria-invalid={Boolean(error) || undefined}
                    aria-describedby={error ? `${id}-error` : undefined}
                >
            </div>
            <div class="m_82577fc2 mantine-Input-section" data-position="right">
                <button
                    type="button"
                    class="m_b1072d44 mantine-PasswordInput-visibilityToggle m_8d3f4000 mantine-ActionIcon-root m_87cf2631"
                    data-variant="subtle"
                    aria-pressed={show}
                    aria-label="Toggle password visibility"
                    onclick={() => (show = !show)}
                >
                    <span class="m_8d3afb97 mantine-ActionIcon-icon">
                        <svg
                            aria-hidden="true"
                            viewBox="0 0 256 256"
                            fill="none"
                            stroke="currentColor"
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="16"
                        >
                            {#if show}
                                <path
                                    d="m48 40 160 176M41 113c-17 18-25 31-25 31s32 72 112 72c26 0 47-8 65-18M78 69a108 108 0 0 1 50-13c80 0 112 72 112 72s-8 18-28 38"
                                />
                            {:else}
                                <path
                                    d="M128 56c-80 0-112 72-112 72s32 72 112 72 112-72 112-72-32-72-112-72z"
                                />
                                <circle cx="128" cy="128" r="40" />
                            {/if}
                        </svg>
                    </span>
                </button>
            </div>
        {:else}
            <input
                {...attrs}
                {id}
                {required}
                bind:value
                class="m_8fb7ebe7 mantine-Input-input"
                data-variant="default"
                aria-invalid={Boolean(error) || undefined}
                aria-describedby={error ? `${id}-error` : undefined}
            >
        {/if}
    </div>
    {#if error}
        <div id={`${id}-error`} class="m_8f816625 mantine-InputWrapper-error" role="alert">
            {error}
        </div>
    {/if}
</div>

<style>
    button {
        --ai-radius: var(--mantine-radius-md);
        --ai-bg: transparent;
        --ai-hover: var(--mantine-color-gray-light-hover);
        --ai-color: var(--mantine-color-gray-light-color);
        --ai-bd: 1px solid transparent;
    }

    svg {
        width: var(--psi-icon-size);
        height: var(--psi-icon-size);
    }
</style>
