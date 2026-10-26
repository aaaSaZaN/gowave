import type { UseFormReturnType } from '@mantine/form'
import type { CreateNodeCommand, UpdateNodeCommand } from '@remnawave/backend-contract'
import { NodeIpsEditor } from '@shared/ui/node-ips'
import type { ForwardRefComponent, HTMLMotionProps, Variants } from 'motion/react'

interface IProps<T extends CreateNodeCommand.RequestBody | UpdateNodeCommand.RequestBody> {
    cardVariants: Variants
    form: UseFormReturnType<T>
    motionWrapper: ForwardRefComponent<HTMLDivElement, HTMLMotionProps<'div'>>
}

export const NodeIpsCard = <
    T extends CreateNodeCommand.RequestBody | UpdateNodeCommand.RequestBody
>(
    props: IProps<T>
) => {
    const { cardVariants, form, motionWrapper } = props

    const MotionWrapper = motionWrapper

    return (
        <MotionWrapper variants={cardVariants}>
            <NodeIpsEditor form={form} key={form.key('ips')} />
        </MotionWrapper>
    )
}
