import { Box, type BoxProps, type ElementProps } from '@mantine/core'
import cx from '@shared/lib/classes'
import { panelUrl } from '@shared/lib/panel-path'

import classes from './logo.module.css'

// A branded path and build version bypass copies of the previous stock artwork.
export const GOWAVE_LOGO_SRC = panelUrl(`/favicons/gowave.svg?v=${__BUILD_ID__}`)

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
            src={GOWAVE_LOGO_SRC}
            style={{ width: size, height: size, ...style }}
            {...props}
        />
    )
}
