import { Tooltip } from '@mantine/core'

import classes from './tooltip.module.css'

export default {
    Tooltip: Tooltip.extend({
        classNames: {
            tooltip: classes.tooltip
        },
        defaultProps: {
            radius: 'md',
            withArrow: true,
            transitionProps: { transition: 'scale-x', duration: 300 },
            arrowSize: 2
        }
    })
}
