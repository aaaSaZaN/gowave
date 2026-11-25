<script lang="ts">
    import { panelUrl } from '@shared/lib/panel-path'
    import { locale } from '../i18n.svelte'
    import { route } from '../router'
    import Icon from '../ui/Icon.svelte'
    import type { IconName } from '../ui/icons'
    import desktopStyles from './desktop-navigation.module.css'
    import { desktopSections } from './desktop-sections'
    import mobileStyles from './mobile-navigation.module.css'
    import { mobileSections } from './mobile-sections'
    import { activePath, type MenuLink } from './navigation'

    let { sidebar = false, onSelect }: { sidebar?: boolean; onSelect?: () => void } = $props()
    const groups = $derived(sidebar ? mobileSections(locale.t) : desktopSections(locale.t))
    const id = $props.id()
    const logo = panelUrl(`favicons/gowave.svg?v=${__BUILD_ID__}`)
    let open = $state('')
    let expandedGroup = $state<Record<string, boolean>>({})

    function isBrandIcon(name: IconName | 'Logo' | undefined): boolean {
        return (
            name === 'XrayLogo' ||
            name === 'MihomoLogo' ||
            name === 'StashLogo' ||
            name === 'SingboxLogo'
        )
    }

    function go(item: MenuLink): string {
        return item.newTab ? item.href : panelUrl(item.href)
    }

    function current(item: MenuLink): boolean {
        return !item.newTab && activePath(route.pathname, item.href)
    }

    function select(): void {
        open = ''
        onSelect?.()
    }

    function openWithKeyboard(event: KeyboardEvent, id: string): void {
        if (event.key === 'ArrowDown') {
            event.preventDefault()
            open = id
            const trigger = event.currentTarget
            if (trigger instanceof HTMLElement) {
                requestAnimationFrame(() => {
                    trigger.parentElement?.querySelector<HTMLElement>('[role=menuitem]')?.focus()
                })
            }
        }
    }
</script>

<svelte:window
    onkeydown={(event) => {
        if (event.key === 'Escape') open = ''
    }}
/>

{#snippet icon(name: IconName | 'Logo', size = 18)}
    {#if name === 'Logo'}
        <img src={logo} alt="" width={size} height={size}>
    {:else}
        <Icon {name} {size} />
    {/if}
{/snippet}

{#snippet desktopLink(item: MenuLink)}
    <a
        class={[
            'menu-link m_99ac2aa1 mantine-UnstyledButton-root',
            current(item) && desktopStyles['menuItemActive']
        ]}
        href={go(item)}
        data-preload={item.newTab ? undefined : 'hover'}
        role="menuitem"
        target={item.newTab ? '_blank' : undefined}
        rel={item.newTab ? 'noopener noreferrer' : undefined}
        onclick={select}
    >
        <span class={[desktopStyles['icon'], isBrandIcon(item.icon) && desktopStyles['logoIcon']]}
            >{@render icon(item.icon)}</span
        >
        {item.name}
    </a>
{/snippet}

{#snippet sidebarLink(item: MenuLink, nested = false)}
    {@const selected = !item.newTab && (nested ? current(item) : route.pathname === item.href)}
    <a
        class={[
            'sidebar-link',
            nested ? mobileStyles['sectionDropdownItemLink'] : mobileStyles['sectionLink']
        ]}
        href={go(item)}
        data-preload={item.newTab ? undefined : 'hover'}
        data-active={selected ? 'true' : undefined}
        aria-current={selected ? 'page' : undefined}
        target={item.newTab ? '_blank' : undefined}
        rel={item.newTab ? 'noopener noreferrer' : undefined}
        onclick={select}
    >
        <span class="sidebar-icon">{@render icon(item.icon, 20)}</span>
        <span>{item.name}</span>
    </a>
{/snippet}

{#if sidebar}
    <nav class="sidebar-navigation" aria-label="Dashboard">
        {#each groups as group, index (group.id)}
            <section>
                {#if index > 0}
                    <hr>
                {/if}
                <h6 class={mobileStyles['sectionTitle']}>{group.header}</h6>
                <div class="sidebar-links">
                    {#each group.section as item (item.id)}
                        {#if item.dropdownItems}
                            {@const expanded =
                                expandedGroup[item.id] ?? item.dropdownItems.some(current)}
                            <button
                                type="button"
                                class={[
                                    'sidebar-link',
                                    'mantine-UnstyledButton-root',
                                    mobileStyles['sectionLink']
                                ]}
                                aria-controls={id + item.id}
                                aria-expanded={expanded}
                                onclick={() => (expandedGroup[item.id] = !expanded)}
                            >
                                <span class="sidebar-icon">{@render icon(item.icon, 20)}</span>
                                <span>{item.name}</span>
                                <span class="sidebar-caret">
                                    <Icon name="TbChevronRight" size={14} />
                                </span>
                            </button>
                            <div
                                id={id + item.id}
                                class="sidebar-submenu"
                                class:expanded
                                inert={!expanded}
                            >
                                <div>
                                    {#each item.dropdownItems as link (link.id)}
                                        {@render sidebarLink(link, true)}
                                    {/each}
                                </div>
                            </div>
                        {:else}
                            {@render sidebarLink(item)}
                        {/if}
                    {/each}
                </div>
            </section>
        {/each}
    </nav>
{:else}
    <nav class={['desktop-navigation', desktopStyles['navBar']]} aria-label="Dashboard">
        {#each groups as group (group.id)}
            {@const first = group.section[0]}
            {@const grouped = group.section.length > 1 || Boolean(first?.dropdownItems)}
            {@const selected = group.section.some(
                (item) => current(item) || item.dropdownItems?.some(current)
            )}
            {#if first}
                <!-- svelte-ignore a11y_no_static_element_interactions (The link opens the menu with ArrowDown.) -->
                <div
                    class="menu-group"
                    onpointerenter={() => (open = grouped ? group.id : '')}
                    onpointerleave={() => (open = '')}
                    onfocusout={(event) => {
                        if (
                            !(event.relatedTarget instanceof Node) ||
                            !event.currentTarget.contains(event.relatedTarget)
                        )
                            open = ''
                    }}
                >
                    <a
                        class={[
                            'mantine-UnstyledButton-root',
                            desktopStyles['navItem'],
                            selected && desktopStyles['navItemActive']
                        ]}
                        href={go(first.dropdownItems?.[0] ?? first)}
                        data-preload="hover"
                        aria-haspopup={grouped ? 'menu' : undefined}
                        aria-expanded={grouped ? open === group.id : undefined}
                        data-expanded={open === group.id ? '' : undefined}
                        onkeydown={(event) => {
                            if (grouped) openWithKeyboard(event, group.id)
                        }}
                        onclick={select}
                    >
                        <span
                            class={[
                                desktopStyles['icon'],
                                isBrandIcon(group.icon ?? first.icon) && desktopStyles['logoIcon']
                            ]}
                            >{@render icon(group.icon ?? first.icon)}</span
                        >
                        <span>{group.header}</span>
                        {#if grouped}
                            <Icon name="PiCaretDownBold" class={desktopStyles['caret']} size={11} />
                        {/if}
                    </a>
                    {#if grouped}
                        <div
                            class="section-menu"
                            class:visible={open === group.id}
                            inert={open !== group.id}
                            role="menu"
                            aria-label={group.header}
                        >
                            {#each group.section as item, index (item.id)}
                                {#if item.dropdownItems}
                                    {#if index > 0}
                                        <div class="menu-divider"></div>
                                    {/if}
                                    <div class="m_9bfac126">{item.name}</div>
                                    {#each item.dropdownItems as link (link.id)}
                                        {@render desktopLink(link)}
                                    {/each}
                                    {#if index < group.section.length - 1}
                                        <div class="menu-divider"></div>
                                    {/if}
                                {:else}
                                    {@render desktopLink(item)}
                                {/if}
                            {/each}
                        </div>
                    {/if}
                </div>
            {/if}
        {/each}
    </nav>
{/if}

<style>
    .desktop-navigation {
        display: flex;
        min-width: 0;
    }

    .menu-group {
        position: relative;
        min-width: max-content;
    }

    .menu-group > a {
        text-decoration: none;
    }

    .section-menu {
        position: absolute;
        inset-block-start: calc(100% + 8px);
        inset-inline-start: 0;
        z-index: 15;
        width: 250px;
        opacity: 0;
        visibility: hidden;
        pointer-events: none;
        transition:
            opacity 180ms ease-out,
            visibility 180ms;
        padding: 4px;
        background: var(--mantine-color-dark-6);
        border: 1px solid var(--mantine-color-dark-4);
        border-radius: var(--mantine-radius-md);
        box-shadow:
            0 8px 32px rgba(0, 0, 0, 0.4),
            0 2px 8px rgba(0, 0, 0, 0.2);
    }

    .section-menu::before {
        position: absolute;
        inset-block-end: 100%;
        inset-inline: 0;
        height: 8px;
        content: "";
    }

    .section-menu.visible {
        opacity: 1;
        visibility: visible;
        pointer-events: auto;
    }

    .menu-link {
        gap: var(--mantine-spacing-xs);
        text-decoration: none;
    }

    .menu-divider {
        margin: 4px 0;
        border-top: 1px solid var(--mantine-color-dark-5);
    }

    .sidebar-navigation {
        display: flex;
        flex-direction: column;
        gap: var(--mantine-spacing-md);
        padding-block: var(--mantine-spacing-md);
    }

    .sidebar-navigation h6 {
        margin-top: 0;
    }

    .sidebar-navigation hr {
        margin: 0 0 var(--mantine-spacing-lg);
        border: 0;
        border-top: 1px dashed var(--mantine-color-cyan-4);
        opacity: 0.3;
    }

    .sidebar-links {
        display: flex;
        flex-direction: column;
        gap: 0.5px;
    }

    .sidebar-link {
        display: flex;
        align-items: center;
        gap: var(--mantine-spacing-sm);
        min-height: 41px;
        padding: 8px 10px;
        border-block: 0;
        background-color: transparent;
        color: var(--mantine-color-text);
        font-size: var(--mantine-font-size-sm);
        text-align: start;
        text-decoration: none;
    }

    .sidebar-link[data-active="true"] {
        color: var(--mantine-color-cyan-4);
        background-image: linear-gradient(135deg, rgb(6 182 212 / 15%), rgb(6 182 212 / 10%));
    }

    .sidebar-icon {
        display: inline-flex;
        flex-shrink: 0;
        align-items: center;
    }

    .sidebar-caret {
        display: flex;
        margin-inline-start: auto;
        transition: transform 200ms ease-in-out;
    }

    .sidebar-caret:dir(rtl) {
        transform: rotate(180deg);
    }
    [aria-expanded="true"] .sidebar-caret {
        transform: rotate(90deg);
    }

    .sidebar-submenu {
        display: grid;
        grid-template-rows: 0fr;
        opacity: 0;
        transition:
            grid-template-rows 200ms ease-in-out,
            opacity 200ms ease-in-out;
    }

    .sidebar-submenu.expanded {
        grid-template-rows: 1fr;
        opacity: 1;
    }

    .sidebar-submenu > div {
        min-height: 0;
        overflow: clip;
    }

    @media (prefers-reduced-motion: reduce) {
        .section-menu,
        .sidebar-caret,
        .sidebar-submenu {
            transition: none;
        }
    }
</style>
