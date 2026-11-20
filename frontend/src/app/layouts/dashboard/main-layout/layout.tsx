import { useGowaveInfo, useIsLoadingGowaveUpdates } from '@entities/dashboard/updates-store'
import { useExperimentalFeature } from '@entities/dashboard/view-preferences-store'
import { useMediaQuery } from '@mantine/hooks'
import { useIsMobile } from '@shared/hooks'
import { optionalProp } from '@shared/lib/optional-prop'
import { HeaderControls } from '@shared/ui/header-buttons'

import { DASHBOARD_LINKS } from './layout-shared'
import { CompactLayout } from './layout-variants/compact.layout'
import { MobileLayout } from './layout-variants/mobile.layout'
import { SidebarLayout } from './layout-variants/sidebar.layout'

import '@shared/_modals/modal-registry'

export function MainLayout() {
    const isLegacyLayoutStyle = useExperimentalFeature('legacyLayoutStyle')

    const isMobile = useIsMobile()

    const isHiResDesktop = useMediaQuery(`(min-width: 2048px)`, undefined, {
        getInitialValueInEffect: false
    })

    const gowaveInfo = useGowaveInfo()
    const isLoadingUpdates = useIsLoadingGowaveUpdates()

    const headerControls = (
        <HeaderControls
            {...DASHBOARD_LINKS}
            isGithubLoading={isLoadingUpdates}
            {...optionalProp('stars', gowaveInfo.starsCount || undefined)}
            withGithub={!isMobile}
            withRecap={!isMobile}
        />
    )

    if (isMobile) {
        return (
            <MobileLayout
                headerControls={headerControls}
                isSocialButtons={isMobile}
                isLoadingUpdates={isLoadingUpdates}
                gowaveInfo={gowaveInfo}
            />
        )
    }

    return isLegacyLayoutStyle ? (
        <SidebarLayout headerControls={headerControls} />
    ) : (
        <CompactLayout headerControls={headerControls} isHiResDesktop={isHiResDesktop} />
    )
}
