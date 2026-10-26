import { ActionIcon } from '@mantine/core'
import { showModal } from '@shared/_modals/show-modal'
import { PiUserCircle } from 'react-icons/pi'

interface IProps {
    userId: number
}

export function ResolveUserActionShared(props: IProps) {
    const { userId } = props

    return (
        <ActionIcon
            onClick={() => showModal('users_viewUserModal', { userId: userId })}
            size="input-sm"
            variant="soft"
        >
            <PiUserCircle size="1.5rem" />
        </ActionIcon>
    )
}
