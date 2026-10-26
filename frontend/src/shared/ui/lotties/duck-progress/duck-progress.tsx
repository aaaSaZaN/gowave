import { DotLottieReact } from '@lottiefiles/dotlottie-react'
import { panelUrl } from '@shared/lib/panel-path'
import type { FC } from 'react'

import type { IProps } from './interfaces/props.interface'

export const LottieDuckProgressShared: FC<IProps> = (props) => {
    const { width = 96, height = 115, autoplay = true, loop = true } = props

    return (
        <div style={{ width, height }}>
            <DotLottieReact
                autoplay={autoplay}
                loop={loop}
                renderConfig={{
                    autoResize: true,
                    devicePixelRatio: window.devicePixelRatio || 2
                }}
                src={panelUrl('/lotties/duck-progress.lottie')}
            />
        </div>
    )
}
