import { Box, type BoxProps, type ElementProps } from '@mantine/core'
import cx from '@shared/lib/classes'
import { panelUrl } from '@shared/lib/panel-path'

import classes from './logo.module.css'

interface LogoProps
    extends ElementProps<'img', keyof BoxProps>,
        Omit<BoxProps, 'children' | 'ref'> {
    size?: number | string
}

export function Logo({ size, style, className, ...props }: LogoProps) {
    return (
        <Box
            alt="GoWave"
            className={cx(classes.logo, className)}
            component="img"
            src={panelUrl('/favicons/logo.svg')}
            style={{ width: size, height: size, ...style }}
            {...props}
        />
    )
}
