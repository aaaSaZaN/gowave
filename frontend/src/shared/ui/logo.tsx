import { Box, type BoxProps, type ElementProps } from '@mantine/core'
import { panelUrl } from '@shared/lib/panel-path'

interface LogoProps
    extends ElementProps<'img', keyof BoxProps>,
        Omit<BoxProps, 'children' | 'ref'> {
    size?: number | string
}

export function Logo({ size, style, ...props }: LogoProps) {
    return (
        <Box
            alt="GoWave"
            component="img"
            src={panelUrl('/favicons/logo.svg')}
            style={{ width: size, height: size ?? 'auto', aspectRatio: '1', ...style }}
            {...props}
        />
    )
}
