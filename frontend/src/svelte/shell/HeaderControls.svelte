<script lang="ts">
    import { logoutEvents } from '@shared/emitters/emit-logout'
    import { panelUrl } from '@shared/lib/panel-path'
    import { createQuery } from '@tanstack/svelte-query'
    import { fade } from 'svelte/transition'
    import { z } from 'zod'
    import { languages } from '../../app/i18n/languages'
    import { locale } from '../i18n.svelte'
    import { theme } from '../theme.svelte'
    import Icon from '../ui/Icon.svelte'

    let {
        mobile = false,
        footer = false,
        version,
        updateAvailable,
        showRecap,
        showBuild
    }: {
        mobile?: boolean
        footer?: boolean
        version: string
        updateAvailable: boolean
        showRecap: () => void
        showBuild: () => void
    } = $props()

    const github = 'https://github.com/aaaSaZaN/gowave'
    const logo = panelUrl(`favicons/gowave.svg?v=${__BUILD_ID__}`)
    const starsSchema = z.object({ totalStars: z.number().nonnegative() })
    const storeSchema = z.object({
        state: z.object({
            lastUpdateTimestamp: z.number(),
            gowaveInfo: z.object({ starsCount: z.number() })
        }),
        version: z.literal(3)
    })

    function readStars(): { count: number; updatedAt: number } | undefined {
        try {
            const stored = storeSchema.safeParse(
                JSON.parse(localStorage.getItem('updatesStore') ?? 'null')
            )
            if (stored.success && stored.data.state.gowaveInfo.starsCount > 0) {
                return {
                    count: stored.data.state.gowaveInfo.starsCount,
                    updatedAt: stored.data.state.lastUpdateTimestamp
                }
            }
        } catch {
            // Storage can be disabled.
        }
        return undefined
    }

    const cached = readStars()
    const stars = createQuery(() => ({
        queryKey: ['gowave', 'repository-stars'],
        queryFn: async ({ signal }) => {
            const response = await fetch('https://ungh.cc/stars/aaaSaZaN/gowave', {
                signal: AbortSignal.any([signal, AbortSignal.timeout(5000)])
            })
            if (!response.ok) throw new Error('Could not load repository stars')
            return starsSchema.parse(await response.json()).totalStars
        },
        initialData: cached?.count,
        initialDataUpdatedAt: cached?.updatedAt ?? 0,
        enabled: !mobile || footer,
        staleTime: 86_400_000,
        retry: false
    }))

    let languageOpen = $state(false)
    let languageTrigger = $state<HTMLButtonElement>()
    let languageMenu = $state<HTMLDivElement>()
    let languageError = $state('')
    const nextTheme = $derived(theme.scheme === 'dark' ? 'light' : 'dark')

    $effect(() => {
        if (languageOpen) languageMenu?.focus({ preventScroll: true })
    })

    function handleLanguageKeys(event: KeyboardEvent): void {
        if (event.key === 'Escape' && languageOpen) {
            languageOpen = false
            languageTrigger?.focus()
            return
        }
        if (
            !languageOpen ||
            !languageMenu ||
            !['ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)
        )
            return
        const items = [...languageMenu.querySelectorAll<Element>('button')]
        const current = document.activeElement ? items.indexOf(document.activeElement) : -1
        const index =
            event.key === 'End'
                ? items.length - 1
                : event.key === 'Home' || current < 0
                  ? 0
                  : (current + (event.key === 'ArrowUp' ? -1 : 1) + items.length) % items.length
        event.preventDefault()
        const item = items[index]
        if (item instanceof HTMLButtonElement) item.focus()
    }

    function changeLanguage(language: string): void {
        languageOpen = false
        languageTrigger?.focus()
        languageError = ''
        void locale.setLanguage(language).catch((error: unknown) => {
            languageError =
                error instanceof Error ? error.message : locale.t('common.message.error')
        })
    }
</script>

<div class="header-controls" class:mobile class:footer>
    {#if !footer}
        <button
            type="button"
            class="header-control version"
            class:update-available={updateAvailable && version !== 'dev'}
            aria-label="Build info"
            title="Build info"
            disabled={!version}
            onclick={showBuild}
        >
            {#if version}
                <img src={logo} alt="" width="20" height="20"><span>{version}</span>
            {/if}
        </button>
    {/if}
    {#if !mobile || footer}
        <a
            class="header-control github"
            href={github}
            target="_blank"
            rel="noopener noreferrer"
            aria-label="GitHub"
        >
            <Icon name="TbBrandGithub" size={22} />
            {#if stars.isFetching && stars.data === undefined}
                <span class="stars loading">
                    <Icon name="TbStar" size={16} color="gold" />
                    <span class="stars-spinner" aria-hidden="true"></span>
                </span>
            {:else if stars.data !== undefined}
                <span class="stars"><Icon name="TbStar" size={16} color="gold" />{stars.data}</span>
            {/if}
        </a>
    {/if}
    <a
        class="header-control"
        href={__TELEGRAM_CHAT_URL__}
        target="_blank"
        rel="noopener noreferrer"
        aria-label="Telegram"
        title="Telegram"
        ><Icon name="TbBrandTelegram" size={22} /></a
    >
    {#if !mobile}
        <button
            type="button"
            class="header-control"
            aria-label="Recap"
            title="Recap"
            onclick={showRecap}
        >
            <Icon name="TbSparkles" size={22} />
        </button>
    {/if}
    <button
        type="button"
        class="header-control"
        aria-label={`Switch to ${nextTheme} theme`}
        title={`Switch to ${nextTheme} theme`}
        onclick={theme.toggle}
    >
        <Icon name={theme.scheme === 'dark' ? 'PiSun' : 'PiMoon'} size={22} />
    </button>
    {#if !footer}
        <div
            class="language"
            role="presentation"
            onfocusout={(event) => {
                if (
                    !(event.relatedTarget instanceof Node) ||
                    !event.currentTarget.contains(event.relatedTarget)
                )
                    languageOpen = false
            }}
        >
            <button
                type="button"
                class="header-control"
                bind:this={languageTrigger}
                onkeydown={handleLanguageKeys}
                aria-label="Language"
                title="Language"
                aria-haspopup="menu"
                aria-expanded={languageOpen}
                onclick={() => (languageOpen = !languageOpen)}
            >
                <Icon name="TbLanguage" size={22} />
            </button>
            {#if languageOpen}
                <div
                    bind:this={languageMenu}
                    class="language-menu"
                    role="menu"
                    aria-label="Language"
                    tabindex="-1"
                    onkeydown={handleLanguageKeys}
                    transition:fade={{ duration: 150 }}
                >
                    {#each languages as language (language.value)}
                        <button
                            type="button"
                            role="menuitem"
                            tabindex="-1"
                            onclick={() => changeLanguage(language.value)}
                        >
                            <span aria-hidden="true">{language.emoji}</span>{language.label}
                        </button>
                    {/each}
                </div>
            {/if}
        </div>
        <button
            type="button"
            class="header-control logout"
            aria-label="Sign out"
            title="Sign out"
            onclick={() => logoutEvents.emit()}
        >
            <Icon name="PiSignOut" size={22} />
        </button>
    {/if}
</div>
{#if languageError}
    <p role="alert">{languageError}</p>
{/if}

<style>
    .header-controls {
        display: flex;
        flex: 0 0 auto;
        align-items: center;
        gap: var(--mantine-spacing-xs);
    }

    .header-controls.mobile:not(.footer) {
        gap: var(--mantine-spacing-md);
    }

    .footer {
        justify-content: center;
    }

    .header-control {
        display: inline-flex;
        position: relative;
        overflow: hidden;
        width: 44px;
        height: 44px;
        flex-shrink: 0;
        align-items: center;
        justify-content: center;
        padding: 0;
        border: 1px solid light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-4));
        border-radius: var(--mantine-radius-lg);
        color: light-dark(var(--mantine-color-gray-7), var(--mantine-color-white));
        background: linear-gradient(
            135deg,
            light-dark(var(--mantine-color-white), var(--mantine-color-dark-6)),
            light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-7))
        );
        box-shadow:
            0 2px 8px light-dark(rgb(0 0 0 / 5%), rgb(0 0 0 / 20%)),
            0 1px 4px light-dark(rgb(0 0 0 / 3%), rgb(0 0 0 / 10%));
        text-decoration: none;
        transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
    }

    .header-control:hover {
        transform: translateY(-1px);
        color: light-dark(var(--mantine-color-cyan-6), var(--mantine-color-cyan-3));
        border-color: light-dark(var(--mantine-color-cyan-3), var(--mantine-color-cyan-4));
        background: linear-gradient(
            135deg,
            light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-5)),
            light-dark(var(--mantine-color-gray-1), var(--mantine-color-dark-6))
        );
    }

    .header-control.version {
        width: auto;
        gap: 8px;
        min-width: 85px;
        padding-inline: 10px;
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
        font-family: var(--mantine-font-family-monospace);
    }

    .header-control.logout {
        border-color: light-dark(var(--mantine-color-cyan-7), var(--mantine-color-cyan-4));
        color: light-dark(var(--mantine-color-cyan-7), var(--mantine-color-cyan-4));
    }

    :global([data-mantine-color-scheme="dark"]) .header-control.version.update-available {
        border-color: var(--mantine-color-cyan-5);
        color: var(--mantine-color-cyan-5);
        animation: update-glow 3s ease-in-out infinite;
    }

    @keyframes update-glow {
        50% {
            box-shadow:
                0 2px 8px rgb(0 0 0 / 20%),
                0 1px 4px rgb(0 0 0 / 10%),
                0 0 12px 2px rgb(6 182 212 / 30%);
        }
    }

    .version:disabled::after {
        content: "";
        position: absolute;
        inset: 0;
        background: linear-gradient(90deg, transparent, rgba(var(--gw-fg-rgb), 0.06), transparent);
        animation: version-loading 1.5s infinite;
    }
    @keyframes version-loading {
        from {
            transform: translateX(-100%);
        }
        to {
            transform: translateX(100%);
        }
    }
    .github {
        width: auto;
        gap: 8px;
        padding-inline: 10px;
    }

    .stars {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        font-size: var(--mantine-font-size-sm);
        font-weight: 600;
    }

    .stars.loading {
        gap: 8px;
    }

    .stars-spinner {
        width: 18px;
        height: 18px;
        border: 2px solid currentColor;
        border-inline-start-color: transparent;
        border-radius: 50%;
        animation: stars-spin 1s linear infinite;
    }

    @keyframes stars-spin {
        to {
            transform: rotate(1turn);
        }
    }

    @media (prefers-reduced-motion: reduce) {
        .header-control.update-available,
        .stars-spinner {
            animation: none;
        }
    }

    .language {
        position: relative;
    }

    .language-menu {
        position: absolute;
        z-index: 15;
        inset-inline-end: 0;
        inset-block-start: calc(100% + 6px);
        width: 150px;
        outline: none;
        padding: 4px;
        border: 1px solid var(--mantine-color-default-border);
        border-radius: var(--mantine-radius-md);
        background: var(--mantine-color-body);
        box-shadow: var(--mantine-shadow-md);
    }

    .language-menu button {
        display: flex;
        align-items: center;
        gap: 8px;
        width: 100%;
        min-height: 34px;
        padding-inline: 10px;
        border: none;
        border-radius: var(--mantine-radius-sm);
        color: var(--mantine-color-text);
        background: transparent;
        font-size: var(--mantine-font-size-sm);
        text-align: start;
    }

    .language-menu button:hover {
        background: light-dark(var(--mantine-color-gray-1), var(--mantine-color-dark-6));
    }

    @media (max-width: 25rem) {
        .header-controls.mobile:not(.footer) {
            gap: var(--mantine-spacing-xs);
        }
    }
</style>
