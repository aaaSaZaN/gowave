<script lang="ts">
    import { createQuery } from '@tanstack/svelte-query'
    import type { Snippet } from 'svelte'
    import { MediaQuery } from 'svelte/reactivity'
    import { layout } from './layout.svelte'
    import RouteLoading from './RouteLoading.svelte'
    import Recap from './recap/Recap.svelte'
    import { navigationPending, route } from './router'
    import Brand from './shell/Brand.svelte'
    import HeaderControls from './shell/HeaderControls.svelte'
    import shell from './shell/layout.module.css'
    import Navigation from './shell/Navigation.svelte'
    import BuildInfo from './updates/BuildInfo.svelte'
    import { metadataQuery, updateCheckQuery } from './updates/queries'

    let { children }: { children: Snippet } = $props()
    const mobile = new MediaQuery('(max-width: 64rem)')
    const wide = new MediaQuery('(min-width: 128rem)')
    const reducedMotion = new MediaQuery('(prefers-reduced-motion: reduce)')
    const sidebar = $derived(mobile.current || layout.legacy)
    let showMobileNav = $state(false)
    let showDesktopNav = $state(true)
    let showBuild = $state(false)
    let showRecap = $state(false)
    const navOpen = $derived(mobile.current ? showMobileNav : showDesktopNav)
    const metadata = createQuery(() => metadataQuery)
    const update = createQuery(() => updateCheckQuery())
    let page = $state<HTMLDivElement>()

    $effect(() => {
        if (!route.pathname || !page || reducedMotion.current) return
        const entry = page.animate([{ opacity: 0 }, { opacity: 1 }], {
            id: 'dashboard-page-entry',
            duration: 300,
            easing: 'ease-in-out'
        })
        return () => entry.cancel()
    })

    function toggleNavigation(): void {
        if (mobile.current) showMobileNav = !showMobileNav
        else showDesktopNav = !showDesktopNav
    }
</script>

<svelte:window
    onkeydown={(event) => {
        if (event.key === 'Escape') showMobileNav = false
    }}
/>

<div
    class={['dashboard', shell['appShellFadeIn']]}
    class:sidebar
    class:mobile={mobile.current}
    class:wide={wide.current && !sidebar}
    class:sidebar-pinned={sidebar && !mobile.current && showDesktopNav}
>
    <header class={['dashboard-header', shell['header']]}>
        <div class={shell['brandRow']}>
            {#if sidebar}
                <button
                    type="button"
                    class="mantine-Burger-root mantine-UnstyledButton-root"
                    aria-label="Navigation"
                    aria-controls="dashboard-sidebar"
                    aria-expanded={navOpen}
                    style:--burger-size="var(--burger-size-md)"
                    onclick={toggleNavigation}
                >
                    <span class="mantine-Burger-burger" data-opened={navOpen || undefined}></span>
                </button>
            {:else}
                <div class="brand-group">
                    <Brand />
                    {#if wide.current}
                        <div class="brand-divider"></div>
                        <Navigation />
                    {/if}
                </div>
            {/if}
            <HeaderControls
                mobile={mobile.current}
                version={metadata.data?.version ?? ''}
                updateAvailable={Boolean(update.data?.hasUpdate)}
                showBuild={() => (showBuild = true)}
                showRecap={() => (showRecap = true)}
            />
        </div>
        {#if !sidebar && !wide.current}
            <div class={shell['navRowDesktop']}><Navigation /></div>
        {/if}
    </header>

    {#if sidebar}
        {#if mobile.current && navOpen}
            <button
                class="sidebar-dismiss"
                type="button"
                aria-label="Close navigation"
                tabindex="-1"
                onclick={() => (showMobileNav = false)}
            ></button>
        {/if}
        <aside
            id="dashboard-sidebar"
            class={['dashboard-sidebar', shell['sidebarWrapper']]}
            class:nav-open={navOpen}
            inert={!navOpen}
            aria-hidden={!navOpen}
        >
            <div class={['sidebar-brand', shell['logoSection']]}>
                {#if mobile.current}
                    <button
                        type="button"
                        class="mantine-Burger-root mantine-UnstyledButton-root sidebar-burger"
                        aria-label="Close navigation"
                        style:--burger-size="var(--burger-size-sm)"
                        onclick={toggleNavigation}
                    >
                        <span class="mantine-Burger-burger" data-opened="true"></span>
                    </button>
                {/if}
                <Brand />
            </div>
            <div class={['sidebar-scroll', shell['scrollArea']]}>
                <Navigation sidebar onSelect={() => (showMobileNav = false)} />
            </div>
            {#if mobile.current}
                <div class={['sidebar-footer', shell['footerSection']]}>
                    <HeaderControls
                        mobile
                        footer
                        version=""
                        updateAvailable={false}
                        showBuild={() => (showBuild = true)}
                        showRecap={() => (showRecap = true)}
                    />
                </div>
            {/if}
        </aside>
    {/if}

    {#if showRecap}
        <Recap close={() => (showRecap = false)} />
    {/if}
    {#if showBuild && metadata.data}
        <BuildInfo metadata={metadata.data} close={() => (showBuild = false)} />
    {/if}

    {#if $navigationPending}
        <div class="navigation-loading"><RouteLoading compact /></div>
    {/if}
    <div class="dashboard-content" aria-busy={Boolean($navigationPending)} bind:this={page}>
        <div
            class="route-content"
            class:pending={Boolean($navigationPending)}
            inert={Boolean($navigationPending)}
        >
            {@render children()}
        </div>
    </div>
</div>

<style>
    .dashboard {
        --header-height: 116px;
        --page-top: calc(var(--header-height) + var(--mantine-spacing-md));
        --page-inset: var(--mantine-spacing-xl);
        --sidebar-offset: 0px;
        min-height: 100vh;
    }

    .dashboard.sidebar,
    .dashboard.wide {
        --header-height: 64px;
    }

    .dashboard.sidebar {
        --page-top: calc(var(--header-height) + 10px);
    }

    .navigation-loading {
        position: fixed;
        z-index: 10;
        inset-block-start: var(--page-top);
        inset-inline: 0;
        width: 3rem;
        margin-inline: auto;
        border: 1px solid var(--gw-border);
        border-radius: var(--mantine-radius-md);
        background: var(--gw-raised-bg);
        pointer-events: none;
        animation: show-loading 150ms ease-out 120ms both;
    }

    .route-content.pending {
        visibility: hidden;
    }

    @keyframes show-loading {
        from {
            opacity: 0;
        }
        to {
            opacity: 1;
        }
    }

    .dashboard.mobile {
        --page-inset: var(--mantine-spacing-md);
    }

    .dashboard.wide {
        --page-inset: 10vw;
    }

    .dashboard.sidebar-pinned {
        --sidebar-offset: 300px;
    }

    .dashboard-header {
        display: block;
        position: fixed;
        z-index: 100;
        inset-inline: var(--sidebar-offset) 0;
        top: 0;
        height: var(--header-height);
        padding: 0;
        border: 0;
        border-bottom: 1px solid
            light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-4));
        background: var(--mantine-color-dark-7);
        transition: inset-inline-start 0.5s ease-in-out;
    }

    .sidebar .dashboard-header {
        border: 0;
    }

    .brand-group {
        display: flex;
        height: 100%;
        min-width: 0;
        align-items: stretch;
        gap: var(--mantine-spacing-xs);
    }

    .brand-divider {
        align-self: center;
        height: 50%;
        margin-inline: 10px;
        border-inline-start: 1px solid var(--mantine-color-default-border);
    }

    .dashboard .dashboard-content {
        max-width: none;
        min-width: 0;
        min-height: 100vh;
        margin: 0;
        margin-inline-start: var(--sidebar-offset);
        padding: var(--page-top) var(--page-inset) var(--page-inset);
        transition: margin-inline-start 0.5s ease-in-out;
    }

    .dashboard-sidebar {
        position: fixed;
        z-index: 200;
        inset-inline-start: 0;
        top: env(safe-area-inset-top, 0px);
        display: flex;
        flex-direction: column;
        padding: var(--mantine-spacing-md) var(--mantine-spacing-md) 0;
        width: min(300px, calc(100vw - 24px));
        transform: translateX(calc(-100% - 24px));
        transition: transform 500ms ease-in-out;
    }

    .dashboard-sidebar:dir(rtl) {
        transform: translateX(calc(100% + 24px));
    }

    .dashboard-sidebar.nav-open {
        transform: none;
    }

    .sidebar-brand {
        display: flex;
        justify-content: center;
        flex-shrink: 0;
    }

    .sidebar-burger {
        position: absolute;
        inset-inline-start: 0;
    }

    .sidebar-scroll {
        flex: 1 1 auto;
        min-height: 0;
        overflow-y: auto;
        scrollbar-width: thin;
    }

    .mobile .sidebar-scroll {
        scrollbar-width: none;
    }

    .sidebar-footer {
        padding-top: var(--mantine-spacing-md);
        flex-shrink: 0;
    }

    @media (prefers-reduced-motion: reduce) {
        .navigation-loading {
            animation: none;
        }

        .dashboard-header,
        .dashboard .dashboard-content,
        .dashboard-sidebar {
            transition: none;
        }
    }

    .sidebar-dismiss {
        position: fixed;
        inset: 0;
        z-index: 150;
        width: 100%;
        border: 0;
        padding: 0;
        background: transparent;
    }
</style>
