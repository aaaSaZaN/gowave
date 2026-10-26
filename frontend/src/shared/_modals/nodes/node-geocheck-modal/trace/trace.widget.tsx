import { Box, Center, Text } from '@mantine/core'
import clsx from '@shared/lib/classes'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import classes from './Trace.module.css'

import { TargetDetail } from './target-detail'
import { TargetList } from './target-list'
import type { IGeocheckConnectivity } from './trace.types'
import { TraceSummary } from './trace-summary'

interface IProps {
    connectivity: IGeocheckConnectivity
    isFullscreen: boolean
}

export const GeocheckTraceWidget = (props: IProps) => {
    const { connectivity, isFullscreen } = props
    const { t } = useTranslation()

    const [selectedId, setSelectedId] = useState<null | string>(null)

    const { targets } = connectivity
    const selected = targets.find((target) => target.id === selectedId) ?? targets[0]

    if (targets.length === 0) {
        return (
            <Center h={240}>
                <Text c="dimmed" size="sm">
                    {t('common.message.no-data-available')}
                </Text>
            </Center>
        )
    }

    return (
        <Box className={clsx(classes.root, isFullscreen && classes.fullHeight)}>
            <TraceSummary connectivity={connectivity} />

            <Box className={classes.body}>
                <TargetList onSelect={setSelectedId} selectedId={selected?.id} targets={targets} />

                <Box className={classes.detail}>
                    {selected && <TargetDetail target={selected} />}
                </Box>
            </Box>
        </Box>
    )
}
