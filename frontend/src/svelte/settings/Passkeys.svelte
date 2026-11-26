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
    import { callCommand, submitCommand, submitVoidCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import Button from '../ui/Button.svelte'
    import { useConfirmation } from '../ui/confirmation'
    import EmptyState from '../ui/EmptyState.svelte'
    import FormCard from '../ui/FormCard.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import TextInput from '../ui/TextInput.svelte'
    import { passkeys } from './queries'

    const confirmAction = useConfirmation()

    let { close }: { close: () => void } = $props()
    let overlay: ReturnType<typeof Overlay>
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
</script>

<Overlay
    title={t('passkeys-drawer.component.passkeys')}
    icon="TbFingerprint"
    width={640}
    {close}
    preventClose={action.isPending}
    drawer
    bind:this={overlay}
>
    {#if keys.isPending}
        <RouteLoading compact />
    {/if}
    {#if keys.error || action.error}
        <p role="alert">{keys.error?.message || action.error?.message}</p>
    {/if}
    {#if keys.isSuccess && keys.data.passkeys.length === 0}
        <EmptyState
            icon="TbFingerprint"
            label={t('passkeys-drawer.component.no-passkeys-registered-yet')}
            description={t('passkeys-drawer.component.add-passkeys-description')}
        >
            {#snippet actions()}
                {@render register()}
            {/snippet}
        </EmptyState>
    {:else if keys.data}
        <ul>
            {#each keys.data.passkeys as key (key.id)}
                <li>
                    <FormCard title={key.name} icon="TbFingerprint" tone="teal">
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
                                        action.mutate({
                                            kind: 'rename',
                                            id: key.id,
                                            name: name.trim()
                                        })
                                }}
                            >
                                <TextInput
                                    label={t('common.field.name')}
                                    name="passkeyName"
                                    required
                                    minlength={2}
                                    maxlength={30}
                                    bind:value={name}
                                />
                                <Button
                                    type="submit"
                                    variant="soft"
                                    color="teal"
                                    pending={action.isPending}
                                >
                                    {t('common.action.save')}
                                </Button>
                                <Button
                                    disabled={action.isPending}
                                    onclick={() => {
                                        renaming = undefined
                                        action.reset()
                                    }}
                                >
                                    {t('common.action.cancel')}
                                </Button>
                            </form>
                        {:else}
                            <div class="button-row">
                                <Button
                                    disabled={action.isPending}
                                    onclick={() => {
                                        action.reset()
                                        renaming = key.id
                                        name = key.name
                                    }}
                                >
                                    {t('common.action.rename')}
                                </Button>
                                <Button
                                    variant="soft"
                                    color="red"
                                    disabled={action.isPending}
                                    onclick={async () => {
                                        if (
                                            await confirmAction(
                                                t('common.message.confirm-action-description')
                                            )
                                        )
                                            action.mutate({ kind: 'delete', id: key.id })
                                    }}
                                >
                                    {t('common.action.delete')}
                                </Button>
                            </div>
                        {/if}
                    </FormCard>
                </li>
            {/each}
        </ul>
    {/if}
    <div class="button-row footer">
        {#if keys.isError || keys.data?.passkeys.length}
            {@render register()}
        {/if}
        <Button disabled={keys.isFetching || action.isPending} onclick={() => void keys.refetch()}>
            {t('common.action.refresh')}
        </Button>
        <Button disabled={action.isPending} onclick={() => overlay.dismiss()}>
            {t('common.action.close')}
        </Button>
    </div>
</Overlay>

{#snippet register()}
    <Button
        icon="TbFingerprint"
        color="teal"
        variant="soft"
        pending={action.isPending}
        onclick={() => action.mutate({ kind: 'register' })}
    >
        {t('passkeys-drawer.component.register')}
    </Button>
{/snippet}

<style>
    ul {
        display: grid;
        gap: var(--mantine-spacing-sm);
        padding: 0;
        list-style: none;
    }
    p {
        margin: 0;
    }
    .footer {
        margin-block-start: var(--mantine-spacing-md);
        justify-content: flex-end;
    }
    form {
        align-items: end;
    }
</style>
