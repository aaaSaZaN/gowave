import { Loader, Menu } from '@mantine/core'
import { modals } from '@mantine/modals'
import { hideModal } from '@shared/_modals/show-modal'
import { QueryKeys, useDeleteUser } from '@shared/api/hooks'
import { queryClient } from '@shared/api/query-client'
import { runAsync } from '@shared/lib/run-async'
import { useTranslation } from 'react-i18next'
import { TbTrash } from 'react-icons/tb'

interface IProps {
    userId: number
}

export function DeleteUserFeature(props: IProps) {
    const { userId } = props
    const { t } = useTranslation()

    const { mutate: deleteUser, isPending: isDeleteUserPending } = useDeleteUser({
        mutationFns: {
            onSuccess: () => {
                runAsync(hideModal('users_viewUserModal'))

                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.users.getAllUsers._def
                    })
                )
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.users.getUserTags.queryKey
                    })
                )
                runAsync(
                    queryClient.refetchQueries({
                        queryKey: QueryKeys.system.getSystemStats.queryKey
                    })
                )
            }
        }
    })

    const handleDeleteUser = () => {
        deleteUser({
            route: {
                userId: userId
            }
        })
    }

    const openModal = () =>
        modals.openConfirmModal({
            title: t('common.action.confirm-action'),
            children: t('common.message.confirm-action-description'),
            labels: {
                confirm: t('common.action.delete'),
                cancel: t('common.action.cancel')
            },
            centered: true,
            confirmProps: { color: 'red', variant: 'soft' },
            cancelProps: {
                variant: 'subtle'
            },
            onConfirm: () => handleDeleteUser()
        })

    return (
        <Menu.Item
            color="red"
            leftSection={
                isDeleteUserPending ? <Loader color="red" size={16} /> : <TbTrash size={16} />
            }
            onClick={openModal}
        >
            {t('common.action.delete')}
        </Menu.Item>
    )
}
