import type { TSubscriptionPagePlatformKey } from '@remnawave/subscription-page-types'
import {
    TbBrandAndroid,
    TbBrandApple,
    TbBrandWindows,
    TbDeviceDesktop,
    TbDeviceTv
} from 'react-icons/tb'

export const PLATFORM_ICONS: Record<
    TSubscriptionPagePlatformKey,
    React.ComponentType<{ size: number }>
> = {
    android: TbBrandAndroid,
    androidTV: TbDeviceTv,
    appleTV: TbDeviceTv,
    ios: TbBrandApple,
    linux: TbDeviceDesktop,
    macos: TbBrandApple,
    windows: TbBrandWindows
}

export const PLATFORM_LABELS: Record<TSubscriptionPagePlatformKey, string> = {
    android: 'Android',
    androidTV: 'Android TV',
    appleTV: 'Apple TV',
    ios: 'iOS',
    linux: 'Linux',
    macos: 'macOS',
    windows: 'Windows'
}

export const AVAILABLE_PLATFORMS: { label: string; value: TSubscriptionPagePlatformKey }[] = [
    { label: 'iOS', value: 'ios' },
    { label: 'Android', value: 'android' },
    { label: 'Linux', value: 'linux' },
    { label: 'macOS', value: 'macos' },
    { label: 'Windows', value: 'windows' },
    { label: 'Android TV', value: 'androidTV' },
    { label: 'Apple TV', value: 'appleTV' }
]
