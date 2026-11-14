import { rem } from '@mantine/core'
import { TbBrandTelegram } from 'react-icons/tb'

import { HeaderControl } from './HeaderControl'

export function TelegramControl() {
    return (
        <HeaderControl
            aria-label="Telegram"
            component="a"
            href={__TELEGRAM_CHAT_URL__}
            rel="noopener noreferrer"
            target="_blank"
            title="Telegram"
        >
            <TbBrandTelegram aria-hidden="true" style={{ width: rem(22), height: rem(22) }} />
        </HeaderControl>
    )
}
