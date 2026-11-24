<script lang="ts">
    import { PluginExecutorCommand } from '@remnawave/backend-contract'
    import { createMutation } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { submitVoidCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import type { Node } from '../nodes/queries'
    import { type ExecutorAction, executorCommand } from './config'

    let { nodes, close }: { nodes: Node[]; close: () => void } = $props()
    let t = $derived(locale.t)
    let dialog: HTMLDialogElement
    let action = $state<ExecutorAction>('blockIps')
    let input = $state('')
    let selection = $state<string[]>([])
    let problem = $state('')
    let connected = $derived(nodes.filter((node) => node.isConnected))
    let selected = $derived(connected.filter((node) => selection.includes(node.uuid)))
    let actionLabel = $derived(
        action === 'blockIps'
            ? t('node-plugin-executor.content.block-ips')
            : action === 'unblockIps'
              ? t('node-plugin-executor.content.unblock-ips')
              : t('node-plugin-executor.content.recreate-tables')
    )

    const run = createMutation(() => ({
        mutationFn: (data: PluginExecutorCommand.RequestBody) =>
            submitVoidCommand(PluginExecutorCommand, data),
        onSuccess: () => dialog.close()
    }))

    onMount(() => dialog.showModal())

    function submit(event: SubmitEvent): void {
        event.preventDefault()
        if (!selected.length || run.isPending) return
        run.reset()
        problem = ''
        if (action !== 'recreateTables' && !input.trim()) {
            problem = t('node-plugin-executor.content.enter-at-least-one-ip-address')
            return
        }
        try {
            const command = executorCommand(action, input)
            if (
                !window.confirm(`${actionLabel} · ${selected.map((node) => node.name).join(', ')}?`)
            )
                return
            run.mutate({
                command,
                targetNodes: {
                    target: 'specificNodes',
                    nodeUuids: selected.map((node) => node.uuid)
                }
            })
        } catch (error) {
            problem = error instanceof Error ? error.message : String(error)
        }
    }
</script>

<dialog
    bind:this={dialog}
    aria-labelledby="plugin-executor-title"
    onclose={close}
    oncancel={(event) => {
        if (run.isPending) event.preventDefault()
    }}
>
    <form onsubmit={submit}>
        <h2 id="plugin-executor-title">Plugin executor</h2>
        <label>
            {t('common.action.select')}
            <select
                bind:value={action}
                onchange={() => {
                    input = ''
                    problem = ''
                    run.reset()
                }}
                disabled={run.isPending}
            >
                <option value="blockIps">{t('node-plugin-executor.content.block-ips')}</option>
                <option value="unblockIps">{t('node-plugin-executor.content.unblock-ips')}</option>
                <option value="recreateTables">
                    {t('node-plugin-executor.content.recreate-tables')}
                </option>
            </select>
        </label>
        <p>{t('node-plugin-executor.content.executor-description')}</p>
        {#if action !== 'recreateTables'}
            <label>
                {action === 'blockIps'
                    ? t('node-plugin-executor.content.ips-to-block')
                    : t('node-plugin-executor.content.ips-to-unblock')}
                <textarea
                    bind:value={input}
                    rows="5"
                    placeholder={action === 'blockIps' ? 'IP;timeout' : 'IP'}
                    disabled={run.isPending}
                ></textarea>
            </label>
            <p>
                {action === 'blockIps'
                    ? t('node-plugin-executor.content.block-ips-description')
                    : t('node-plugin-executor.content.unblock-ips-description')}
            </p>
        {/if}
        <fieldset disabled={run.isPending}>
            <legend>{t('constants.nodes')}</legend>
            {#if connected.length}
                <div class="button-row">
                    <button
                        type="button"
                        onclick={() => (selection = connected.map((node) => node.uuid))}
                    >
                        {t('common.action.select-all')}
                    </button>
                    <button type="button" onclick={() => (selection = [])}>
                        {t('node-plugin-executor.content.deselect-all')}
                    </button>
                </div>
                {#each connected as node (node.uuid)}
                    <label class="node"
                        ><input type="checkbox" value={node.uuid} bind:group={selection}>
                        {node.name}</label
                    >
                {/each}
            {:else}
                <p>{t('node-plugin-executor.content.no-connected-nodes-available')}</p>
            {/if}
        </fieldset>
        {#if problem || run.error}
            <p role="alert">{problem || run.error?.message}</p>
        {/if}
        <div class="button-row">
            <button type="submit" disabled={!selected.length || run.isPending}>
                {t('node-plugin-executor.content.execute')}
            </button>
            <button type="button" disabled={run.isPending} onclick={() => dialog.close()}>
                {t('common.action.cancel')}
            </button>
        </div>
    </form>
</dialog>

<style>
    dialog {
        width: min(34rem, 95vw);
    }
    form,
    label {
        display: grid;
        gap: 0.5rem;
    }
    label.node {
        display: flex;
        align-items: center;
    }
    textarea {
        width: 100%;
        resize: vertical;
    }
    h2,
    p {
        margin: 0;
    }
    fieldset {
        min-width: 0;
    }
</style>
