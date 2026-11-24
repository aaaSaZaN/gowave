export function generateDefaultXrayConfig() {
    const randomNumber = Math.floor(Math.random() * 999999) + 1

    return {
        log: {
            loglevel: 'info'
        },
        inbounds: [
            {
                tag: `Shadowsocks_${randomNumber}`,
                port: 1234,
                protocol: 'shadowsocks',
                settings: {
                    clients: [],
                    method: 'chacha20-ietf-poly1305',
                    network: 'tcp,udp'
                },
                sniffing: {
                    enabled: true,
                    destOverride: ['http', 'tls', 'quic']
                }
            }
        ],
        outbounds: [
            {
                protocol: 'freedom',
                tag: 'DIRECT'
            },
            {
                protocol: 'blackhole',
                tag: 'BLOCK'
            }
        ],
        routing: {
            rules: []
        }
    }
}
