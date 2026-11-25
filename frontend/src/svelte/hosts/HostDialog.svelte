<script lang="ts">
    import { CloneHostCommand, DeleteHostCommand } from '@remnawave/backend-contract'
    import { buildOpenEntityUrl, OPEN_ENTITY } from '@shared/constants/routes'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { executeCommand, submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import HostConfirmation from './HostConfirmation.svelte'
    import HostEditor from './HostEditor.svelte'
    import { hostQuery, hostsKey } from './queries'

    let { uuid, close }: { uuid?: string | undefined; close: () => void } = $props()
    const t = $derived(locale.t)
    const record = createQuery(() => ({ ...hostQuery(uuid ?? ''), enabled: Boolean(uuid) }))
    let editor: ReturnType<typeof HostEditor> | undefined = $state()
    let confirmation = $state<'clone' | 'delete'>()
    let copied = $state(false)
    let copyError = $state('')
    let host = $derived(record.data)
    const mutation = createMutation(() => ({
        mutationFn: async (action: 'clone' | 'delete') => {
            if (!uuid) return
            if (action === 'clone') {
                await submitCommand(CloneHostCommand, { cloneFromUuid: uuid })
            } else {
                const command = DeleteHostCommand
                await executeCommand({ ...command, TSQ_url: command.url(uuid) })
            }
        },
        onSuccess: async () => {
            await queryClient.invalidateQueries({ queryKey: hostsKey })
            close()
        }
    }))

    function copyLink(): void {
        if (!uuid) return
        copyError = ''
        void navigator.clipboard.writeText(buildOpenEntityUrl(OPEN_ENTITY.HOST, uuid)).then(
            () => (copied = true),
            (error: unknown) => (copyError = error instanceof Error ? error.message : String(error))
        )
    }
</script>

<Overlay
    drawer
    largePadding
    inlineActions
    wrapActions
    class={uuid ? 'host-details' : ''}
    width={700}
    title={host?.remark ??
        (uuid ? t('edit-host-modal.widget.edit-host') : t('create-host-modal.widget.new-host'))}
    subtitle={uuid}
    icon="PiListChecks"
    tone="teal"
    {close}
>
    {#snippet actions()}
        {#if uuid}
            <ActionIcon
                icon={copied ? 'TbCheck' : 'TbLink'}
                label={t('common.action.copy-link')}
                subtle
                size={28}
                iconSize={16}
                onclick={copyLink}
            />
        {/if}
        {#if !uuid || host}
            <Button size="xs" icon="PiEye" variant="light" onclick={() => editor?.showPreview()}
                >{t('host-preview.open')}</Button
            >
        {/if}
    {/snippet}
    {#if uuid && record.isPending}
        <RouteLoading compact />
    {:else if uuid && record.error}
        <p role="alert">{record.error.message}</p>
        <Button onclick={() => void record.refetch()}>{t('common.action.try-again')}</Button>
    {:else}
        <HostEditor bind:this={editor} {host} onsaved={close}>
            {#snippet footer()}
                {#if host}
                    <div class="actions">
                        <ActionIcon
                            icon="TbCopy"
                            tone="cyan"
                            size={44}
                            label={t('common.action.clone')}
                            disabled={mutation.isPending ||
                                !host.inbound.configProfileInboundUuid ||
                                !host.inbound.configProfileUuid}
                            onclick={() => (confirmation = 'clone')}
                        />
                        <ActionIcon
                            icon="PiTrashDuotone"
                            tone="red"
                            size={44}
                            label={t('common.action.delete')}
                            disabled={mutation.isPending}
                            onclick={() => (confirmation = 'delete')}
                        />
                    </div>
                {/if}
            {/snippet}
        </HostEditor>
    {/if}
    {#if mutation.error || copyError}
        <p role="alert">{mutation.error?.message || copyError}</p>
    {/if}
</Overlay>

{#if confirmation}
    {@const action = confirmation}
    {@const label = t(action === 'delete' ? 'common.action.delete' : 'common.action.clone')}
    <HostConfirmation
        title={label}
        {label}
        icon={action === 'delete' ? 'PiTrashDuotone' : 'TbCopy'}
        tone={action === 'delete' ? 'red' : 'cyan'}
        description={t('common.message.confirm-action-description')}
        close={() => (confirmation = undefined)}
        confirm={() => {
            confirmation = undefined
            mutation.mutate(action)
        }}
    />
{/if}

<style>
    :global(.host-details .overlay-header) {
        gap: 0;
    }
    :global(.host-details .inline-actions) {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }
    .actions {
        display: flex;
        gap: var(--mantine-spacing-xs);
    }
</style>
