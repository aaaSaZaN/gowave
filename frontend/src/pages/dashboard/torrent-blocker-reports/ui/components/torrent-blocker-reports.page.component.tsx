import { Stack } from '@mantine/core'
import { Page } from '@shared/ui'
import { TorrentBlockerReportsTableWidget } from '@widgets/dashboard/torrent-blocker-reports/torrent-blocker-reports-table'
import { TorrentBlockerStatsWidget } from '@widgets/dashboard/torrent-blocker-reports/torrent-blocker-stats'
import { motion } from 'motion/react'
import { useTranslation } from 'react-i18next'

export default function TorrentBlockerReportsPageComponent() {
    const { t } = useTranslation()

    return (
        <Page title={t('constants.tb-reports')}>
            <Stack>
                <TorrentBlockerStatsWidget />

                <motion.div
                    animate={{ opacity: 1 }}
                    initial={{ opacity: 0 }}
                    transition={{ duration: 0.5, ease: [0, 0.71, 0.2, 1.01] }}
                >
                    <TorrentBlockerReportsTableWidget />
                </motion.div>
            </Stack>
        </Page>
    )
}
