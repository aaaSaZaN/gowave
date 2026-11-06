import { ActionIcon, Tooltip } from '@mantine/core'
import type { GetNodeCommand } from '@remnawave/backend-contract'
import { showModal } from '@shared/_modals/show-modal'
import { runAsync } from '@shared/lib/run-async'
import semver from '@shared/lib/version'
import { memo, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { TbMapSearch } from 'react-icons/tb'

const MIN_TS_NODE_VERSION = '3.3.0'

interface IProps {
    node: GetNodeCommand.Response['response']
}

const GetNodeGeocheckFeatureComponent = (props: IProps) => {
    const { node } = props
    const { t } = useTranslation()

    const nodeVersion = node.versions?.node

    const isSupported = useMemo(() => {
        // Only legacy TypeScript Remnawave nodes before 3.3.0 lack geocheck.
        // Every GoWave Node (Go) release supports geocheck out of the box.
        if (nodeVersion?.includes('(TS)')) {
            const version = semver.coerce(nodeVersion)
            return version !== null && semver.gte(version, MIN_TS_NODE_VERSION)
        }

        return true
    }, [nodeVersion])

    const tooltipLabel = useMemo(() => {
        if (!isSupported) {
            return t('node-geocheck.requires-node-version', {
                version: `TS ${MIN_TS_NODE_VERSION}`
            })
        }
        if (!node.isConnected) {
            return `${t('node-geocheck.title')} (${t('common.status.offline', { defaultValue: 'нода не в сети' })})`
        }
        return t('node-geocheck.title')
    }, [isSupported, node.isConnected, t])

    return (
        <Tooltip label={tooltipLabel}>
            <ActionIcon
                disabled={!isSupported}
                color="indigo"
                onClick={() => {
                    runAsync(showModal('nodes_nodeGeocheckModal', { node }))
                }}
                size="lg"
                variant="soft"
            >
                <TbMapSearch size="22px" />
            </ActionIcon>
        </Tooltip>
    )
}

export const GetNodeGeocheckFeature = memo(GetNodeGeocheckFeatureComponent)
