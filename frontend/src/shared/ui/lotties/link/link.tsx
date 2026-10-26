import { DotLottieReact } from '@lottiefiles/dotlottie-react'
import { panelUrl } from '@shared/lib/panel-path'
import type { FC } from 'react'

import type { IProps } from './interfaces/props.interface'

export const LottieLinkShared: FC<IProps> = (props) => {
    const { width = 96, height = 115 } = props

    return (
        <div style={{ width, height }}>
            <DotLottieReact
                autoplay
                loop
                renderConfig={{
                    autoResize: true,
                    devicePixelRatio: window.devicePixelRatio || 2
                }}
                src={panelUrl('/lotties/satellite.lottie')}
            />
        </div>
    )
}
