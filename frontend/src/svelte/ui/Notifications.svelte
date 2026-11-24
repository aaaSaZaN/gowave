<script lang="ts">
    import { locale } from '../i18n.svelte'
    import ActionIcon from './ActionIcon.svelte'
    import { notifications } from './notification-state.svelte'
</script>

<div class="notifications">
    {#each notifications.visible as notice (notice.id)}
        <div
            class="notice"
            role={notice.color === 'red' ? 'alert' : 'status'}
            style:--notice-color={`var(--mantine-color-${notice.color}-6)`}
        >
            <div>
                <strong>{notice.title}</strong>
                <p>{notice.message}</p>
            </div>
            <ActionIcon
                icon="TbX"
                label={locale.t('common.action.close')}
                size={22}
                iconSize={14}
                subtle
                onclick={() => notifications.hide(notice.id)}
            />
        </div>
    {/each}
</div>

<style>
    .notifications {
        position: fixed;
        z-index: 400;
        inset: var(--mantine-spacing-md) var(--mantine-spacing-md) auto auto;
        width: min(440px, calc(100vw - 2 * var(--mantine-spacing-md)));
        pointer-events: none;
    }

    .notice {
        display: flex;
        position: relative;
        align-items: center;
        justify-content: space-between;
        gap: var(--mantine-spacing-xs);
        margin-bottom: var(--mantine-spacing-md);
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        padding: 12px 12px 12px 22px;
        overflow-wrap: anywhere;
        background: var(--mantine-color-body);
        color: var(--mantine-color-text);
        box-shadow: var(--mantine-shadow-lg);
        font-size: var(--mantine-font-size-sm);
        pointer-events: auto;
    }

    .notice::before {
        position: absolute;
        inset-block: 8px;
        inset-inline-start: 4px;
        width: 6px;
        border-radius: var(--mantine-radius-sm);
        background: var(--notice-color);
        content: "";
    }

    .notice p {
        margin: 0;
        color: var(--mantine-color-dimmed);
    }
</style>
