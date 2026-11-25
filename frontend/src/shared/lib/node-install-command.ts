const NODE_INSTALL_URL = 'https://raw.githubusercontent.com/aaaSaZaN/gowave-node/main/install.sh'

// Download the complete migration installer before the shell executes it.
export const buildNodeInstallCommand = (secretKey: string, port?: number) =>
    `sudo SECRET_KEY='${secretKey.replaceAll("'", "'\\''")}' NODE_PORT=${port ?? 2222} sh -c "$(curl -fsSL ${NODE_INSTALL_URL})"`
