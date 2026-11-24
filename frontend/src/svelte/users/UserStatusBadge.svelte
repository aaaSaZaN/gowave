<script lang="ts">
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import type { User } from './queries'

    let { status }: { status: User['status'] } = $props()
    const statuses: Record<User['status'], { color: string; icon: IconName }> = {
        ACTIVE: { color: 'teal', icon: 'PiPulse' },
        DISABLED: { color: 'shaded-gray', icon: 'PiProhibit' },
        EXPIRED: { color: 'red', icon: 'PiClockUser' },
        LIMITED: { color: 'orange', icon: 'PiClockCountdown' }
    }
    const badge = $derived(statuses[status])
</script>

<span
    class="user-status"
    style:--badge-ink={`var(--mantine-color-${badge.color}-4)`}
    style:--badge-edge={`var(--mantine-color-${badge.color}-6)`}
    style:--badge-shade={`var(--mantine-color-${badge.color}-7)`}
>
    <Icon name={badge.icon} size={18} />
    <span>{status}</span>
</span>

<style>
    .user-status {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        height: 26px;
        padding-inline: 12px;
        gap: 6px;
        border: 1px solid color-mix(in srgb, var(--badge-edge) 30%, transparent);
        border-radius: var(--mantine-radius-md);
        color: var(--badge-ink);
        font-size: 13px;
        font-weight: 700;
        letter-spacing: 0.25px;
        background: linear-gradient(
            135deg,
            color-mix(in srgb, var(--badge-edge) 15%, transparent),
            color-mix(in srgb, var(--badge-shade) 10%, transparent)
        );
    }
</style>
