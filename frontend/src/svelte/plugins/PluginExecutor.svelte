<script lang="ts">
    import { PluginExecutorCommand } from '@remnawave/backend-contract'
    import { createMutation } from '@tanstack/svelte-query'
    import { submitVoidCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import type { Node } from '../nodes/queries'
    import Button from '../ui/Button.svelte'
    import Checkbox from '../ui/Checkbox.svelte'
    import { useConfirmation } from '../ui/confirmation'
    import FormCard from '../ui/FormCard.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import SelectField from '../ui/SelectField.svelte'
    import Textarea from '../ui/Textarea.svelte'
    import { type ExecutorAction, executorCommand } from './config'

    const confirmAction = useConfirmation()

    let { nodes, close }: { nodes: Node[]; close: () => void } = $props()
    let t = $derived(locale.t)
    let overlay: ReturnType<typeof Overlay>
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
        onSuccess: () => overlay.dismiss()
    }))

    async function submit(event: SubmitEvent): Promise<void> {
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
            const targetNodes = selected
            if (
                !(await confirmAction(
                    `${actionLabel} · ${targetNodes.map((node) => node.name).join(', ')}?`
                ))
            )
                return
            run.mutate({
                command,
                targetNodes: {
                    target: 'specificNodes',
                    nodeUuids: targetNodes.map((node) => node.uuid)
                }
            })
        } catch (error) {
            problem = error instanceof Error ? error.message : String(error)
        }
    }
</script>

<Overlay
    title="Plugin executor"
    icon="TbTerminal"
    width={800}
    largePadding
    {close}
    fullscreenOnMobile={false}
    bind:this={overlay}
    preventClose={run.isPending}
>
    <form onsubmit={submit}>
        <SelectField
            label={t('common.action.select')}
            placeholder=""
            value={action}
            searchable={false}
            clearable={false}
            options={[
                { value: 'blockIps', label: t('node-plugin-executor.content.block-ips') },
                { value: 'unblockIps', label: t('node-plugin-executor.content.unblock-ips') },
                {
                    value: 'recreateTables',
                    label: t('node-plugin-executor.content.recreate-tables')
                }
            ]}
            onchoose={(value) => {
                if (value !== 'blockIps' && value !== 'unblockIps' && value !== 'recreateTables')
                    return
                action = value
                input = ''
                problem = ''
                run.reset()
            }}
            disabled={run.isPending}
        />
        <p>{t('node-plugin-executor.content.executor-description')}</p>
        {#if action !== 'recreateTables'}
            <Textarea
                label={action === 'blockIps'
                    ? t('node-plugin-executor.content.ips-to-block')
                    : t('node-plugin-executor.content.ips-to-unblock')}
                description={action === 'blockIps'
                    ? t('node-plugin-executor.content.block-ips-description')
                    : t('node-plugin-executor.content.unblock-ips-description')}
                bind:value={input}
                rows={5}
                placeholder={action === 'blockIps' ? 'IP;timeout' : 'IP'}
                disabled={run.isPending}
            />
        {/if}
        <FormCard title={t('constants.nodes')} icon="TbServer" tone="indigo">
            {#if connected.length}
                <div class="button-row">
                    <Button
                        disabled={run.isPending}
                        onclick={() => (selection = connected.map((node) => node.uuid))}
                    >
                        {t('common.action.select-all')}
                    </Button>
                    <Button disabled={run.isPending} onclick={() => (selection = [])}>
                        {t('node-plugin-executor.content.deselect-all')}
                    </Button>
                </div>
                {#each connected as node (node.uuid)}
                    <Checkbox
                        disabled={run.isPending}
                        label={node.name}
                        checked={selection.includes(node.uuid)}
                        onchange={(event) => {
                            selection = event.currentTarget.checked
                                ? [...selection, node.uuid]
                                : selection.filter((uuid) => uuid !== node.uuid)
                        }}
                    />
                {/each}
            {:else}
                <p>{t('node-plugin-executor.content.no-connected-nodes-available')}</p>
            {/if}
        </FormCard>
        {#if problem || run.error}
            <p role="alert">{problem || run.error?.message}</p>
        {/if}
        <div class="button-row">
            <Button
                type="submit"
                variant="soft"
                color="teal"
                pending={run.isPending}
                disabled={!selected.length}
            >
                {t('node-plugin-executor.content.execute')}
            </Button>
            <Button disabled={run.isPending} onclick={() => overlay.dismiss()}>
                {t('common.action.cancel')}
            </Button>
        </div>
    </form>
</Overlay>

<style>
    form {
        display: grid;
        gap: var(--mantine-spacing-md);
    }

    p {
        margin: 0;
    }
</style>
