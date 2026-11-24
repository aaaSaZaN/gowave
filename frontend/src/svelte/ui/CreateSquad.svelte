<script lang="ts">
    import {
        CreateExternalSquadCommand,
        CreateInternalSquadCommand
    } from '@remnawave/backend-contract'
    import { createMutation } from '@tanstack/svelte-query'
    import { submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import Button from './Button.svelte'
    import Overlay from './Overlay.svelte'

    let {
        kind,
        close,
        oncreated
    }: {
        kind: 'internal' | 'external'
        close: () => void
        oncreated: (uuid: string) => void
    } = $props()
    let name = $state('')
    const t = $derived(locale.t)
    const parsed = $derived(
        kind === 'internal'
            ? CreateInternalSquadCommand.RequestBodySchema.safeParse({
                  name: name.trim(),
                  inbounds: []
              })
            : CreateExternalSquadCommand.RequestBodySchema.safeParse({ name: name.trim() })
    )
    const create = createMutation(() => ({
        mutationFn: async () => {
            const created =
                kind === 'internal'
                    ? await submitCommand(CreateInternalSquadCommand, {
                          name: name.trim(),
                          inbounds: []
                      })
                    : await submitCommand(CreateExternalSquadCommand, { name: name.trim() })
            return created.uuid
        },
        onSuccess: async (uuid) => {
            await queryClient.invalidateQueries({
                queryKey: [kind === 'internal' ? 'internalSquads' : 'externalSquads']
            })
            close()
            oncreated(uuid)
        }
    }))
</script>

<Overlay
    title={t('common.field.creation')}
    icon="TbNewSection"
    tone="teal"
    width={440}
    fullscreenOnMobile={false}
    preventClose={create.isPending}
    {close}
>
    <form
        onsubmit={(event) => {
            event.preventDefault()
            if (parsed.success && !create.isPending) create.mutate()
        }}
    >
        <label>
            <span
                >{kind === 'internal'
                    ? t('internal-squad-header-action-buttons.feature.squad-name')
                    : t('header-action-buttons.feature.external-squad-name')}
                <span class="required">*</span></span
            >
            <input
                name="name"
                bind:value={name}
                required
                maxlength="30"
                data-autofocus
                placeholder={kind === 'internal'
                    ? t('internal-squad-header-action-buttons.feature.enter-squad-name')
                    : 'My Awesome Squad'}
            >
        </label>
        {#if name && !parsed.success}
            <p role="alert">{parsed.error.issues[0]?.message}</p>
        {/if}
        {#if create.error}
            <p role="alert">{create.error.message}</p>
        {/if}
        <div class="button-row">
            <Button
                color="gray"
                variant={kind === 'internal' ? 'soft' : 'subtle'}
                onclick={close}
                disabled={create.isPending}
                >{t('common.action.cancel')}</Button
            >
            <Button
                type="submit"
                color="teal"
                variant={kind === 'internal' ? 'default' : 'soft'}
                disabled={!parsed.success || create.isPending}
                >{t('common.action.create')}</Button
            >
        </div>
    </form>
</Overlay>

<style>
    form {
        display: grid;
        gap: var(--mantine-spacing-md);
    }
    label {
        display: grid;
        font-size: var(--mantine-font-size-sm);
        font-weight: 500;
        line-height: var(--mantine-line-height-md);
    }
    input {
        width: 100%;
        height: 36px;
        min-height: 36px;
        font-size: var(--mantine-font-size-sm);
        font-weight: 400;
    }
    input:focus-visible {
        outline: none;
        border-color: var(--mantine-primary-color-filled);
    }
    .required {
        color: var(--mantine-color-error);
    }
    p {
        margin: 0;
        font-size: var(--mantine-font-size-xs);
    }
    .button-row {
        justify-content: flex-end;
        gap: var(--mantine-spacing-md);
    }
</style>
