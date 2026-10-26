import { Badge, Card, Group, Text } from '@mantine/core'
import { showModal } from '@shared/_modals/show-modal'
import { runAsync } from '@shared/lib/run-async'
import { formatInt } from '@shared/utils/misc'
import { memo } from 'react'
import { PiTag, PiUsers } from 'react-icons/pi'
import { TbCirclesRelation } from 'react-icons/tb'

import classes from './Checkbox.module.css'
import type { IProps } from './interfaces'

export const InternalSquadCardShared = memo((props: IProps) => {
    const { internalSquad } = props

    const handleOpenEditModal = (squadUuid: string) => {
        runAsync(
            showModal('internalSquads_internalSquadsInboundsDrawer', {
                squadUuid
            })
        )
    }

    if (!internalSquad) {
        return null
    }

    return (
        <Card
            className={classes.compactRoot}
            key={internalSquad.uuid}
            onClick={() => handleOpenEditModal(internalSquad.uuid)}
        >
            <Group align="center" gap="xs" justify="space-between" wrap="nowrap">
                <Group align="center" gap="xs" style={{ flex: 1, minWidth: 0 }} wrap="nowrap">
                    <TbCirclesRelation size={20} />
                    <Text className={classes.compactLabel} size="xs" truncate>
                        {internalSquad.name}
                    </Text>
                </Group>

                <Group gap="xs" wrap="nowrap">
                    <Badge
                        color="teal"
                        leftSection={<PiUsers size="16" />}
                        size="md"
                        variant="light"
                        visibleFrom="sm"
                    >
                        {formatInt(internalSquad.info.membersCount, {
                            thousandSeparator: ','
                        })}{' '}
                    </Badge>
                    <Badge color="blue" leftSection={<PiTag size="16" />} size="md" variant="light">
                        {internalSquad.info.inboundsCount}
                    </Badge>
                </Group>
            </Group>
        </Card>
    )
})
