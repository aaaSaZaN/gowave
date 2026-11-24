<script lang="ts">
    import dayjs, { type Dayjs } from 'dayjs'
    import { type Snippet, tick, untrack } from 'svelte'
    import { locale } from '../i18n.svelte'
    import { dateWithinRange, monthDays } from './calendar'
    import Icon from './Icon.svelte'

    let {
        value = $bindable(),
        min,
        max,
        layout = 'compact',
        end,
        pending = '',
        shortcuts,
        onselect
    }: {
        value: string
        min?: string | undefined
        max?: string | undefined
        layout?: 'compact' | 'billing' | 'range' | 'datetime'
        end?: string
        pending?: string
        shortcuts?: Snippet<[choose: (day: Dayjs) => void]>
        onselect?: (day: string) => void
    } = $props()

    const today = dayjs().startOf('day')
    let month = $state(untrack(() => dayjs(value || end || today).startOf('month')))
    let level = $state<'month' | 'year' | 'decade'>('month')
    let hovered = $state('')
    let calendar: HTMLDivElement
    const t = $derived(locale.t)
    const localized = $derived(month.locale(locale.language))
    const decade = $derived(Math.floor(month.year() / 10) * 10)
    const title = $derived(
        level === 'month'
            ? localized.format('MMMM YYYY')
            : level === 'year'
              ? String(month.year())
              : `${decade} – ${decade + 9}`
    )
    const days = $derived(monthDays(localized))
    const months = $derived(Array.from({ length: 12 }, (_, index) => localized.month(index)))
    const years = $derived(Array.from({ length: 10 }, (_, index) => localized.year(decade + index)))
    const range = $derived(layout === 'range')
    const datetime = $derived(layout === 'datetime')
    const anchor = $derived(pending || value)
    const edge = $derived(pending ? hovered || pending : end || value)
    const reversed = $derived(edge < anchor)
    const first = $derived(reversed ? edge : anchor)
    const last = $derived(reversed ? anchor : edge)

    function periodAllowed(first: Dayjs, last: Dayjs): boolean {
        return (
            (!min || last.format('YYYY-MM-DD') >= min) &&
            (!max || first.format('YYYY-MM-DD') <= max)
        )
    }

    function period(direction: number): { head: Dayjs; tail: Dayjs } {
        if (level === 'month') {
            const head = month.add(direction, 'month').startOf('month')
            return { head, tail: head.endOf('month') }
        }
        if (level === 'year') {
            const head = month.add(direction, 'year').startOf('year')
            return { head, tail: head.endOf('year') }
        }
        const head = month.year(decade + direction * 10).startOf('year')
        return { head, tail: head.add(9, 'year').endOf('year') }
    }

    function select(day: Dayjs): void {
        if (!dateWithinRange(day, min, max)) return
        const date = day.format('YYYY-MM-DD')
        if (!range) {
            value = date
            month = day.startOf('month')
        }
        level = 'month'
        onselect?.(date)
    }

    async function moveFocus(event: KeyboardEvent, day: Dayjs): Promise<void> {
        const sign = locale.direction === 'rtl' ? -1 : 1
        const offsets: Record<string, number> = {
            ArrowLeft: -sign,
            ArrowRight: sign,
            ArrowUp: -7,
            ArrowDown: 7
        }
        const offset = offsets[event.key]
        if (offset === undefined) return
        event.preventDefault()
        const moved = day.add(offset, 'day')
        if (!dateWithinRange(moved, min, max)) return
        month = moved.startOf('month')
        await tick()
        const target = calendar.querySelector<HTMLButtonElement>(
            `[data-date="${moved.format('YYYY-MM-DD')}"]`
        )
        target?.focus()
    }
</script>

<div
    class="calendar"
    class:billing={layout === 'billing'}
    class:range
    class:datetime
    bind:this={calendar}
>
    {#if shortcuts}
        <div class="shortcuts">{@render shortcuts(select)}</div>
    {/if}
    <div class="period-picker">
        <div class="calendar-header">
            {#each range || datetime ? [-1, 1, 0] : [-1, 0, 1] as direction (direction)}
                {#if direction === 0}
                    <button
                        class="mantine-UnstyledButton-root period-label"
                        type="button"
                        disabled={level === 'decade'}
                        onclick={() => (level = level === 'month' ? 'year' : 'decade')}
                    >
                        {title}
                    </button>
                {:else}
                    {@const target = period(direction)}
                    <button
                        class="mantine-UnstyledButton-root period-arrow"
                        type="button"
                        aria-label={direction < 0 ? 'Previous' : t('common.action.next')}
                        disabled={!periodAllowed(target.head, target.tail)}
                        onclick={() => (month = target.head)}
                    >
                        <span class:back={direction < 0}
                            ><Icon name="TbChevronRight" size={range ? 18 : 14} /></span
                        >
                    </button>
                {/if}
            {/each}
        </div>
        {#if level === 'month'}
            <fieldset class="days" aria-label={title} onpointerleave={() => (hovered = '')}>
                {#each days.slice(0, 7) as day (day.valueOf())}
                    <span class="weekday">{day.format('dd')}</span>
                {/each}
                {#each days as day (day.valueOf())}
                    {@const iso = day.format('YYYY-MM-DD')}
                    {@const selected = anchor === iso || (range && !pending && edge === iso)}
                    {@const allowed = dateWithinRange(day, min, max)}
                    <button
                        type="button"
                        class="mantine-UnstyledButton-root day"
                        class:outside={!day.isSame(month, 'month')}
                        class:weekend={day.day() === 0 || day.day() === 6}
                        class:today={today.isSame(day, 'day')}
                        class:selected
                        class:in-range={range && iso >= first && iso <= last}
                        class:range-start={range && iso === first}
                        class:range-end={range && iso === last}
                        aria-label={day.format('D MMMM YYYY')}
                        aria-pressed={selected}
                        aria-current={today.isSame(day, 'day') ? 'date' : undefined}
                        data-date={iso}
                        disabled={!allowed}
                        onclick={() => select(day)}
                        onpointerenter={() => {
                            if (pending && allowed) hovered = iso
                        }}
                        onfocus={() => {
                            if (pending) hovered = iso
                        }}
                        onkeydown={(event) => void moveFocus(event, day)}
                    >
                        {day.date()}
                    </button>
                {/each}
            </fieldset>
        {:else}
            <div class="period-options">
                {#each level === 'year' ? months : years as entry (entry.valueOf())}
                    {@const unit = level === 'year' ? 'month' : 'year'}
                    <button
                        type="button"
                        class="mantine-UnstyledButton-root"
                        disabled={!periodAllowed(entry.startOf(unit), entry.endOf(unit))}
                        class:selected={entry.isSame(dayjs(value), unit)}
                        onclick={() => {
                            month = entry.startOf('month')
                            level = level === 'year' ? 'month' : 'year'
                        }}
                    >
                        {entry.format(level === 'year' ? 'MMM' : 'YYYY')}
                    </button>
                {/each}
            </div>
        {/if}
    </div>
</div>

<style>
    .calendar {
        display: grid;
        grid-template-columns: minmax(0, 259px);
        font-size: var(--mantine-font-size-sm);
    }
    .calendar.billing {
        width: min(382px, calc(100% + 18px));
        justify-self: center;
        grid-template-columns: minmax(92px, 1fr) minmax(0, 259px);
        gap: 6px;
    }
    .shortcuts {
        display: flex;
        flex-direction: column;
        align-items: stretch;
        padding-inline-end: 7px;
        border-inline-end: 1px solid
            light-dark(var(--mantine-color-gray-2), var(--mantine-color-dark-5));
    }
    .billing .shortcuts :global(button) {
        min-height: 36px;
        border-radius: var(--mantine-radius-md);
        color: inherit;
        padding-inline: min(0.8em, 2.9vw);
        white-space: nowrap;
    }
    .calendar-header {
        display: grid;
        grid-template-columns: 36px minmax(0, 1fr) 36px;
        gap: 4px;
        margin-block-end: 10px;
    }
    .calendar-header button,
    .period-options button {
        display: grid;
        place-items: center;
        min-height: 36px;
        border-radius: var(--mantine-radius-md);
        color: inherit;
        text-align: center;
    }
    .period-arrow span {
        display: flex;
    }
    .period-arrow .back {
        transform: rotate(180deg);
    }
    .period-label {
        font-weight: 600;
    }
    .days,
    .period-options {
        display: grid;
        grid-template-columns: repeat(7, minmax(0, 1fr));
        gap: 1px;
        padding: 0 0.5px 0.5px;
    }
    .days {
        border: 0;
        margin: 0;
        min-inline-size: 0;
    }
    .period-options {
        grid-template-columns: repeat(3, minmax(0, 1fr));
    }
    .day {
        display: grid;
        place-items: center;
        min-height: 36px;
        border: 2px solid transparent;
        border-radius: var(--mantine-radius-md);
        color: var(--mantine-color-text);
        font-size: calc(36px / 2.8);
    }
    .weekday {
        color: var(--mantine-color-dimmed);
        text-align: center;
        padding-block-end: 6px;
    }
    .weekend {
        color: var(--mantine-color-red-6);
    }
    .outside {
        color: var(--mantine-color-dimmed);
        opacity: 0.5;
    }
    .calendar button:not(:disabled):hover {
        background: var(--mantine-color-default-hover);
    }
    .calendar button:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }
    .day.today:not(.selected) {
        border-color: var(--mantine-color-default-border);
    }
    .selected {
        color: var(--mantine-primary-color-contrast);
        background: var(--mantine-primary-color-filled);
    }
    .billing .day.selected {
        border-color: var(--mantine-primary-color-filled);
        background: transparent;
    }
    .billing .day.today:not(.selected) {
        border-color: var(--mantine-color-red-9);
        border-radius: 50%;
    }
    @media (max-width: 360px) {
        .billing .shortcuts :global(button) {
            white-space: normal;
        }
    }
    .calendar.datetime {
        grid-template-columns: minmax(105px, 121px) minmax(0, 1fr);
        gap: 7px;
    }
    .datetime .shortcuts :global(button) {
        min-height: 36px;
        padding-inline: 11px;
        border-radius: var(--mantine-radius-md);
    }
    .datetime .shortcuts :global(button:hover) {
        background: var(--mantine-color-default-hover);
    }
    .datetime .calendar-header {
        grid-template-columns: repeat(2, minmax(24px, 36px)) minmax(0, 1fr);
        gap: 1px;
    }
    .datetime .period-label {
        justify-content: start;
        padding-inline: 6px;
        text-transform: capitalize;
    }
    .datetime .day {
        font-size: var(--mantine-font-size-xs);
    }
    @media (max-width: 360px) {
        .calendar.datetime {
            grid-template-columns: minmax(0, 1fr);
        }
        .datetime .shortcuts {
            flex-flow: row wrap;
            border-inline-end: 0;
        }
    }
    .calendar.range {
        width: max-content;
        grid-template-columns: max-content 301px;
        gap: 8px;
    }
    .range .shortcuts {
        justify-content: center;
        padding-inline-end: 0.5em;
        font-size: var(--mantine-font-size-md);
    }
    .range .shortcuts :global(button) {
        padding: 0.52em 0.8em;
        border-radius: var(--mantine-radius-default);
        color: inherit;
        white-space: nowrap;
    }
    .range .calendar-header {
        grid-template-columns: 42px 42px minmax(0, 1fr);
        gap: 1px;
        margin-block-end: 8px;
    }
    .range .calendar-header button,
    .range .period-options button {
        min-height: 42px;
    }
    .range .period-label {
        justify-content: end;
        font-size: var(--mantine-font-size-md);
    }
    .range .period-arrow:disabled {
        opacity: 0.2;
    }
    .range .weekday {
        padding-block-end: 10.5px;
        font-size: var(--mantine-font-size-md);
    }
    .range .day {
        min-height: 42px;
        font-size: calc(42px / 2.8);
        border: 0;
        border-radius: var(--mantine-radius-default);
    }
    .range .day:disabled {
        color: var(--mantine-color-disabled-color);
    }
    .range .day.in-range {
        border-radius: 0;
        background: var(--mantine-primary-color-light);
    }
    .range .day.range-start {
        border-start-start-radius: var(--mantine-radius-default);
        border-end-start-radius: var(--mantine-radius-default);
    }
    .range .day.range-end {
        border-start-end-radius: var(--mantine-radius-default);
        border-end-end-radius: var(--mantine-radius-default);
    }
    .range .day.selected {
        background: var(--mantine-primary-color-filled);
    }
    .range .shortcuts :global(button:hover) {
        background: var(--mantine-color-default-hover);
    }
</style>
