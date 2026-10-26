import clsx from '@shared/lib/classes'
import type { ReactNode, Ref } from 'react'

import styles from './CodeEditor.module.css'

interface Props {
    children: ReactNode
    className?: string
    ref?: Ref<HTMLDivElement>
}

export function EditorFooter(props: Props) {
    const { children, className, ref } = props

    return (
        <div className={clsx(styles.footer, className)} ref={ref}>
            {children}
        </div>
    )
}
