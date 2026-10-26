import { Group, Modal, type ModalProps, useProps } from '@mantine/core'
import classes from '@shared/constants/theme/overrides/modal/modal.module.css'
import type { ReactNode } from 'react'

interface IProps {
    modalProps: Omit<ModalProps, 'children' | 'title'>
    children: ReactNode
    title: ReactNode
    buttons?: ReactNode
}

export const CompoundModalShared = (props: IProps) => {
    const { modalProps, children, title, buttons } = props

    const rootProps = useProps('Modal', {}, modalProps)

    return (
        <Modal.Root {...rootProps} className={classes.modalRoot}>
            <Modal.Overlay />
            <Modal.Content className={classes.modalContent}>
                <Modal.Header className={classes.modalHeader}>
                    <Modal.Title>{title}</Modal.Title>
                    <Group gap="xs" justify="flex-end" wrap="nowrap">
                        {buttons}
                        <Modal.CloseButton />
                    </Group>
                </Modal.Header>
                <Modal.Body className={classes.modalBody}>{children}</Modal.Body>
            </Modal.Content>
        </Modal.Root>
    )
}
