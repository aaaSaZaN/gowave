<script lang="ts">
    import { createMutation } from '@tanstack/svelte-query'
    import { ZodError } from 'zod'
    import { locale } from '../i18n.svelte'
    import ActionIcon from './ActionIcon.svelte'
    import Overlay from './Overlay.svelte'

    let {
        currentName,
        maxlength = 30,
        rename,
        close
    }: {
        currentName: string
        maxlength?: number
        rename: (name: string) => Promise<void>
        close: () => void
    } = $props()

    const form = $props.id()
    const t = $derived(locale.t)
    let name = $state('')
    const update = createMutation(() => ({ mutationFn: rename, onSuccess: close }))
    const error = $derived(
        update.error instanceof ZodError ? update.error.issues[0]?.message : update.error?.message
    )
</script>

<Overlay
    title={t('common.action.rename')}
    icon="PiPencil"
    tone="teal"
    width={440}
    fullscreenOnMobile={false}
    preventClose={update.isPending}
    {close}
>
    {#snippet actions()}
        <ActionIcon
            icon="PiFloppyDisk"
            label={t('common.action.save')}
            tone="teal"
            type="submit"
            {form}
            disabled={!name.trim() || update.isPending}
        />
    {/snippet}

    <form
        id={form}
        onsubmit={(event) => {
            event.preventDefault()
            if (!update.isPending && name.trim()) update.mutate(name.trim())
        }}
    >
        <input
            class="mantine-Input-input"
            name="name"
            aria-label={t('common.field.name')}
            placeholder={currentName}
            bind:value={name}
            required
            {maxlength}
            data-autofocus
        >
        {#if error}
            <p role="alert">{error}</p>
        {/if}
    </form>
</Overlay>

<style>
    input {
        width: 100%;
    }
</style>
