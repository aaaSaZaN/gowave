import { HomePage } from '@pages/dashboard/home/components'

import { useGetBandwidthStats, useGetGowaveHealth, useGetSystemStats } from '@shared/api/hooks'
import { LoadingScreen } from '@shared/ui/loading-screen'

export const HomePageConnector = () => {
    const { data: systemInfo } = useGetSystemStats()
    const { data: bandwidthStats } = useGetBandwidthStats()
    const { data: gowaveHealth } = useGetGowaveHealth()

    if (!systemInfo || !bandwidthStats || !gowaveHealth) {
        return <LoadingScreen />
    }

    return (
        <HomePage
            bandwidthStats={bandwidthStats}
            gowaveHealth={gowaveHealth}
            systemInfo={systemInfo}
        />
    )
}
