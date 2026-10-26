import { removeToken } from '@entities/auth'
import { ActionIcon, Group } from '@mantine/core'
import { clearQueryClient } from '@shared/api'
import { ROUTES } from '@shared/constants'
import { logoutEvents } from '@shared/emitters'
import { resetAllStores } from '@shared/hocs/store-wrapper'
import { useAuth } from '@shared/hooks'
import { runAsync } from '@shared/lib/run-async'
import { LanguagePicker } from '@shared/ui/language-picker/language-picker.shared'
import { PiArrowsClockwise, PiSignOutDuotone } from 'react-icons/pi'
import { useNavigate } from 'react-router'

export const HeaderButtons = () => {
    const { setIsAuthenticated } = useAuth()
    const navigate = useNavigate()

    const handleLogout = () => {
        logoutEvents.emit()
        setIsAuthenticated(false)
        removeToken()
        resetAllStores()
        clearQueryClient()
        runAsync(navigate(ROUTES.AUTH.LOGIN))
    }

    const handleRefresh = () => {
        resetAllStores()
        clearQueryClient()
        runAsync(navigate(0))
    }

    return (
        <Group grow preventGrowOverflow={false} wrap="wrap">
            <LanguagePicker />

            <ActionIcon color="gray" onClick={handleRefresh} size="xl">
                <PiArrowsClockwise size="24px" />
            </ActionIcon>

            <ActionIcon color="cyan" onClick={handleLogout} size="xl">
                <PiSignOutDuotone size="24px" />
            </ActionIcon>
        </Group>
    )
}
