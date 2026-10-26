import { useEffect } from 'react'

import { nprogress } from '@shared/ui/navigation-progress'

export function LoadingProgress() {
    useEffect(() => {
        nprogress.start()
        return () => nprogress.complete()
    }, [])

    return <></>
}
