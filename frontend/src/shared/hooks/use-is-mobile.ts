import { IsMobileContext } from '@shared/hocs/is-mobile-provider'
import { useContext } from 'react'

export function useIsMobile(): boolean {
    return useContext(IsMobileContext)
}
