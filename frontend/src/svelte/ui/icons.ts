// Paths from the icon sets used by Gowave's React frontend, as distributed in react-icons 5.7.0 and @tabler/icons-react 3.48.0.
// Phosphor, Tabler, Heroicons, BoxIcons, and React Icons are MIT licensed. Simple Icons is CC0.
import type { dashboardIcons } from './dashboard-icons'

const publicIcons = {
    BiLogoGithub: {
        attrs: {
            fill: 'currentColor',
            stroke: 'currentColor',
            'stroke-width': '0',
            viewBox: '0 0 24 24'
        },
        content:
            '<path fill-rule="evenodd" clip-rule="evenodd" d="M12.026 2c-5.509 0-9.974 4.465-9.974 9.974 0 4.406 2.857 8.145 6.821 9.465.499.09.679-.217.679-.481 0-.237-.008-.865-.011-1.696-2.775.602-3.361-1.338-3.361-1.338-.452-1.152-1.107-1.459-1.107-1.459-.905-.619.069-.605.069-.605 1.002.07 1.527 1.028 1.527 1.028.89 1.524 2.336 1.084 2.902.829.091-.645.351-1.085.635-1.334-2.214-.251-4.542-1.107-4.542-4.93 0-1.087.389-1.979 1.024-2.675-.101-.253-.446-1.268.099-2.64 0 0 .837-.269 2.742 1.021a9.582 9.582 0 0 1 2.496-.336 9.554 9.554 0 0 1 2.496.336c1.906-1.291 2.742-1.021 2.742-1.021.545 1.372.203 2.387.099 2.64.64.696 1.024 1.587 1.024 2.675 0 3.833-2.33 4.675-4.552 4.922.355.308.675.916.675 1.846 0 1.334-.012 2.41-.012 2.737 0 .267.178.577.687.479C19.146 20.115 22 16.379 22 11.974 22 6.465 17.535 2 12.026 2z"></path>'
    },
    BiLogoTelegram: {
        attrs: {
            fill: 'currentColor',
            stroke: 'currentColor',
            'stroke-width': '0',
            viewBox: '0 0 24 24'
        },
        content:
            '<path d="m20.665 3.717-17.73 6.837c-1.21.486-1.203 1.161-.222 1.462l4.552 1.42 10.532-6.645c.498-.303.953-.14.579.192l-8.533 7.701h-.002l.002.001-.314 4.692c.46 0 .663-.211.921-.46l2.211-2.15 4.599 3.397c.848.467 1.457.227 1.668-.785l3.019-14.228c.309-1.239-.473-1.8-1.282-1.434z"></path>'
    },
    PiShuffleDuotone: {
        attrs: {
            fill: 'currentColor',
            stroke: 'currentColor',
            'stroke-width': '0',
            viewBox: '0 0 256 256'
        },
        content:
            '<path d="M107.14,98.8,128,128l-20.86,29.2A64,64,0,0,1,55.06,184H32V72H55.06A64,64,0,0,1,107.14,98.8ZM200.94,72a64,64,0,0,0-52.08,26.8L128,128l20.86,29.2A64,64,0,0,0,200.94,184H232V72Z" opacity="0.2"></path><path d="M237.66,178.34a8,8,0,0,1,0,11.32l-24,24a8,8,0,0,1-11.32-11.32L212.69,192H200.94a72.12,72.12,0,0,1-58.59-30.15l-41.72-58.4A56.1,56.1,0,0,0,55.06,80H32a8,8,0,0,1,0-16H55.06a72.12,72.12,0,0,1,58.59,30.15l41.72,58.4A56.1,56.1,0,0,0,200.94,176h11.75l-10.35-10.34a8,8,0,0,1,11.32-11.32ZM143,107a8,8,0,0,0,11.16-1.86l1.2-1.67A56.1,56.1,0,0,1,200.94,80h11.75L202.34,90.34a8,8,0,0,0,11.32,11.32l24-24a8,8,0,0,0,0-11.32l-24-24a8,8,0,0,0-11.32,11.32L212.69,64H200.94a72.12,72.12,0,0,0-58.59,30.15l-1.2,1.67A8,8,0,0,0,143,107Zm-30,42a8,8,0,0,0-11.16,1.86l-1.2,1.67A56.1,56.1,0,0,1,55.06,176H32a8,8,0,0,0,0,16H55.06a72.12,72.12,0,0,0,58.59-30.15l1.2-1.67A8,8,0,0,0,113,149Z"></path>'
    },
    PiSignInDuotone: {
        attrs: {
            fill: 'currentColor',
            stroke: 'currentColor',
            'stroke-width': '0',
            viewBox: '0 0 256 256'
        },
        content:
            '<path d="M200,40V216H40a16,16,0,0,1-16-16V56A16,16,0,0,1,40,40Z" opacity="0.2"></path><path d="M141.66,133.66l-40,40a8,8,0,0,1-11.32-11.32L116.69,136H24a8,8,0,0,1,0-16h92.69L90.34,93.66a8,8,0,0,1,11.32-11.32l40,40A8,8,0,0,1,141.66,133.66ZM200,32H136a8,8,0,0,0,0,16h56V208H136a8,8,0,0,0,0,16h64a8,8,0,0,0,8-8V40A8,8,0,0,0,200,32Z"></path>'
    },
    PiSignpostDuotone: {
        attrs: {
            fill: 'currentColor',
            stroke: 'currentColor',
            'stroke-width': '0',
            viewBox: '0 0 256 256'
        },
        content:
            '<path d="M240,112l-33.62,37.35a8,8,0,0,1-5.94,2.65H40a8,8,0,0,1-8-8V80a8,8,0,0,1,8-8H200.44a8,8,0,0,1,5.94,2.65Z" opacity="0.2"></path><path d="M246,106.65,212.33,69.3A16,16,0,0,0,200.44,64H136V32a8,8,0,0,0-16,0V64H40A16,16,0,0,0,24,80v64a16,16,0,0,0,16,16h80v64a8,8,0,0,0,16,0V160h64.44a16,16,0,0,0,11.89-5.3L246,117.35A8,8,0,0,0,246,106.65ZM200.44,144H40V80H200.44l28.8,32Z"></path>'
    },
    SiKeycloak: {
        attrs: {
            fill: 'currentColor',
            stroke: 'currentColor',
            'stroke-width': '0',
            role: 'img',
            viewBox: '0 0 24 24'
        },
        content:
            '<path d="m18.742 1.182-12.493.002C4.155 4.784 2.079 8.393 0 12.002c2.071 3.612 4.162 7.214 6.252 10.816l12.49-.004 3.089-5.404h2.158v-.002H24L23.996 6.59h-2.168zM8.327 4.792h2.081l1.04 1.8-3.12 5.413 3.117 5.403-1.035 1.81H8.327a2047.566 2047.566 0 0 0-4.168-7.204C5.547 9.606 6.937 7.2 8.327 4.792Zm6.241 0 2.086.003c1.393 2.405 2.78 4.813 4.166 7.222l-4.167 7.2h-2.08c-.382-.562-1.038-1.808-1.038-1.808l3.123-5.405-3.124-5.413z"></path>'
    },
    TbFingerprint: {
        attrs: {
            fill: 'none',
            stroke: 'currentColor',
            'stroke-width': '2',
            viewBox: '0 0 24 24',
            'stroke-linecap': 'round',
            'stroke-linejoin': 'round'
        },
        content:
            '<path d="M18.9 7a8 8 0 0 1 1.1 5v1a6 6 0 0 0 .8 3"></path><path d="M8 11a4 4 0 0 1 8 0v1a10 10 0 0 0 2 6"></path><path d="M12 11v2a14 14 0 0 0 2.5 8"></path><path d="M8 15a18 18 0 0 0 1.8 6"></path><path d="M4.9 19a22 22 0 0 1 -.9 -7v-1a8 8 0 0 1 12 -6.95"></path>'
    },
    TbKey: {
        attrs: {
            fill: 'none',
            stroke: 'currentColor',
            'stroke-width': '2',
            viewBox: '0 0 24 24',
            'stroke-linecap': 'round',
            'stroke-linejoin': 'round'
        },
        content:
            '<path d="M16.555 3.843l3.602 3.602a2.877 2.877 0 0 1 0 4.069l-2.643 2.643a2.877 2.877 0 0 1 -4.069 0l-.301 -.301l-6.558 6.558a2 2 0 0 1 -1.239 .578l-.175 .008h-1.172a1 1 0 0 1 -.993 -.883l-.007 -.117v-1.172a2 2 0 0 1 .467 -1.284l.119 -.13l.414 -.414h2v-2h2v-2l2.144 -2.144l-.301 -.301a2.877 2.877 0 0 1 0 -4.069l2.643 -2.643a2.877 2.877 0 0 1 4.069 0"></path><path d="M15 9h.01"></path>'
    },
    PocketIdLogo: {
        attrs: {
            viewBox: '0 0 1015 1015',
            fill: 'currentColor',
            stroke: 'none',
            'stroke-width': '0'
        },
        content:
            '<path d="M506.6,0c209.52,0,379.98,170.45,379.98,379.96,0,82.33-25.9,160.68-74.91,226.54-48.04,64.59-113.78,111.51-190.13,135.71l-21.1,6.7-50.29-248.04,13.91-6.73c45.41-21.95,74.76-68.71,74.76-119.11,0-72.91-59.31-132.23-132.21-132.23s-132.23,59.32-132.23,132.23c0,50.4,29.36,97.16,74.77,119.11l13.65,6.61-81.01,499.24h-226.36V0h351.18Z" fill="currentColor" />'
    },
    TbX: {
        attrs: {
            fill: 'none',
            stroke: 'currentColor',
            'stroke-width': '2',
            viewBox: '0 0 24 24',
            'stroke-linecap': 'round',
            'stroke-linejoin': 'round'
        },
        content: '<path d="M18 6l-12 12"></path><path d="M6 6l12 12"></path>'
    },
    PiWifiSlash: {
        attrs: { viewBox: '0 0 256 256', fill: 'currentColor' },
        content:
            '<path d="M213.92,210.62a8,8,0,1,1-11.84,10.76l-52-57.15a60,60,0,0,0-57.41,7.24,8,8,0,1,1-9.42-12.93A75.43,75.43,0,0,1,128,144c1.28,0,2.55,0,3.82.1L104.9,114.49A108,108,0,0,0,61,135.31,8,8,0,0,1,49.73,134,8,8,0,0,1,51,122.77a124.27,124.27,0,0,1,41.71-21.66L69.37,75.4a155.43,155.43,0,0,0-40.29,24A8,8,0,0,1,18.92,87,171.87,171.87,0,0,1,58,62.86L42.08,45.38A8,8,0,1,1,53.92,34.62ZM128,192a12,12,0,1,0,12,12A12,12,0,0,0,128,192ZM237.08,87A172.3,172.3,0,0,0,106,49.4a8,8,0,1,0,2,15.87A158.33,158.33,0,0,1,128,64a156.25,156.25,0,0,1,98.92,35.37A8,8,0,0,0,237.08,87ZM195,135.31a8,8,0,0,0,11.24-1.3,8,8,0,0,0-1.3-11.24,124.25,124.25,0,0,0-51.73-24.2A8,8,0,1,0,150,114.24,108.12,108.12,0,0,1,195,135.31Z"></path>'
    }
} as const

export type IconName = keyof typeof publicIcons | keyof typeof dashboardIcons
type DashboardName = keyof typeof dashboardIcons
type Glyph = { attrs: Record<string, string>; content: string }

let loaded: typeof dashboardIcons | undefined
let loading: Promise<typeof dashboardIcons> | undefined

export function getIcon(name: IconName): Glyph | Promise<Glyph> {
    if (Object.hasOwn(publicIcons, name)) return publicIcons[name as keyof typeof publicIcons]
    const key = name as DashboardName
    if (loaded) return loaded[key]
    loading ??= import('./dashboard-icons')
        .then(({ dashboardIcons }) => {
            loaded = dashboardIcons
            return dashboardIcons
        })
        .catch((error: unknown) => {
            loading = undefined
            throw error
        })
    return loading.then((icons) => icons[key])
}
