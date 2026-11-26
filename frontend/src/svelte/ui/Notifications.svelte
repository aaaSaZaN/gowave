<script lang="ts">
    import { backOut } from 'svelte/easing'
    import { MediaQuery } from 'svelte/reactivity'
    import { fly } from 'svelte/transition'
    import { locale } from '../i18n.svelte'
    import Icon from './Icon.svelte'
    import { notifications } from './notification-state.svelte'

    const reducedMotion = new MediaQuery('(prefers-reduced-motion: reduce)')
    let hovered = $state(false)
    let focused = $state(false)
    $effect(() => notifications.pause(hovered || focused))

    function dismiss(id: number): void {
        focused = false
        if (notifications.visible.length === 1) hovered = false
        notifications.hide(id)
    }
</script>

<div
    class="notifications"
    role="presentation"
    onpointerenter={() => (hovered = true)}
    onpointerleave={() => (hovered = false)}
    onfocusin={() => (focused = true)}
    onfocusout={(event) => {
        focused =
            event.relatedTarget instanceof Node && event.currentTarget.contains(event.relatedTarget)
    }}
>
    {#each notifications.visible as notice (notice.id)}
        <div
            class="mantine-Notification-root notice"
            role={notice.color === 'red' ? 'alert' : 'status'}
            style:--notification-radius="var(--mantine-radius-md)"
            style:--notification-color={`var(--mantine-color-${notice.color}-filled)`}
            transition:fly={{
                x: '100%',
                duration: reducedMotion.current ? 0 : 250,
                easing: backOut
            }}
        >
            <div class="mantine-Notification-body">
                <strong class="mantine-Notification-title">{notice.title}</strong>
                <div class="mantine-Notification-description" data-with-title>
                    {notice.message}
                </div>
            </div>
            <button
                type="button"
                class="gw-close-button mantine-Notification-closeButton mantine-UnstyledButton-root"
                aria-label={locale.t('common.action.close')}
                onclick={() => dismiss(notice.id)}
            >
                <Icon name="TbX" size={16} />
            </button>
        </div>
    {/each}
</div>

<style>
    .notifications {
        position: fixed;
        z-index: 400;
        top: var(--mantine-spacing-md);
        right: var(--mantine-spacing-md);
        width: min(440px, calc(100vw - 2 * var(--mantine-spacing-md)));
        pointer-events: none;
    }
    .notice {
        max-height: 200px;
        overflow-wrap: anywhere;
        pointer-events: auto;
    }
    .notice + .notice {
        margin-top: var(--mantine-spacing-md);
    }
    strong {
        display: block;
    }
</style>
