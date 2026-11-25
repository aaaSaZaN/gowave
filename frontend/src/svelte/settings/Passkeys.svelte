<script lang="ts">
    import {
        DeletePasskeyCommand,
        GetPasskeyRegistrationOptionsCommand,
        UpdatePasskeyCommand,
        VerifyPasskeyRegistrationCommand
    } from '@remnawave/backend-contract'
    import {
        type PublicKeyCredentialCreationOptionsJSON,
        startRegistration
    } from '@simplewebauthn/browser'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { callCommand, submitCommand, submitVoidCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import { cancelDialog, dismissDialog } from '../ui/dialog-motion'
    import { passkeys } from './queries'

    let { close }: { close: () => void } = $props()
    let dialog: HTMLDialogElement
    let t = $derived(locale.t)
    let date = $derived(
        new Intl.DateTimeFormat(locale.language, { dateStyle: 'medium', timeStyle: 'short' })
    )
    let renaming = $state<string>()
    let name = $state('')
    const keys = createQuery(() => passkeys)

    type Action =
        | { kind: 'register' }
        | { kind: 'rename'; id: string; name: string }
        | { kind: 'delete'; id: string }
    const action = createMutation(() => ({
        mutationFn: async (value: Action) => {
            if (value.kind === 'rename') {
                await submitCommand(UpdatePasskeyCommand, { id: value.id, name: value.name })
            } else if (value.kind === 'delete') {
                await submitVoidCommand(DeletePasskeyCommand, { id: value.id })
            } else {
                const options = await callCommand(GetPasskeyRegistrationOptionsCommand)
                const response = await startRegistration({
                    optionsJSON: options as PublicKeyCredentialCreationOptionsJSON
                })
                const result = await submitCommand(VerifyPasskeyRegistrationCommand, { response })
                if (!result.verified) throw new Error('The passkey could not be verified.')
            }
        },
        onSuccess: async () => {
            renaming = undefined
            await queryClient.invalidateQueries({ queryKey: passkeys.queryKey })
        }
    }))

    onMount(() => dialog.showModal())
</script>

<dialog
    bind:this={dialog}
    onclose={close}
    aria-labelledby="passkeys-heading"
    oncancel={(event) => cancelDialog(event, action.isPending)}
>
    <h2 id="passkeys-heading">{t('passkeys-drawer.component.passkeys')}</h2>
    {#if keys.isPending}
        <p role="status">{t('common.message.loading')}</p>
    {/if}
    {#if keys.error || action.error}
        <p role="alert">{keys.error?.message || action.error?.message}</p>
    {/if}
    {#if keys.data}
        <ul>
            {#each keys.data.passkeys as key (key.id)}
                <li>
                    <h3>{key.name}</h3>
                    <p>
                        {t('common.field.created-at')} {date.format(new Date(key.createdAt))}
                        · {t('passkeys-drawer.component.last-used-at')}
                        {date.format(new Date(key.lastUsedAt))}
                    </p>
                    {#if renaming === key.id}
                        <form
                            class="button-row"
                            onsubmit={(event) => {
                                event.preventDefault()
                                if (!action.isPending)
                                    action.mutate({ kind: 'rename', id: key.id, name: name.trim() })
                            }}
                        >
                            <label
                                >{t('common.field.name')}
                                <input
                                    name="passkeyName"
                                    required
                                    minlength="2"
                                    maxlength="30"
                                    bind:value={name}
                                ></label
                            >
                            <button type="submit" disabled={action.isPending}>
                                {t('common.action.save')}
                            </button>
                            <button
                                type="button"
                                disabled={action.isPending}
                                onclick={() => {
                                    renaming = undefined
                                    action.reset()
                                }}
                            >
                                {t('common.action.cancel')}
                            </button>
                        </form>
                    {:else}
                        <div class="button-row">
                            <button
                                type="button"
                                disabled={action.isPending}
                                onclick={() => {
                                    action.reset()
                                    renaming = key.id
                                    name = key.name
                                }}
                            >
                                {t('common.action.rename')}
                            </button>
                            <button
                                type="button"
                                disabled={action.isPending}
                                onclick={() => {
                                    if (
                                        window.confirm(
                                            t('common.message.confirm-action-description')
                                        )
                                    )
                                        action.mutate({ kind: 'delete', id: key.id })
                                }}
                            >
                                {t('common.action.delete')}
                            </button>
                        </div>
                    {/if}
                </li>
            {:else}
                <li>{t('passkeys-drawer.component.no-passkeys-registered-yet')}</li>
            {/each}
        </ul>
    {/if}
    <div class="button-row">
        <button
            type="button"
            disabled={action.isPending}
            onclick={() => action.mutate({ kind: 'register' })}
        >
            {t('passkeys-drawer.component.register')}
        </button>
        <button
            type="button"
            disabled={keys.isFetching || action.isPending}
            onclick={() => void keys.refetch()}
        >
            {t('common.action.refresh')}
        </button>
        <button type="button" disabled={action.isPending} onclick={() => dismissDialog(dialog)}>
            {t('common.action.close')}
        </button>
    </div>
</dialog>

<style>
    dialog {
        width: min(48rem, 94vw);
    }
    ul {
        padding: 0;
        list-style: none;
    }
    li {
        padding: 0.65rem 0;
        border-block-start: 1px solid var(--gw-border);
    }
    h3 {
        margin: 0;
        font-size: 1rem;
    }
    form {
        align-items: end;
    }
    label {
        display: grid;
        gap: 0.25rem;
    }
</style>
