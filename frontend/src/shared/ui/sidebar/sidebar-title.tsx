import { Text } from '@mantine/core'
import { useGetAuthStatus } from '@shared/api/hooks/auth/auth.query.hooks'
import { parseColoredTextUtil } from '@shared/utils/misc'
import { useMemo } from 'react'

import classes from './sidebar.module.css'

export const SidebarTitleShared = () => {
    const { data: authStatus } = useGetAuthStatus()

    const titleParts = useMemo(() => {
        if (authStatus?.branding.title) {
            return parseColoredTextUtil(authStatus.branding.title)
        }

        return [
            { text: 'Go', color: 'cyan' },
            { text: 'Wave', color: 'white' }
        ]
    }, [authStatus])

    return (
        <Text className={classes.logoTitle}>
            {titleParts.map((part) => (
                <Text c={part.color || 'white'} component="span" inherit key={part.text}>
                    {part.text}
                </Text>
            ))}
        </Text>
    )
}
