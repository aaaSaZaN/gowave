<script lang="ts">
    import { DeleteSharedListCommand, SyncSharedListCommand } from '@remnawave/backend-contract'
    import { buildTree, resolvePath } from '@shared/ui/tree-browser/build-tree'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { submitVoidCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import RouteLoading from '../RouteLoading.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import { useConfirmation } from '../ui/confirmation'
    import Icon from '../ui/Icon.svelte'
    import { notifications } from '../ui/notification-state.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import { sharedListQuery, sharedListsQuery } from './queries'
    import SharedListEditor from './SharedListEditor.svelte'

    const confirmAction = useConfirmation()

    let { close }: { close: () => void } = $props()
    let path = $state<string[]>([])
    let editing = $state<string | null>()
    let search = $state('')
    let t = $derived(locale.t)
    const list = createQuery(() => sharedListsQuery)
    const selected = createQuery(() => sharedListQuery(editing ?? ''))
    let rows = $derived(
        list.data?.sharedLists.filter(({ name, type }) =>
            `${name} ${type}`.toLowerCase().includes(search.trim().toLowerCase())
        ) ?? []
    )

    const tree = $derived(buildTree(rows, ({ name }) => name))
    const level = $derived(resolvePath(tree, path) ?? tree)

    const action = createMutation(() => ({
        mutationFn: ({ kind, name }: { kind: 'delete' | 'sync'; name: string }) =>
            kind === 'delete'
                ? submitVoidCommand(DeleteSharedListCommand, { name })
                : submitVoidCommand(SyncSharedListCommand, { name }),
        onSuccess: async (_data, { kind, name }) => {
            if (kind === 'delete') {
                queryClient.removeQueries({ queryKey: sharedListQuery(name).queryKey })
                await queryClient.invalidateQueries({ queryKey: sharedListsQuery.queryKey })
            }
            notifications.show({
                title: t('common.message.success'),
                message: t('common.message.operation-completed'),
                color: 'teal'
            })
        }
    }))

    async function apply(kind: 'delete' | 'sync', name: string): Promise<void> {
        const question =
            kind === 'delete'
                ? t('shared-lists.modal.delete-description')
                : t('apply-to-nodes.apply-description')
        if (!(await confirmAction(question))) return
        action.mutate({ kind, name })
    }

    function copy(name: string): void {
        void navigator.clipboard.writeText(`ext:${name}`).then(
            () =>
                notifications.show({
                    title: t('common.message.copied'),
                    message: `ext:${name}`,
                    color: 'teal'
                }),
            (error: unknown) =>
                notifications.show({
                    title: t('common.message.error'),
                    message: error instanceof Error ? error.message : String(error),
                    color: 'red'
                })
        )
    }
</script>

<Overlay
    {close}
    width={700}
    icon="TbList"
    tone="indigo"
    title={t('common.field.shared-lists')}
    subtitle={editing === undefined ? t('shared-lists.modal.subtitle') : undefined}
>
    {#if editing === undefined}
        <div class="button-row">
            <input
                class="gw-input"
                type="search"
                aria-label={t('common.action.search')}
                placeholder={t('common.action.search')}
                bind:value={search}
                oninput={() => (path = [])}
            >
            <ActionIcon
                icon="TbRefresh"
                label={t('common.action.refresh')}
                pending={list.isFetching}
                onclick={() => void list.refetch()}
            />
            <ActionIcon
                icon="TbPlus"
                tone="teal"
                label={t('common.action.create')}
                onclick={() => (editing = null)}
            />
        </div>
        {#if list.isPending}
            <RouteLoading compact />
        {/if}
        {#if list.error}
            <p role="alert">{list.error.message}</p>
        {/if}
        {#if list.data}
            {#if path.length}
                <nav class="button-row" aria-label={t('common.field.shared-lists')}>
                    <Button size="xs" variant="subtle" onclick={() => (path = [])}
                        >{t('common.field.shared-lists')}</Button
                    >
                    {#each path as label, index (index)}
                        <Button
                            size="xs"
                            variant="subtle"
                            onclick={() => (path = path.slice(0, index + 1))}
                            >{label}</Button
                        >
                    {/each}
                </nav>
            {/if}
            <ul>
                {#each level as row (row.path)}
                    <li>
                        {#if row.children.length}
                            <Button
                                icon="TbFolder"
                                variant="subtle"
                                onclick={() => path.push(row.label)}
                                >{row.label}
                                · {row.total}</Button
                            >
                        {/if}
                        {#if row.item}
                            {@const item = row.item}
                            <button
                                type="button"
                                class="list-entry"
                                onclick={() => (editing = item.name)}
                            >
                                <Icon name="TbList" size={18} />
                                <span
                                    ><strong>{row.label}</strong><code>ext:{item.name}</code></span
                                >
                            </button>
                            <span class="badge" class:asn={item.type === 'asList'}
                                >{item.type}</span
                            >
                            <small>{item.itemsCount}</small>
                            <ActionIcon
                                icon="TbCopy"
                                label={t('common.action.copy')}
                                size={30}
                                iconSize={16}
                                onclick={() => copy(item.name)}
                            />
                            <ActionIcon
                                icon="TbRocket"
                                label={t('apply-to-nodes.apply')}
                                size={30}
                                iconSize={16}
                                disabled={action.isPending}
                                onclick={() => apply('sync', item.name)}
                            />
                            <ActionIcon
                                icon="PiTrashDuotone"
                                tone="red"
                                label={t('common.action.delete')}
                                size={30}
                                iconSize={16}
                                disabled={action.isPending}
                                onclick={() => apply('delete', item.name)}
                            />
                        {/if}
                    </li>
                {:else}
                    <li>{t('common.message.nothing-found')}</li>
                {/each}
            </ul>
        {/if}
        {#if action.error}
            <p role="alert">{action.error.message}</p>
        {/if}
    {:else if editing === null}
        <SharedListEditor
            existingName={null}
            existingConfig={undefined}
            close={() => (editing = undefined)}
        />
    {:else if selected.data}
        {#key editing}
            <SharedListEditor
                existingName={selected.data.name}
                existingConfig={selected.data.config}
                close={() => (editing = undefined)}
            />
        {/key}
    {:else}
        {#if selected.isPending}
            <RouteLoading compact />
        {/if}
        {#if selected.error}
            <p role="alert">{selected.error.message}</p>
        {/if}
        <Button onclick={() => (editing = undefined)}>
            {t('common.action.cancel')}
        </Button>
    {/if}
</Overlay>

<style>
    .button-row input {
        flex: 1;
        min-width: 0;
    }
    .list-entry {
        display: flex;
        align-items: center;
        gap: 8px;
        flex: 1;
        min-width: 0;
        text-align: start;
        border: 0;
        background: transparent;
        color: inherit;
    }
    .list-entry span {
        min-width: 0;
    }
    .list-entry :is(strong, code) {
        display: block;
        overflow-wrap: anywhere;
    }
    .list-entry code {
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
    }
    .badge {
        border-radius: 999px;
        padding: 2px 6px;
        background: var(--mantine-color-cyan-light);
        color: var(--mantine-color-cyan-light-color);
        font-size: var(--mantine-font-size-xs);
    }
    .badge.asn {
        background: var(--mantine-color-orange-light);
        color: var(--mantine-color-orange-light-color);
    }
    ul {
        list-style: none;
        padding: 0;
        max-height: 35vh;
        overflow: auto;
    }
    li {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-xs);
        flex-wrap: wrap;
        padding: 0.5rem;
        border-block-end: 1px solid var(--gw-border);
    }
</style>
