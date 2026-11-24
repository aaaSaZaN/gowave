<script lang="ts">
    import dayjs from 'dayjs'
    import type { Snippet } from 'svelte'
    import { locale } from '../i18n.svelte'
    import ActionIcon from '../ui/ActionIcon.svelte'
    import Button from '../ui/Button.svelte'
    import Calendar from '../ui/Calendar.svelte'
    import { cancelDialog, dismissDialog } from '../ui/dialog-motion'
    import PageHeader from '../ui/PageHeader.svelte'

    type Range = { start: string; end: string }
    let {
        range,
        page = false,
        hideRefresh = false,
        control,
        busy,
        apply,
        refresh
    }: {
        range: Range
        page?: boolean
        hideRefresh?: boolean
        control: Snippet
        busy: boolean
        apply: (range: Range) => void
        refresh: () => void
    } = $props()
    let t = $derived(locale.t)
    let selecting = $state(false)
    let pending = $state('')
    let today = $state(dayjs())
    let dialog = $state<HTMLDialogElement>()
    const rangeText = $derived(
        `${dayjs(range.start).locale(locale.language).format('DD MMM, YYYY')} – ${dayjs(range.end).locale(locale.language).format('DD MMM, YYYY')}`
    )
    const presets = [
        [3, 'statistic-nodes.component.3-days'],
        [7, 'statistic-nodes.component.7-days'],
        [14, 'statistic-nodes.component.14-days'],
        [30, 'statistic-nodes.component.30-days'],
        [60, 'statistic-nodes.component.60-days'],
        [90, 'statistic-nodes.component.90-days'],
        [180, 'statistic-nodes.component.180-days']
    ] as const

    $effect(() => {
        if (!dialog) return
        dialog.showModal()
        dialog.focus({ preventScroll: true })
        dialog.scrollLeft = 0
    })

    function applyRange(start: string, end: string): void {
        apply(start <= end ? { start, end } : { start: end, end: start })
        dismissDialog(dialog)
    }

    function selectDate(date: string): void {
        if (pending) applyRange(pending, date)
        else pending = date
    }

    function applyPreset(days?: number): void {
        const first = days ? today.subtract(days - 1, 'day') : today.startOf('month')
        applyRange(first.format('YYYY-MM-DD'), today.format('YYYY-MM-DD'))
    }
</script>

{#snippet filters()}
    <div class="usage-actions" class:page-actions={page}>
        {@render control()}
        <div class="range-actions">
            <div class="range">
                <Button
                    icon="TbCalendar"
                    size="md"
                    iconSize={hideRefresh ? 20 : 24}
                    aria-haspopup="dialog"
                    aria-expanded={selecting}
                    onclick={() => {
                        today = dayjs()
                        selecting = true
                    }}
                    >{rangeText}</Button
                >
            </div>
            {#if page}
                <ActionIcon
                    icon="TbRefresh"
                    tone="cyan"
                    label={t('common.action.refresh')}
                    disabled={busy}
                    onclick={refresh}
                />
            {:else if !hideRefresh}
                <ActionIcon
                    icon="TbRefresh"
                    tone="cyan"
                    label={t('common.action.update')}
                    disabled={busy}
                    onclick={() => apply(range)}
                />
            {/if}
        </div>
    </div>
{/snippet}

{#if page}
    <PageHeader title={t('constants.nodes-statistics')} icon="HiChartPie">
        {@render filters()}
    </PageHeader>
{:else}
    <div class="embedded-filters" class:drawer-filters={hideRefresh}>{@render filters()}</div>
{/if}

{#if selecting}
    <dialog
        oncancel={cancelDialog}
        bind:this={dialog}
        class="range-dialog"
        closedby="any"
        tabindex="-1"
        aria-label={rangeText}
        onclose={() => {
            pending = ''
            selecting = false
        }}
    >
        <div class="range-content">
            <Calendar
                layout="range"
                value={range.start}
                end={range.end}
                {pending}
                max={today.format('YYYY-MM-DD')}
                onselect={selectDate}
            >
                {#snippet shortcuts()}
                    <button
                        class="mantine-UnstyledButton-root"
                        type="button"
                        onclick={() => applyPreset()}
                    >
                        {t('statistic-nodes.component.current-month')}
                    </button>
                    {#each presets as [days, label] (days)}
                        <button
                            class="mantine-UnstyledButton-root"
                            type="button"
                            onclick={() => applyPreset(days)}
                        >
                            {t(label)}
                        </button>
                    {/each}
                {/snippet}
            </Calendar>
        </div>
    </dialog>
{/if}

<style>
    .embedded-filters {
        display: flex;
        align-items: center;
        flex-wrap: wrap;
        gap: var(--mantine-spacing-xs);
        margin-bottom: var(--mantine-spacing-md);
    }
    .usage-actions,
    .range-actions {
        display: flex;
        align-items: center;
        justify-content: end;
        gap: var(--mantine-spacing-sm);
        min-width: 0;
    }
    .usage-actions {
        flex-wrap: wrap;
    }
    .drawer-filters {
        margin: 0;
    }
    .drawer-filters .usage-actions {
        width: 100%;
        justify-content: space-between;
        flex-wrap: nowrap;
    }
    .range {
        position: relative;
        min-width: 0;
    }
    .range > :global(button) {
        padding-inline: 10px;
        font-weight: 400;
        color: var(--mantine-color-text);
        max-width: 100%;
    }
    .range > :global(button svg) {
        color: var(--mantine-color-dimmed);
    }
    @media (max-width: 25em) {
        .page-actions {
            flex-direction: column;
            align-items: flex-end;
        }
        .page-actions .range-actions {
            display: flex;
            width: 100%;
        }
        .page-actions .range {
            flex: 1;
        }
        .page-actions .range > :global(button) {
            width: 100%;
        }
    }
    .range-dialog {
        position: fixed;
        inset: 0;
        margin: auto;
        padding: 0;
        max-width: 90vw;
        max-height: 90dvh;
        width: max-content;
        overflow: auto;
        outline: none;
        background: var(--mantine-color-body);
        color: var(--mantine-color-text);
        border: 1px solid rgba(var(--gw-fg-rgb), 0.08);
        border-radius: var(--mantine-radius-md);
        box-shadow: var(--mantine-shadow-xl);
        scrollbar-width: thin;
        scrollbar-color: var(--mantine-primary-color-filled) transparent;
    }
    .range-content {
        width: max-content;
        padding: 12px;
    }
    .range-dialog::backdrop {
        background: rgb(0 0 0 / 60%);
    }
</style>
