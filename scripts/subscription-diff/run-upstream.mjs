// Renders every case of matrix.json with upstream's RenderTemplatesService
// and writes the golden outputs the Go test compares against:
//   TZ=UTC UPSTREAM_DIR=... $NODE24 scripts/subscription-diff/run-upstream.mjs \
//     backend-go/internal/subtpl/testdata/matrix.json | gzip -9 > backend-go/internal/subtpl/testdata/golden.json.gz
// Time is pinned to matrix.now and Math.random to 0 so shuffles and random
// picks are deterministic on both sides.
import { readFileSync } from 'node:fs';
import { loadUpstream } from './load-upstream.mjs';

const dir = process.env.UPSTREAM_DIR;
const matrix = JSON.parse(readFileSync(process.argv[2], 'utf8'));

const RealDate = Date;
const fixed = new RealDate(matrix.now).getTime();
globalThis.Date = class extends RealDate {
    constructor(...args) {
        super(...(args.length ? args : [fixed]));
    }
    static now() {
        return fixed;
    }
};
Math.random = () => 0;

const jiti = loadUpstream(dir);
const mod = (p) => jiti.import(`${dir}/src/modules/subscription-template/${p}`);
const { load } = await jiti.import('js-yaml');
const { YAML_MERGE_SCHEMA } = await jiti.import(`${dir}/src/common/utils/yaml-merge-schema.util.ts`);
const { ResolveProxyConfigService } = await mod('resolve-proxy/resolve-proxy-config.service.ts');
const { XrayGeneratorService } = await mod('generators/xray.generator.service.ts');
const { XrayJsonGeneratorService } = await mod('generators/xray-json.generator.service.ts');
const { SingBoxGeneratorService } = await mod('generators/singbox.generator.service.ts');
const { MihomoGeneratorService } = await mod('generators/mihomo.generator.service.ts');
const { ClashGeneratorService } = await mod('generators/clash.generator.service.ts');
const { RenderTemplatesService } = await mod('render-templates.service.ts');

const configService = { getOrThrow: (key) => ({ SUB_PUBLIC_DOMAIN: matrix.subPublicDomain })[key] };

function templateService(overrides = {}) {
    const templates = { ...matrix.templates, ...overrides };
    return {
        async getCachedTemplateByType(type) {
            const raw = templates[type];
            const parsed = typeof raw === 'string' ? load(raw, { schema: YAML_MERGE_SCHEMA, maxAliases: -1, maxTotalMergeKeys: -1 }) : raw;
            // Upstream caches templates as JSON.
            return JSON.parse(JSON.stringify(parsed));
        },
    };
}

function toUser(u) {
    return {
        ...u,
        id: BigInt(u.id),
        trafficLimitBytes: BigInt(u.trafficLimitBytes),
        telegramId: u.telegramId === null ? null : BigInt(u.telegramId),
        expireAt: new Date(u.expireAt),
        createdAt: new Date(u.createdAt),
        lastTrafficResetAt: u.lastTrafficResetAt ? new Date(u.lastTrafficResetAt) : null,
        userTraffic: { usedTrafficBytes: BigInt(u.usedTrafficBytes), lifetimeUsedTrafficBytes: BigInt(u.lifetimeUsedTrafficBytes) },
    };
}

const golden = {};
for (const c of matrix.cases) {
    const tpl = templateService(c.templates);
    const render = new RenderTemplatesService(
        new ResolveProxyConfigService(configService),
        new MihomoGeneratorService(tpl),
        new ClashGeneratorService(tpl),
        new XrayGeneratorService(),
        new SingBoxGeneratorService(tpl),
        new XrayJsonGeneratorService(tpl),
    );
    for (const type of c.types) {
        for (const extended of c.extended ?? [false]) {
            for (const ignore of c.ignoreHostXrayJsonTemplate ?? [false]) {
                const indexes = c.hostIndexes ?? matrix.hosts.map((_, i) => i);
                const hosts = indexes.map((i) => structuredClone({ ...matrix.hosts[i], ...(c.hostPatch?.[i] ?? {}) }));
                const settings = { ...matrix.settings, ...(c.settingsPatch ?? {}) };
                const result = await render.generateSubscription({
                    srrContext: {
                        matchedResponseType: type,
                        isExtendedClient: extended,
                        overrideTemplateName: undefined,
                        subscriptionSettings: settings,
                        excludeHostsByTags: c.excludeHostsByTags ? new Set(c.excludeHostsByTags) : undefined,
                        ignoreHostXrayJsonTemplate: ignore,
                    },
                    user: toUser({ ...matrix.user, ...(c.userPatch ?? {}) }),
                    hosts,
                    hostsOverrides: c.hostOverrides,
                    fallbackOptions: c.fallback,
                });
                golden[`${c.name}/${type}/${extended}/${ignore}`] = { body: result.subscription, contentType: result.contentType };
            }
        }
    }
}
console.log(JSON.stringify(golden, null, 1));
