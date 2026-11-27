<script lang="ts">
    import type { ComponentProps } from 'svelte'
    import type { HTMLInputAttributes } from 'svelte/elements'
    import Field from './Field.svelte'

    type Props = Omit<HTMLInputAttributes, 'value' | 'type'> &
        Pick<
            ComponentProps<typeof Field>,
            'label' | 'description' | 'error' | 'leftIcon' | 'rightSection'
        > & { value?: number | undefined }

    let {
        id: inputId,
        label,
        description,
        error,
        leftIcon,
        rightSection,
        required,
        value = $bindable(undefined),
        class: className,
        ...attributes
    }: Props = $props()

    const generatedId = $props.id()
    const id = $derived(inputId ?? generatedId)
    const describedBy = $derived(
        [attributes['aria-describedby'], description && `${id}-description`, error && `${id}-error`]
            .filter(Boolean)
            .join(' ')
    )
</script>

<Field
    {id}
    {label}
    {description}
    {error}
    {leftIcon}
    {rightSection}
    required={required ?? undefined}
    class={className ?? undefined}
>
    <input
        {...attributes}
        type="number"
        {id}
        {required}
        class="mantine-Input-input"
        aria-invalid={error ? true : attributes['aria-invalid']}
        aria-describedby={describedBy || undefined}
        bind:value
    >
</Field>
