import { Button, CopyButton, Group, Skeleton } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { PiCheck, PiTerminal } from 'react-icons/pi'

import { useGetNodeSecretKey } from '@shared/api/hooks'

interface IProps {
    port?: number
}

const NODE_INSTALL_URL = 'https://raw.githubusercontent.com/aaaSaZaN/remnanode-go/main/install.sh'

// The installer is downloaded in full before sh runs it: no curl | sh, and
// sh -c also works on routers without bash.
const buildNodeInstallCommand = (secretKey: string, port?: number) =>
    `sudo SECRET_KEY='${secretKey}' NODE_PORT=${port ?? 2222} sh -c "$(curl -fsSL ${NODE_INSTALL_URL})"`

export const CopyNodeInstallCommandWidget = ({ port }: IProps) => {
    const { data: secretKey, isLoading: isSecretKeyLoading } = useGetNodeSecretKey()
    const { t } = useTranslation()

    if (isSecretKeyLoading || !secretKey) {
        return <Skeleton height={40} />
    }

    return (
        <Group mt="lg">
            <CopyButton
                timeout={2000}
                value={buildNodeInstallCommand(secretKey.secretKey.trim(), port)}
            >
                {({ copied, copy }) => (
                    <Button
                        color={copied ? 'teal' : 'gray'}
                        fullWidth
                        leftSection={copied ? <PiCheck size={18} /> : <PiTerminal size={18} />}
                        onClick={copy}
                        size="md"
                    >
                        {t('copy-node-install-command.widget.copy-install-command')}
                    </Button>
                )}
            </CopyButton>
        </Group>
    )
}
