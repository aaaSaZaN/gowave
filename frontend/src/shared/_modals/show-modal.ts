import NiceModal from '@ebay/nice-modal-react'
import type { ModalArgs, ModalId } from './modal-registry'

export function showModal<K extends ModalId>(
    id: K,
    ...[args]: Record<never, never> extends ModalArgs<K>
        ? [args?: ModalArgs<K>]
        : [args: ModalArgs<K>]
) {
    return NiceModal.show(id, args as Record<string, unknown>)
}

export function hideModal(id: ModalId) {
    return NiceModal.hide(id)
}
