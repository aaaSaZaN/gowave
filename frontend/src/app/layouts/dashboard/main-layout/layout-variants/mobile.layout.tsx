import type { IGowaveInfo } from '@entities/dashboard/updates-store'
import { Group } from '@mantine/core'
import { useClickOutside, useDisclosure } from '@mantine/hooks'
import { optionalProp } from '@shared/lib/optional-prop'
import { HeaderControls } from '@shared/ui'

import { DASHBOARD_LINKS } from '../layout-shared'
import { SidebarShellLayout } from './sidebar-shell.layout'

interface IProps {
    headerControls: React.ReactNode
    isLoadingUpdates: boolean
    isSocialButtons: boolean
    gowaveInfo: IGowaveInfo
}

export const MobileLayout = (props: IProps) => {
    const { headerControls, isLoadingUpdates, isSocialButtons, gowaveInfo } = props

    const [opened, { toggle }] = useDisclosure()

    const ref = useClickOutside(() => {
        if (opened) {
            toggle()
        }
    })

    return (
        <SidebarShellLayout
            closedSide="mobile"
            footer={
                isSocialButtons && (
                    <Group justify="center" mt="md" style={{ flexShrink: 0 }}>
                        <HeaderControls
                            {...DASHBOARD_LINKS}
                            isGithubLoading={isLoadingUpdates}
                            {...optionalProp('stars', gowaveInfo.starsCount || undefined)}
                            withLanguage={false}
                            withLogout={false}
                            withVersion={false}
                        />
                    </Group>
                )
            }
            headerControls={headerControls}
            navbarRef={ref}
            onNavClose={toggle}
            opened={opened}
            padding="md"
            toggle={toggle}
        />
    )
}
