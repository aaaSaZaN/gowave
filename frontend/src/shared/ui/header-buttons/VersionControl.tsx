import { Group, Text } from '@mantine/core'
import { modals } from '@mantine/modals'
import { useGetGowaveMetadata } from '@shared/api/hooks'
import clsx from '@shared/lib/classes'

import { Logo } from '../logo'
import { BaseOverlayHeader } from '../overlays/base-overlay-header'
import { BuildInfoModal } from '../sidebar/build-info-modal'
import { usePanelUpdateCheck } from '../sidebar/panel-update-check'
import { HeaderControl } from './HeaderControl'
import { SkeletonHeaderControl } from './SkeletonHeaderControl'
import classes from './VersionControl.module.css'

export function VersionControl() {
    const { data: gowaveMetadata, isLoading } = useGetGowaveMetadata()
    // Stable builds compare with releases/latest, dev builds with the rolling
    // dev pre-release; the backend picks by the channel it was built for.
    const { data: updateCheck } = usePanelUpdateCheck()

    const isNewVersionAvailable = Boolean(updateCheck?.hasUpdate)
    const isDev = gowaveMetadata?.git?.backend?.branch === 'dev'

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
            children: <BuildInfoModal gowaveMetadata={gowaveMetadata} />
        })
    }

    return (
        <HeaderControl
            className={clsx(classes.version, {
                [classes.newVersion]: isNewVersionAvailable,
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
