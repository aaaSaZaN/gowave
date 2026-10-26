import { preventBackScrollTables } from '@shared/utils/misc'
import { useLayoutEffect } from 'react'

export function usePreventTableBackScroll() {
    useLayoutEffect(() => {
        document.body.addEventListener('wheel', preventBackScrollTables, { passive: false })
        return () => {
            document.body.removeEventListener('wheel', preventBackScrollTables)
        }
    }, [])
}
