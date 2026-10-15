// Writes the deterministic subscription matrix used by the differential test:
//   UPSTREAM_DIR=... node scripts/subscription-diff/matrix.mjs > backend-go/internal/subtpl/testdata/matrix.json
// Each case lists hosts (host row + raw inbound), user and settings overrides,
// and the response types to render. Randomized host values ("a,b", "*") are
// avoided except where the harness pins Math.random.
import { loadUpstream } from './load-upstream.mjs';

const dir = process.env.UPSTREAM_DIR;
const jiti = loadUpstream(dir);
const defaults = await jiti.import(dir + '/src/modules/subscription-template/constants/default-templates.ts');

const X25519_PRIVATE = 'aOHwDZ7ofkXQsQ8VGm6TvUhw3yfh7dQvl7nyH4UBhUo';
const MLDSA_SEED = 'AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyA=';
const MLKEM_SEED = Buffer.alloc(64, 7).toString('base64url');
const X25519_DECRYPTION = Buffer.alloc(32, 9).toString('base64url');

const baseHost = (n) => ({
    uuid: `00000000-0000-4000-8000-${String(n).padStart(12, '0')}`,
    viewPosition: n,
    remark: `host-${n}`,
    address: `h${n}.example.com`,
    port: 443,
    path: null,
    sni: null,
    host: null,
    alpn: null,
    fingerprint: null,
    securityLayer: 'DEFAULT',
    xhttpExtraParams: null,
    muxParams: null,
    sockoptParams: null,
    finalMask: null,
    isDisabled: false,
    serverDescription: null,
    pinnedPeerCertSha256: null,
    verifyPeerCertByName: null,
    mihomoIpVersion: null,
    tags: [],
    isHidden: false,
    overrideSniFromAddress: false,
    keepSniBlank: false,
    vlessRouteId: null,
    shuffleHost: false,
    mihomoX25519: false,
    configProfileUuid: '11111111-1111-4111-8111-111111111111',
    configProfileInboundUuid: null,
    xrayJsonTemplateUuid: null,
    excludeFromSubscriptionTypes: [],
    nodes: [],
    mapper: {},
    internalSquadsMode: 'EXCLUDE',
    internalSquads: [],
    xrayJsonTemplate: null,
});

const settingsFor = {
    vless: () => ({ clients: [], decryption: 'none' }),
    trojan: () => ({ clients: [] }),
    shadowsocks: () => ({ clients: [], method: 'chacha20-ietf-poly1305', network: 'tcp,udp' }),
    ss2022: () => ({ clients: [], method: '2022-blake3-aes-256-gcm', password: 'c2VydmVyLXBhc3N3b3JkLTMyLWJ5dGVzLWxvbmchIQ==', network: 'tcp,udp' }),
    vlessEnc: () => ({ clients: [], decryption: `mlkem768x25519plus.native.600s.${X25519_DECRYPTION}.${MLKEM_SEED}` }),
    hysteria: () => ({ clients: [], version: 2 }),
};

const transports = {
    tcp: () => ({ network: 'tcp' }),
    raw: () => ({ network: 'raw', rawSettings: {} }),
    tcpHttp: () => ({ network: 'tcp', tcpSettings: { header: { type: 'http', request: { version: '1.1', method: 'GET', path: ['/a', '/b'], headers: { Host: ['x.example.com'] } } } } }),
    tcpHttpDefault: () => ({ network: 'tcp', tcpSettings: { header: { type: 'http' } } }),
    ws: () => ({ network: 'ws', wsSettings: { path: '/ws?ed=2048', host: 'ws.example.com', headers: { 'User-Agent': 'probe', Host: 'hdr.example.com' }, heartbeatPeriod: 30 } }),
    httpupgrade: () => ({ network: 'httpupgrade', httpupgradeSettings: { path: '/up', host: 'up.example.com', headers: { 'X-Custom': '1' } } }),
    grpc: () => ({ network: 'grpc', grpcSettings: { serviceName: 'svc', multiMode: true, authority: 'auth.example.com' } }),
    xhttp: () => ({ network: 'xhttp', xhttpSettings: { path: '/x', host: 'x.example.com', mode: 'packet-up', extra: { xPaddingBytes: '100-1000', noGRPCHeader: true, xmux: { maxConcurrency: '16-32', hMaxRequestTimes: 600 }, headers: { 'X-Test': 'y' }, downloadSettings: { address: 'dl.example.com', port: 443, network: 'xhttp', security: 'reality', realitySettings: { publicKey: 'pk', shortId: 'ab' }, xhttpSettings: { path: '/dl', host: 'dl.example.com', extra: { xmux: { maxConnections: 4 } } } } } } }),
    kcp: () => ({ network: 'kcp', kcpSettings: { mtu: 1400, tti: 20, congestion: true } }),
};

const securities = {
    none: () => ({ security: 'none' }),
    tls: () => ({ security: 'tls', tlsSettings: { serverName: 'tls.example.com', alpn: ['h2', 'http/1.1'], fingerprint: 'firefox', enableSessionResumption: true, cipherSuites: 'TLS_AES_128_GCM_SHA256', echConfigList: 'AEn+DQBF', echForceQuery: 'full' } }),
    reality: () => ({ security: 'reality', realitySettings: { privateKey: X25519_PRIVATE, shortIds: ['a1b2', 'c3'], serverNames: ['reality.example.com', 'b.example.com'], spiderX: '/spider', fingerprint: 'safari', mldsa65Seed: MLDSA_SEED, target: 'reality.example.com:443' } }),
};

let counter = 0;
const hosts = [];
const add = (protocol, transport, security, patch = {}, inboundPatch = {}) => {
    counter++;
    const tag = `IN_${counter}`;
    const proto = protocol === 'ss2022' ? 'shadowsocks' : protocol === 'vlessEnc' ? 'vless' : protocol;
    const rawInbound = {
        tag,
        port: 443,
        protocol: proto,
        settings: settingsFor[protocol](),
        streamSettings: { ...transports[transport](), ...securities[security]() },
        ...inboundPatch,
    };
    hosts.push({ ...baseHost(counter), remark: `${protocol}-${transport}-${security}`, inboundTag: tag, rawInbound, ...patch });
};

// Protocol × transport × security core matrix.
for (const protocol of ['vless', 'trojan', 'shadowsocks']) {
    for (const transport of ['tcp', 'raw', 'tcpHttp', 'ws', 'httpupgrade', 'grpc', 'xhttp', 'kcp']) {
        for (const security of ['none', 'tls', 'reality']) {
            add(protocol, transport, security);
        }
    }
}
add('ss2022', 'tcp', 'none');
add('vlessEnc', 'xhttp', 'none');
add('vlessEnc', 'tcp', 'tls');
add('hysteria', 'tcp', 'tls', {}, { streamSettings: { network: 'hysteria', security: 'tls', tlsSettings: { serverName: 'hy.example.com', alpn: ['h3'] }, finalmask: { quicParams: { brutalUp: '100 mbps', brutalDown: 200, udpHop: { ports: '20000-30000,40000', interval: '30' } }, udp: [{ type: 'salamander', settings: { password: 'obfs-pass', packetSize: '100-200' } }] } } });
add('hysteria', 'tcp', 'none', {}, { streamSettings: { network: 'hysteria', hysteriaSettings: { version: 2 }, finalmask: { udp: [{ type: 'salamander', settings: { password: 'salt' } }] } } });
add('vless', 'tcpHttpDefault', 'none');
add('vless', 'tcp', 'tls', {}, { settings: { clients: [], decryption: 'none', flow: '' } });
add('vless', 'ws', 'tls', {}, { settings: { clients: [], decryption: 'none', flow: 'xtls-rprx-vision' } });

// Host-level overrides on top of the matrix.
const overrideHosts = [
    ['vless', 'ws', 'tls', { sni: 'sni.example.com', host: 'host.example.com', path: '/override?ed=4096', alpn: 'h3', fingerprint: 'chrome', pinnedPeerCertSha256: 'abcdef', verifyPeerCertByName: 'verify.example.com' }],
    ['vless', 'tcp', 'reality', { keepSniBlank: true, vlessRouteId: 4660, fingerprint: 'random' }],
    ['trojan', 'grpc', 'tls', { overrideSniFromAddress: true, address: '2001:db8::1', host: 'grpc-auth.example.com', path: 'grpc-svc' }],
    ['vless', 'xhttp', 'tls', { xhttpExtraParams: { scMaxEachPostBytes: 1000000, xmux: { cMaxReuseTimes: '64-128' } }, path: '/hx', host: 'hx.example.com' }],
    ['vless', 'tcp', 'tls', { securityLayer: 'NONE' }],
    ['trojan', 'ws', 'none', { securityLayer: 'TLS', sni: 'forced.example.com' }],
    ['vless', 'ws', 'none', { muxParams: { enabled: true, concurrency: 8, smux: { enabled: true, protocol: 'h2mux', 'max-connections': '4', 'min-streams': 2, padding: true, 'brutal-opts': { enabled: true, up: '50', down: 100 } } }, sockoptParams: { tcpFastOpen: true, mark: 255 } }],
    ['vless', 'tcp', 'reality', { finalMask: { tcp: [{ type: 'fragment', settings: { packets: 'tlshello' } }] }, mihomoX25519: true, mihomoIpVersion: 'ipv4-prefer' }],
    ['shadowsocks', 'tcp', 'none', { serverDescription: 'Описание сервера 🇩🇪' }],
    ['vless', 'tcp', 'tls', { remark: '{{USERNAME}} · {{DAYS_LEFT}}d · {{TRAFFIC_LEFT}} · {{STATUS:ACTIVE=On}} · {{NEXT_TRAFFIC_RESET_AT:format=YYYY-MM-DD HH:mm}}' }],
    ['vless', 'tcp', 'tls', { remark: 'dup' }],
    ['vless', 'tcp', 'tls', { remark: 'dup' }],
    ['vless', 'tcp', 'tls', { remark: 'dup' }],
    ['trojan', 'tcp', 'tls', { excludeFromSubscriptionTypes: ['XRAY_BASE64', 'SINGBOX'] }],
    ['vless', 'grpc', 'reality', { isHidden: true, tags: ['HIDDEN', 'EU'] }],
    ['vless', 'tcp', 'reality', { tags: ['EU'], mapper: {
        base64: [{ op: 'set', to: 'fp', value: 'qq' }, { op: 'copy', from: '$host.metadata.inboundTag', to: '$link.remark' }, { op: 'unset', to: 'sid' }],
        xrayJson: [{ op: 'set', to: 'streamSettings.sockopt.mark', value: 7 }, { op: 'copy', from: 'port', to: 'settings.vnext[0].port' }, { op: 'unset', to: 'mux' }],
        mihomo: [{ op: 'set', to: 'tfo', value: true }, { op: 'copy', from: '$host.securityOptions.publicKey', to: 'reality-opts.public-key' }],
        singbox: [{ op: 'set', to: 'tls.utls.fingerprint', value: 'edge' }, { op: 'set', to: 'tag.blocked', value: 1 }],
    } }],
];
for (const [protocol, transport, security, patch] of overrideHosts) add(protocol, transport, security, patch);

const user = {
    id: 42,
    shortUuid: 'abcdefghijklmnop',
    username: 'matrix_user',
    status: 'ACTIVE',
    trafficLimitBytes: 107374182400,
    trafficLimitStrategy: 'MONTH',
    usedTrafficBytes: 1610612736,
    lifetimeUsedTrafficBytes: 9876543210,
    expireAt: '2026-04-15T10:00:00.000Z',
    createdAt: '2026-01-20T08:30:00.000Z',
    lastTrafficResetAt: '2026-03-01T00:20:00.000Z',
    email: 'user@example.com',
    telegramId: 123456789,
    tag: 'VIP',
    description: 'matrix',
    hwidDeviceLimit: null,
    vlessUuid: '9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d',
    trojanPassword: 'trojan-password-0123456789abcdef',
    ssPassword: 'ss-password-0123456789abcdefghij',
};

const settings = {
    isShowCustomRemarks: true,
    customRemarks: {
        expiredUsers: ['⌛ Expired {{USERNAME}}', 'Renew at {{SUBSCRIPTION_URL}}'],
        limitedUsers: ['🚧 Limited {{TRAFFIC_USED}}'],
        disabledUsers: ['🚫 Disabled'],
        emptyHosts: ['→ No hosts', '{"finalRemark":"json-fallback","address":"1.2.3.4","port":8443,"protocol":"trojan","protocolOptions":{"password":"p"},"transport":"tcp","transportOptions":{"header":null},"security":"tls","securityOptions":{"serverName":"j.example.com","fingerprint":"chrome","alpn":""},"streamOverrides":{"finalMask":null,"sockopt":null},"mux":null,"clientOverrides":{"mapper":{}},"metadata":{"uuid":"00000000-0000-4000-8000-ffffffffffff","tags":[],"excludeFromSubscriptionTypes":[],"isHidden":false}}'],
        HWIDMaxDevicesExceeded: ['Too many devices ({{SS_HWID_LIMIT}})'],
        HWIDNotSupported: ['Client not supported'],
    },
    hwidSettings: { enabled: true, fallbackDeviceLimit: 3 },
};

const injectorTemplate = {
    ...defaults.DEFAULT_TEMPLATE_XRAY_JSON,
    remnawave: {
        addVirtualHostAsOutbound: true,
        injectHosts: [
            { selector: { type: 'sameTagAsRecipient' }, selectFrom: 'ALL', tagPrefix: 'eu' },
            { selector: { type: 'remarkRegex', pattern: '^vless-ws' }, selectFrom: 'NOT_HIDDEN', useHostRemarkAsTag: true },
            { selector: { type: 'uuids', values: [hosts[hosts.length - 2].uuid] }, useHostTagAsTag: true },
        ],
    },
};

const types = ['XRAY_BASE64', 'XRAY_JSON', 'SINGBOX', 'MIHOMO', 'STASH', 'CLASH'];
const cases = [
    { name: 'matrix', types, extended: [false, true] },
    { name: 'expired', types, userPatch: { status: 'EXPIRED' } },
    { name: 'limited', types: ['XRAY_BASE64', 'MIHOMO'], userPatch: { status: 'LIMITED' } },
    { name: 'disabled-no-remarks', types: ['XRAY_BASE64'], userPatch: { status: 'DISABLED' }, settingsPatch: { isShowCustomRemarks: false } },
    { name: 'empty-hosts', types, hostIndexes: [] },
    { name: 'hwid-max', types: ['XRAY_BASE64', 'SINGBOX'], fallback: { showHwidMaxDeviceRemarks: true } },
    { name: 'respond-with', types: ['XRAY_JSON'], fallback: { respondWithRemarks: ['custom {{ID}}', 'rwEncodeBase64:{{EMAIL}}'] } },
    { name: 'external-squad', types: ['XRAY_BASE64', 'XRAY_JSON'], hostOverrides: { vlessRouteId: 255, serverDescription: 'squad description' }, extended: [true] },
    { name: 'exclude-tags', types: ['XRAY_BASE64'], excludeHostsByTags: ['EU'] },
    { name: 'injector', types: ['XRAY_JSON'], templates: { XRAY_JSON: injectorTemplate } },
    { name: 'host-template', types: ['XRAY_JSON'], hostPatch: { 0: { xrayJsonTemplate: { log: { loglevel: 'debug' }, outbounds: [{ tag: 'direct', protocol: 'freedom' }] } } }, ignoreHostXrayJsonTemplate: [false, true] },
];

console.log(
    JSON.stringify(
        {
            now: '2026-03-04T12:00:00.000Z',
            subPublicDomain: 'sub.example.com',
            user,
            settings,
            hosts,
            templates: {
                XRAY_JSON: defaults.DEFAULT_TEMPLATE_XRAY_JSON,
                SINGBOX: defaults.DEFAULT_TEMPLATE_SINGBOX,
                MIHOMO: defaults.DEFAULT_TEMPLATE_MIHOMO,
                STASH: defaults.DEFAULT_TEMPLATE_STASH,
                CLASH: defaults.DEFAULT_TEMPLATE_CLASH,
            },
            cases,
        },
        null,
        1,
    ),
);
