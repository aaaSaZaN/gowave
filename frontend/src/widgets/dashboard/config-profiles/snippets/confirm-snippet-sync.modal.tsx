import { openApplyToNodesModal } from '@shared/_modals/universal'
import { TbCode } from 'react-icons/tb'

export const openConfirmSnippetSyncModal = (onConfirm: () => void) => {
    openApplyToNodesModal({
        IconComponent: TbCode,
        iconColor: 'teal',
        delay: 250,
        onApply: onConfirm
    })
}
