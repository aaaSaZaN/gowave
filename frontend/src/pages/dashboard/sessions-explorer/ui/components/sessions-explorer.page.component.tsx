import { Stack } from '@mantine/core'
import { SessionsExplorerWidget } from '@widgets/dashboard/sessions-explorer/sessions-explorer-widget'
import { motion } from 'motion/react'
import { useTranslation } from 'react-i18next'

import { Page } from '@shared/ui'

export default function SessionsExplorerPageComponent() {
    const { t } = useTranslation()

    return (
        <Page title={t('constants.sessions-explorer')}>
            <Stack>
                <motion.div
                    animate={{ opacity: 1 }}
                    initial={{ opacity: 0 }}
                    transition={{ duration: 0.5, ease: [0, 0.71, 0.2, 1.01] }}
                >
                    <SessionsExplorerWidget />
                </motion.div>
            </Stack>
        </Page>
    )
}
