import { Group, Text } from '@mantine/core'
import { modals } from '@mantine/modals'
import clsx from 'clsx'
import { useMemo } from 'react'
import semver from 'semver'

import { useGetGowaveMetadata } from '@shared/api/hooks'

import { useGowaveInfo } from '@entities/dashboard/updates-store'

import { Logo } from '../logo'
import { BaseOverlayHeader } from '../overlays/base-overlay-header'
import { BuildInfoModal } from '../sidebar/build-info-modal'
import { HeaderControl } from './HeaderControl'
import { SkeletonHeaderControl } from './SkeletonHeaderControl'
import classes from './VersionControl.module.css'

export function VersionControl() {
    const gowaveInfo = useGowaveInfo()
    const { data: gowaveMetadata, isLoading } = useGetGowaveMetadata()

    const [isNewVersionAvailable, isDev] = useMemo(() => {
        if (!gowaveMetadata) return [false, false]

        const currentVersion = semver.clean(gowaveMetadata.version) || gowaveMetadata.version
        const latest = semver.clean(gowaveInfo.latestVersion) || gowaveInfo.latestVersion

        const hasUpdate = Boolean(
            latest &&
            currentVersion &&
            semver.valid(latest) &&
            semver.valid(currentVersion) &&
            semver.gt(latest, currentVersion)
        )

        return [hasUpdate, gowaveMetadata.git?.backend?.branch !== 'main']
    }, [gowaveInfo.latestVersion, gowaveMetadata])

    if (isLoading || !gowaveMetadata) {
        return <SkeletonHeaderControl width={85} />
    }

    const handleClick = () => {
        modals.open({
            title: (
                <BaseOverlayHeader
                    iconColor="teal"
                    IconComponent={Logo}
                    iconVariant="soft"
                    title="Build Info"
                />
            ),
            centered: true,
            size: 'md',
            withCloseButton: true,
            children: (
                <BuildInfoModal
                    isNewVersionAvailable={isNewVersionAvailable}
                    gowaveMetadata={gowaveMetadata}
                />
            )
        })
    }

    return (
        <HeaderControl
            className={clsx(classes.version, {
                [classes.newVersion]: isNewVersionAvailable && !isDev,
                [classes.dev]: isDev
            })}
            onClick={handleClick}
            w="auto"
        >
            <Group gap={8} ml={10} mr={10} wrap="nowrap">
                <Logo size={20} />
                <Text ff="text" fw={600} size="sm">
                    {gowaveMetadata.version}
                </Text>
            </Group>
        </HeaderControl>
    )
}
