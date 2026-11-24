<script lang="ts">
    import { GetRemnawaveHealthCommand, GetStatusCommand } from '@remnawave/backend-contract'
    import { hasAuthorizationToken, instance } from '@shared/api/axios'
    import { createQuery, onlineManager } from '@tanstack/svelte-query'
    import { isAxiosError } from 'axios'
    import { onMount } from 'svelte'
    import { fly } from 'svelte/transition'
    import { locale } from '../i18n.svelte'
    import Icon from './Icon.svelte'

    let online = $state(onlineManager.isOnline())
    let failures = 0

    onMount(() => {
        const unsubscribe = onlineManager.subscribe((value) => {
            online = value
        })
        const interceptor = instance.interceptors.response.use(
            (response) => {
                failures = 0
                return response
            },
            (error: unknown) => {
                if (
                    onlineManager.isOnline() &&
                    isAxiosError(error) &&
                    !error.response &&
                    error.code !== 'ERR_CANCELED' &&
                    ++failures >= 3
                ) {
                    failures = 0
                    onlineManager.setOnline(false)
                }
                throw error
            }
        )
        return () => {
            unsubscribe()
            instance.interceptors.response.eject(interceptor)
        }
    })

    createQuery(() => ({
        queryKey: ['connection-probe'],
        enabled: !online,
        networkMode: 'always',
        retry: true,
        retryDelay: (attempt) => Math.min(1_000 * 2 ** attempt, 30_000),
        staleTime: 0,
        gcTime: 0,
        queryFn: async ({ signal }) => {
            const url = hasAuthorizationToken()
                ? GetRemnawaveHealthCommand.TSQ_url
                : GetStatusCommand.TSQ_url
            try {
                await instance.get(url, { signal, timeout: 5_000 })
            } catch (error) {
                if (!isAxiosError(error) || !error.response) throw error
            }
            onlineManager.setOnline(true)
            return true
        }
    }))
</script>

{#if !online}
    <div class="connection-overlay">
        <div class="notice" role="status" transition:fly={{ y: -10, duration: 150 }}>
            <span class="wireless"><Icon name="PiWifiSlash" size={20} /></span>
            <div class="text">
                <strong>{locale.t('connection-status-overlay.connection-lost')}</strong>
                <span class="retry">
                    <span class="spinner" aria-hidden="true"></span>
                    {locale.t('connection-status-overlay.reconnecting')}
                </span>
            </div>
        </div>
    </div>
{/if}

<style>
    .connection-overlay {
        display: flex;
        position: fixed;
        z-index: var(--mantine-z-index-max);
        inset: 16px 0 auto;
        justify-content: center;
        pointer-events: none;
    }
    .notice {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        padding: var(--mantine-spacing-sm);
        background: linear-gradient(135deg, rgb(30 30 35 / 98%), rgb(45 25 25 / 98%));
        border: 1px solid rgb(250 82 82 / 35%);
        border-radius: var(--mantine-radius-md);
        box-shadow: var(--mantine-shadow-xl);
        pointer-events: auto;
    }
    .wireless {
        display: grid;
        width: 34px;
        height: 34px;
        place-items: center;
        color: var(--mantine-color-red-light-color);
        border: 1px solid rgb(250 82 82 / 30%);
        border-radius: var(--mantine-radius-md);
        background: rgb(250 82 82 / 10%);
    }
    .text {
        display: grid;
        gap: 2px;
        font-size: var(--mantine-font-size-sm);
        line-height: var(--mantine-line-height-sm);
    }
    strong {
        font-weight: 600;
    }
    .retry {
        display: flex;
        align-items: center;
        gap: 6px;
        color: var(--mantine-color-dimmed);
        font-size: var(--mantine-font-size-xs);
        line-height: var(--mantine-line-height-xs);
    }
    .spinner {
        width: 12px;
        height: 12px;
        border: 2px solid var(--mantine-color-red-filled);
        border-inline-start-color: transparent;
        border-radius: 50%;
        animation: spin 1s linear infinite;
    }
    @keyframes spin {
        to {
            transform: rotate(1turn);
        }
    }
    @media (prefers-reduced-motion: reduce) {
        .spinner {
            animation: none;
        }
    }
</style>
