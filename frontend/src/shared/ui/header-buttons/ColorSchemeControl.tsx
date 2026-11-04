import { rem, useComputedColorScheme, useMantineColorScheme } from '@mantine/core'
import { PiMoon, PiSun } from 'react-icons/pi'

import { HeaderControl } from './HeaderControl'

export function ColorSchemeControl() {
    const { setColorScheme } = useMantineColorScheme()
    const computedColorScheme = useComputedColorScheme('dark', { getInitialValueInEffect: false })
    const isDark = computedColorScheme === 'dark'
    const iconStyle = { width: rem(22), height: rem(22) }

    return (
        <HeaderControl
            aria-label={isDark ? 'Switch to light theme' : 'Switch to dark theme'}
            onClick={() => setColorScheme(isDark ? 'light' : 'dark')}
            title={isDark ? 'Light theme' : 'Dark theme'}
        >
            {isDark ? <PiSun style={iconStyle} /> : <PiMoon style={iconStyle} />}
        </HeaderControl>
    )
}
