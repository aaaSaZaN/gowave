import type { IconName } from '../ui/icons'

export interface MenuLink {
    name: string
    id: string
    icon: IconName | 'Logo'
    href: string
    dropdownItems?: MenuLink[]
    newTab?: boolean
}

export interface MenuSection {
    id: string
    header: string
    icon?: IconName | 'Logo'
    section: MenuLink[]
}

export function activePath(path: string, href: string): boolean {
    return path === href || path.startsWith(`${href}/`)
}
