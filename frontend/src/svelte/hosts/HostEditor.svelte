<script lang="ts">
    import {
        CreateHostCommand,
        UpdateHostCommand,
        UpdateManyHostsCommand
    } from '@remnawave/backend-contract'
    import { createMutation } from '@tanstack/svelte-query'
    import type { Snippet } from 'svelte'
    import { SvelteSet } from 'svelte/reactivity'
    import { submitCommand, submitVoidCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import Button from '../ui/Button.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import HostConfirmation from './HostConfirmation.svelte'
    import HostFields from './HostFields.svelte'
    import HostPreview from './HostPreview.svelte'
    import { formText, hostPatch } from './host-fields'
    import { type Host, hostQuery, hostsKey } from './queries'

    let {
        host,
        bulkIds,
        onsaved,
        footer
    }: {
        host: Host | undefined
        bulkIds?: readonly string[]
        onsaved: () => void
        footer?: Snippet
    } = $props()

    let t = $derived(locale.t)
    const changed = new SvelteSet<string>()
    let form: HTMLFormElement
    let previewTarget = $state<{ saved: Host } | { draft: FormData }>()
    let confirmation = $state.raw<{ patch: ReturnType<typeof hostPatch>; summary: string }>()
    let problem = $state('')
    let bulk = $derived(bulkIds !== undefined)

    const save = createMutation(() => ({
        mutationFn: async (patch: ReturnType<typeof hostPatch>): Promise<Host | undefined> => {
            if (bulkIds) {
                await submitVoidCommand(UpdateManyHostsCommand, { ...patch, uuids: [...bulkIds] })
                return undefined
            }
            if (host) return submitCommand(UpdateHostCommand, { ...patch, uuid: host.uuid })
            return submitCommand(CreateHostCommand, patch)
        },
        onSuccess: async (updated) => {
            if (updated) queryClient.setQueryData(hostQuery(updated.uuid).queryKey, updated)
            await queryClient.invalidateQueries({ queryKey: hostsKey })
            changed.clear()
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
            onsaved()
        }
    }))

    export function showPreview(): void {
        if (bulk || previewTarget || !form || save.isPending) return
        previewTarget = host ? { saved: host } : { draft: new FormData(form) }
    }

    function fieldChanged(key: string): void {
        changed.add(key)
        problem = ''
    }

    function changedInput(event: Event): void {
        const target = event.target
        if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement) {
            const [key] = target.name.split(':')
            if (key && key !== 'apply') fieldChanged(key)
        }
    }

    function submit(event: SubmitEvent): void {
        event.preventDefault()
        if (save.isPending || confirmation) return
        const data = new FormData(form)
        if (!host && (!bulk || data.has('apply:inbound')) && !formText(data, 'inbound')) {
            problem = t('common.message.no-inbound-selected')
            form.querySelector<HTMLButtonElement>('.inbound-select')?.focus()
            return
        }
        problem = ''
        save.reset()
        try {
            const patch = hostPatch(data, (key) =>
                bulk ? data.has(`apply:${key}`) : !host || changed.has(key)
            )
            const fields = Object.entries(patch)
            if (!fields.length) {
                if (bulk) problem = t('edit-many-hosts-drawer.no-changes-description')
                return
            }
            if (bulkIds) {
                const summary = fields
                    .map(([key, value]) => `${key}: ${JSON.stringify(value)}`)
                    .join('\n')
                confirmation = { patch, summary }
                return
            }
            save.mutate(patch)
        } catch (error) {
            problem = error instanceof Error ? error.message : String(error)
        }
    }
</script>

<form bind:this={form} onsubmit={submit} oninput={changedInput} onchange={changedInput}>
    <fieldset class="form-fields" disabled={save.isPending}>
        <HostFields {bulk} {host} onfieldchange={fieldChanged} />
    </fieldset>
    <div class="editor-footer" class:with-actions={!!footer && !!host}>
        <Button
            type="submit"
            icon="PiFloppyDiskDuotone"
            color="teal"
            size="md"
            variant="soft"
            disabled={save.isPending || (!bulk && changed.size === 0)}
        >
            {t('common.action.save')}
        </Button>
        {@render footer?.()}
    </div>
</form>

{#if problem || save.error}
    <p role="alert">{problem || save.error?.message}</p>
{/if}

{#if previewTarget}
    <HostPreview target={previewTarget} close={() => (previewTarget = undefined)} />
{/if}

{#if confirmation && bulkIds}
    {@const { patch, summary } = confirmation}
    <HostConfirmation
        title={t('edit-many-hosts-drawer.confirm-title')}
        label={t('common.action.save')}
        icon="PiFloppyDiskDuotone"
        tone="teal"
        description={t('edit-many-hosts-drawer.confirm-description', { count: bulkIds.length })}
        {summary}
        close={() => (confirmation = undefined)}
        confirm={() => {
            confirmation = undefined
            save.mutate(patch)
        }}
    />
{/if}

<style>
    form,
    fieldset {
        display: grid;
        gap: 0.75rem;
        min-width: 0;
    }
    fieldset.form-fields {
        margin: 0;
        padding: 0;
        border: 0;
        gap: var(--mantine-spacing-md);
    }
    .editor-footer {
        position: sticky;
        bottom: -1px;
        z-index: 1;
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin: 4px calc(-1 * var(--mantine-spacing-sm)) calc(-1 * var(--mantine-spacing-sm));
        padding: var(--mantine-spacing-sm) var(--mantine-spacing-lg);
        border-top: 1px solid rgba(var(--gw-fg-rgb), 0.06);
        background: var(--mantine-color-body);
    }
    .with-actions {
        bottom: -6px;
    }
</style>
