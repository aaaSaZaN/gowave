import { useMediaQuery } from '@mantine/hooks'
import { createContext, type ReactNode } from 'react'

const MOBILE_QUERY = `(max-width: 64rem)`
export const IsMobileContext = createContext<boolean>(false)

interface IsMobileProviderProps {
    children: ReactNode
}

export function IsMobileProvider({ children }: IsMobileProviderProps) {
    const isMobile = useMediaQuery(MOBILE_QUERY, undefined, {
        getInitialValueInEffect: false
    })

    return <IsMobileContext.Provider value={isMobile}>{children}</IsMobileContext.Provider>
}
