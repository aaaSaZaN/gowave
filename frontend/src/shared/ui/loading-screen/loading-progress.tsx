import { nprogress } from '@shared/ui/navigation-progress'
import { useEffect } from 'react'

export function LoadingProgress() {
    useEffect(() => {
        nprogress.start()
        return () => nprogress.complete()
    }, [])

    return null
}
