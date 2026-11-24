<script lang="ts">
    import {
        DeleteAllUserHwidDevicesCommand,
        DeleteUserHwidDeviceCommand
    } from '@remnawave/backend-contract'
    import { createMutation, createQuery } from '@tanstack/svelte-query'
    import { onMount } from 'svelte'
    import { submitCommand } from '../commands'
    import { locale } from '../i18n.svelte'
    import { queryClient } from '../query-client'
    import { ipDetailsUrl } from '../torrent-reports/queries'
    import { type HwidDevice, userDevicesQuery } from './queries'

    type TextField = 'platform' | 'hwid' | 'requestIp' | 'deviceModel' | 'userAgent'
    type SortField = TextField | 'createdAt' | 'updatedAt'
    let { userId, close }: { userId: number; close: () => void } = $props()
    const title = $props.id()
    let dialog: HTMLDialogElement
    let t = $derived(locale.t)
    let textFields = $derived([
        { name: 'platform', label: t('common.field.platform') },
        { name: 'hwid', label: 'HWID' },
        { name: 'requestIp', label: t('common.field.ip-address') },
        { name: 'deviceModel', label: t('get-hwid-user-devices.feature.model') },
        { name: 'userAgent', label: t('common.field.user-agent') }
    ] as const)
    let sortFields = $derived([
        ...textFields,
        { name: 'createdAt', label: t('get-hwid-user-devices.feature.added') },
        { name: 'updatedAt', label: t('common.field.updated-at') }
    ] as const)
    let filters = $state<Record<TextField, string>>({
        platform: '',
        hwid: '',
        requestIp: '',
        deviceModel: '',
        userAgent: ''
    })
    let sort = $state<SortField>('updatedAt')
    let ascending = $state(false)
    let notice = $state('')
    let copyError = $state('')
    let dates = $derived(
        new Intl.DateTimeFormat(locale.language, { dateStyle: 'medium', timeStyle: 'short' })
    )
    let collator = $derived(new Intl.Collator(locale.language, { numeric: true }))
    let page = $state(0)
    const size = 25
    const devices = createQuery(() => userDevicesQuery(userId))
    let rows = $derived.by(() => {
        const criteria = textFields.map(({ name }) => ({
            name,
            term: filters[name].trim().toLowerCase()
        }))
        return (devices.data?.devices ?? [])
            .filter((device) =>
                criteria.every(
                    ({ name, term }) =>
                        !term || fieldValue(device, name).toLowerCase().includes(term)
                )
            )
            .sort((a, b) => {
                const left = a[sort]
                const right = b[sort]
                const diff =
                    left instanceof Date && right instanceof Date
                        ? left.getTime() - right.getTime()
                        : collator.compare(String(left ?? ''), String(right ?? ''))
                return (ascending ? diff : -diff) || collator.compare(a.hwid, b.hwid)
            })
    })
    let pages = $derived(Math.max(1, Math.ceil(rows.length / size)))
    const removal = createMutation(() => ({
        mutationFn: async (hwid: string | undefined) => {
            const query = userDevicesQuery(userId)
            await queryClient.cancelQueries({ queryKey: query.queryKey })
            const result =
                hwid === undefined
                    ? await submitCommand(DeleteAllUserHwidDevicesCommand, { userId })
                    : await submitCommand(DeleteUserHwidDeviceCommand, { userId, hwid })
            queryClient.setQueryData(query.queryKey, result)
            await queryClient.invalidateQueries({ queryKey: ['hwid'] })
        },
        onSuccess: () => (notice = t('common.message.operation-completed'))
    }))

    onMount(() => dialog.showModal())

    $effect(() => {
        if (page >= pages) page = pages - 1
    })

    function fieldValue(device: HwidDevice, name: TextField): string {
        if (name !== 'deviceModel') return device[name] ?? ''
        return [device.deviceModel, device.osVersion].filter(Boolean).join(' · ')
    }

    function remove(hwid?: string): void {
        if (removal.isPending || !window.confirm(t('common.message.confirm-action-description')))
            return
        notice = ''
        removal.mutate(hwid)
    }

    function copy(value: string): void {
        copyError = ''
        notice = ''
        void navigator.clipboard.writeText(value).then(
            () => (notice = t('common.message.copied')),
            (failure: unknown) =>
                (copyError = failure instanceof Error ? failure.message : t('common.message.error'))
        )
    }
</script>

<dialog class="user-devices" bind:this={dialog} aria-labelledby={title} onclose={close}>
    <div class="header">
        <h2 id={title}>{t('get-hwid-user-devices.feature.hwid-devices')}</h2>
        <button type="button" onclick={() => dialog.close()}>{t('common.action.close')}</button>
    </div>
    <div class="button-row">
        <button
            type="button"
            disabled={devices.isFetching || removal.isPending}
            onclick={() => void devices.refetch()}
        >
            {t('common.action.refresh')}
        </button>
        <button
            type="button"
            disabled={!devices.data?.total || removal.isPending}
            onclick={() => remove()}
        >
            {t('get-hwid-user-devices.feature.delete-all-devices')}
        </button>
    </div>
    {#if devices.isPending}
        <p role="status">{t('common.message.loading')}</p>
    {/if}
    {#if devices.error || removal.error || copyError}
        <p role="alert">{devices.error?.message || removal.error?.message || copyError}</p>
    {/if}
    {#if notice}
        <p role="status">{notice}</p>
    {/if}
    {#if devices.data}
        <p>
            {t('get-hwid-user-devices.feature.devices')}
            {devices.data.total.toLocaleString(locale.language)}
        </p>
        {#if devices.data.devices.length}
            <details>
                <summary>{t('common.action.search')}</summary>
                <div class="filters">
                    {#each textFields as field (field.name)}
                        <label
                            >{field.label}<input
                                type="search"
                                bind:value={filters[field.name]}
                                oninput={() => (page = 0)}
                            ></label
                        >
                    {/each}
                </div>
            </details>
            <div class="button-row sorting">
                <label
                    >Sort by
                    <select bind:value={sort} onchange={() => (page = 0)}>
                        {#each sortFields as field (field.name)}
                            <option value={field.name}>{field.label}</option>
                        {/each}
                    </select>
                </label>
                <label
                    ><input type="checkbox" bind:checked={ascending} onchange={() => (page = 0)}>
                    Ascending</label
                >
            </div>
        {/if}
        {#if rows.length}
            <div class="table-scroll">
                <table>
                    <thead>
                        <tr>
                            {#each sortFields as field (field.name)}
                                <th scope="col">{field.label}</th>
                            {/each}
                            <th scope="col">{t('common.action.actions')}</th>
                        </tr>
                    </thead>
                    <tbody>
                        {#each rows.slice(page * size, (page + 1) * size) as device (device.hwid)}
                            <tr>
                                <td>{device.platform ?? '—'}</td>
                                <td>
                                    <button
                                        type="button"
                                        title={t('common.action.copy')}
                                        onclick={() => copy(device.hwid)}
                                    >
                                        {device.hwid}
                                    </button>
                                </td>
                                <td>
                                    {#if device.requestIp}
                                        <a
                                            href={ipDetailsUrl(device.requestIp)}
                                            target="_blank"
                                            rel="noreferrer"
                                            >{device.requestIp}</a
                                        >
                                    {:else}
                                        —
                                    {/if}
                                </td>
                                <td>{fieldValue(device, 'deviceModel') || '—'}</td>
                                <td>
                                    {#if device.userAgent}
                                        <button
                                            type="button"
                                            title={t('common.action.copy')}
                                            onclick={() => copy(device.userAgent ?? '')}
                                        >
                                            {device.userAgent}
                                        </button>
                                    {:else}
                                        —
                                    {/if}
                                </td>
                                <td>
                                    <time datetime={device.createdAt.toISOString()}
                                        >{dates.format(device.createdAt)}</time
                                    >
                                </td>
                                <td>
                                    <time datetime={device.updatedAt.toISOString()}
                                        >{dates.format(device.updatedAt)}</time
                                    >
                                </td>
                                <td>
                                    <button
                                        type="button"
                                        disabled={removal.isPending}
                                        onclick={() => remove(device.hwid)}
                                    >
                                        {t('get-hwid-user-devices.feature.delete-device')}
                                    </button>
                                </td>
                            </tr>
                        {/each}
                    </tbody>
                </table>
            </div>
            {#if pages > 1}
                <div class="button-row">
                    <button type="button" disabled={page === 0} onclick={() => page--}>
                        Previous
                    </button>
                    <span>{page + 1} / {pages}</span>
                    <button type="button" disabled={page + 1 >= pages} onclick={() => page++}>
                        {t('common.action.next')}
                    </button>
                </div>
            {/if}
        {:else}
            <p>{t('common.message.no-data-available')}</p>
        {/if}
    {/if}
</dialog>

<style>
    .user-devices {
        width: min(72rem, calc(100vw - 2rem));
    }
    .header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 0.75rem;
        margin-block-end: 0.75rem;
    }
    h2 {
        margin: 0;
    }
    .filters {
        display: flex;
        flex-wrap: wrap;
        gap: 0.5rem;
        margin-block: 0.5rem;
    }
    .filters label {
        display: grid;
        gap: 0.25rem;
    }
    .sorting {
        margin-block: 0.5rem;
    }
    .table-scroll {
        overflow-x: auto;
    }
    table {
        width: 100%;
        border-collapse: collapse;
    }
    th,
    td {
        padding: 0.5rem;
        text-align: start;
        border-block-end: 1px solid var(--gw-border);
    }
    td {
        overflow-wrap: anywhere;
    }
</style>
