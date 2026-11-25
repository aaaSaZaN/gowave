<script lang="ts">
    import dayjs, { type Dayjs } from 'dayjs'
    import { untrack } from 'svelte'
    import { locale } from '../i18n.svelte'
    import Button from '../ui/Button.svelte'
    import Calendar from '../ui/Calendar.svelte'
    import Overlay from '../ui/Overlay.svelte'
    import SelectField from '../ui/SelectField.svelte'

    let {
        value,
        close,
        onchoose
    }: {
        value: string
        close: () => void
        onchoose: (value: string) => void
    } = $props()

    const now = dayjs().startOf('minute')
    const minimum = now.format('YYYY-MM-DD')
    const initial = untrack(() => dayjs(value))
    const times = Array.from({ length: 9 }, (_, index) =>
        now
            .startOf('day')
            .add(360 + 90 * index, 'minute')
            .format('HH:mm')
    )
    let selected = $state(initial.isValid() ? initial.format('YYYY-MM-DD') : minimum)
    let time = $state(initial.isValid() ? initial.format('HH:mm') : now.format('HH:mm'))
    let t = $derived(locale.t)
    let validTime = $derived(/^([01]\d|2[0-3]):[0-5]\d$/.test(time))
    let validDate = $derived(selected >= minimum)

    function apply(): void {
        if (validTime && validDate) onchoose(`${selected}T${time}`)
    }

    function preset(date: Dayjs): void {
        const selectedDate = date.isBefore(now) ? now : date
        onchoose(selectedDate.format('YYYY-MM-DDTHH:mm'))
    }
</script>

<Overlay
    title={t('create-user-modal.widget.expiry-date')}
    icon="PiCalendarDuotone"
    hideHeader
    width={418}
    fullscreenOnMobile={false}
    {close}
>
    <Calendar layout="datetime" bind:value={selected} min={minimum}>
        {#snippet shortcuts()}
            {@const base = initial.isValid() && !initial.isBefore(now) ? initial : now}
            <button
                class="mantine-UnstyledButton-root"
                type="button"
                onclick={() => preset(base.add(1, 'month'))}
            >
                {t('create-user-modal.widget.1-month')}
            </button>
            <button
                class="mantine-UnstyledButton-root"
                type="button"
                onclick={() => preset(base.add(3, 'month'))}
            >
                {t('create-user-modal.widget.3-months')}
            </button>
            <button
                class="mantine-UnstyledButton-root"
                type="button"
                onclick={() => preset(base.add(1, 'year'))}
            >
                {t('create-user-modal.widget.1-year')}
            </button>
            <button
                class="mantine-UnstyledButton-root"
                type="button"
                onclick={() => preset(now.year(2099))}
            >
                {t('create-user-modal.widget.2099-year')}
            </button>
        {/snippet}
    </Calendar>
    <div class="time">
        <SelectField
            label={t('create-user-modal.widget.expiry-time')}
            hideLabel
            placeholder="HH:mm"
            autocomplete="suggest"
            clearable={false}
            options={times.map((value) => ({ value, label: value }))}
            bind:value={time}
        />
        <Button
            variant="default"
            icon="TbCheck"
            iconSize={20}
            aria-label={t('common.action.save')}
            disabled={!validTime || !validDate}
            onclick={apply}
        />
    </div>
</Overlay>

<style>
    .time {
        display: grid;
        grid-template-columns: minmax(0, 1fr) 20%;
        gap: var(--mantine-spacing-md);
        margin-block-start: var(--mantine-spacing-md);
    }
    .time :global(input) {
        font-family: var(--mantine-font-family-monospace);
    }
</style>
