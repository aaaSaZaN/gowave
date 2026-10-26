import NiceModal from '@ebay/nice-modal-react'
import { logoutEvents } from '@shared/emitters'
import { useContext, useEffect, useEffectEvent } from 'react'

export function NiceModalAutoClose() {
    const modals = useContext(NiceModal.NiceModalContext)

    const closeAll = useEffectEvent(() => {
        Object.keys(modals).forEach((id) => {
            NiceModal.remove(id)
        })
    })

    useEffect(() => logoutEvents.subscribe(closeAll), [])

    return null
}
