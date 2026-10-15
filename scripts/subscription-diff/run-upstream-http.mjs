// Serves /api/sub/* through upstream's real ResponseRulesMiddleware,
// SubscriptionController and SubscriptionService on Express 5 (CQRS buses
// stubbed from each scenario) and records status, headers and body:
//   TZ=UTC UPSTREAM_DIR=... $NODE24 scripts/subscription-diff/run-upstream-http.mjs \
//     backend-go/internal/subtpl/testdata/matrix.json | gzip -9 > backend-go/internal/subscription/testdata/http.json.gz
// The Go side (internal/subscription/diff_test.go) seeds a database from
// the same scenarios and compares the responses.
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
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

const require = createRequire(path.join(dir, 'package.json'));
const express = require('express');
const { HttpException } = require('@nestjs/common');

const jiti = loadUpstream(dir);
const up = (p) => jiti.import(`${dir}/src/${p}`);
const { load } = await jiti.import('js-yaml');
const { YAML_MERGE_SCHEMA } = await up('common/utils/yaml-merge-schema.util.ts');
const { ok, fail } = await up('common/types/result.type.ts');
const { ResolveProxyConfigService } = await up('modules/subscription-template/resolve-proxy/resolve-proxy-config.service.ts');
const { XrayGeneratorService } = await up('modules/subscription-template/generators/xray.generator.service.ts');
const { XrayJsonGeneratorService } = await up('modules/subscription-template/generators/xray-json.generator.service.ts');
const { SingBoxGeneratorService } = await up('modules/subscription-template/generators/singbox.generator.service.ts');
const { MihomoGeneratorService } = await up('modules/subscription-template/generators/mihomo.generator.service.ts');
const { ClashGeneratorService } = await up('modules/subscription-template/generators/clash.generator.service.ts');
const { RenderTemplatesService } = await up('modules/subscription-template/render-templates.service.ts');
const { ResponseRulesMatcherService } = await up('modules/subscription-response-rules/services/response-rules-matcher.service.ts');
const { ResponseRulesEncryptionService } = await up('modules/subscription-response-rules/services/response-rules-encryption.service.ts');
const { ResponseRulesMiddleware } = await up('modules/subscription-response-rules/middleware/response-rules.middleware.ts');
const { SubscriptionService } = await up('modules/subscription/subscription.service.ts');
const { SubscriptionController } = await up('modules/subscription/controllers/subscription.controller.ts');

const configService = { getOrThrow: (key) => ({ SUB_PUBLIC_DOMAIN: matrix.subPublicDomain })[key] };

// Hosts are a cross-section of the generator matrix.
const HOST_INDEXES = [0, 1, 2, 10, 16, 19, 25, 34, 45, 60, 81, 88, 93, 94, 95];
const hosts = HOST_INDEXES.map((i, position) => ({ ...matrix.hosts[i], viewPosition: position + 1 }));
hosts[1] = { ...hosts[1], xrayJsonTemplate: { log: { loglevel: 'debug' }, outbounds: [{ tag: 'direct', protocol: 'freedom' }] } };
hosts[3] = { ...hosts[3], isDisabled: true };
hosts[5] = { ...hosts[5], shuffleHost: true };
hosts[6] = { ...hosts[6], shuffleHost: true };

const rules = (extra = []) => ({
    version: '1',
    rules: [
        ...extra,
        { name: 'Browser', enabled: true, operator: 'AND', conditions: [{ headerName: 'accept', operator: 'CONTAINS', value: 'text/html', caseSensitive: true }], responseType: 'BROWSER' },
        { name: 'Mihomo', enabled: true, operator: 'AND', conditions: [{ headerName: 'user-agent', operator: 'REGEX', value: '^(?:flclash|mihomo|clash-verge)', caseSensitive: false }], responseType: 'MIHOMO' },
        { name: 'Stash', enabled: true, operator: 'AND', conditions: [{ headerName: 'user-agent', operator: 'REGEX', value: '^stash', caseSensitive: false }], responseType: 'STASH' },
        { name: 'Singbox', enabled: true, operator: 'OR', conditions: [{ headerName: 'user-agent', operator: 'STARTS_WITH', value: 'SFA', caseSensitive: true }, { headerName: 'user-agent', operator: 'CONTAINS', value: 'sing-box', caseSensitive: false }], responseType: 'SINGBOX' },
        { name: 'Clash', enabled: true, operator: 'AND', conditions: [{ headerName: 'user-agent', operator: 'REGEX', value: '^clash', caseSensitive: false }], responseType: 'CLASH' },
        { name: 'Disabled', enabled: false, operator: 'AND', conditions: [], responseType: 'BLOCK' },
        { name: 'Fallback Base64', enabled: true, operator: 'AND', conditions: [], responseType: 'XRAY_BASE64' },
    ],
});

const baseSettings = {
    serveJsonAtBaseSubscription: false,
    isShowCustomRemarks: true,
    randomizeHosts: false,
    customRemarks: matrix.settings.customRemarks,
    customResponseHeaders: { 'profile-title': 'rwEncodeBase64:{{USERNAME}} · {{TRAFFIC_LEFT}}', 'profile-update-interval': '12', 'support-url': 'https://t.me/support?u={{SHORT_UUID}}' },
    hwidSettings: { enabled: false, fallbackDeviceLimit: 2, maxDevicesAnnounce: 'Limit {{USERNAME}}' },
    responseRules: rules(),
};

const altTemplates = {
    XRAY_JSON: { name: 'Alt', templateType: 'XRAY_JSON', content: { log: { loglevel: 'warning' }, outbounds: [{ tag: 'direct', protocol: 'freedom' }] } },
    MIHOMO: { name: 'Alt', templateType: 'MIHOMO', content: 'mode: rule\nproxies:\nproxy-groups:\n  - name: Alt\n    type: select\n    proxies:\nrules:\n  - MATCH,Alt\n' },
};

const squad = {
    uuid: '66666666-6666-4666-8666-666666666666',
    subscriptionSettings: { serveJsonAtBaseSubscription: true },
    hostOverrides: { serverDescription: 'squad description', vlessRouteId: 7 },
    responseHeadersAdd: { 'x-squad': '{{USERNAME}}', 'profile-update-interval': '6' },
    responseHeadersRemove: ['support-url'],
    hwidSettings: null,
    customRemarks: null,
};

const UA = { 'user-agent': 'curl/8.7.1' };
const age = await jiti.import('age-encryption');
const AGE_RECIPIENT = await age.identityToRecipient(await age.generateIdentity());

const scenarios = [
    { name: 'base64-default', request: { path: '', headers: UA } },
    { name: 'no-user-agent', request: { path: '', headers: {} } },
    { name: 'unknown-user', request: { path: '', shortUuid: 'nobody', headers: UA } },
    { name: 'browser', request: { path: '', headers: { 'user-agent': 'Mozilla/5.0', accept: 'text/html' } } },
    { name: 'browser-unknown-user', request: { path: '', shortUuid: 'nobody', headers: { 'user-agent': 'Mozilla/5.0', accept: 'text/html' } } },
    { name: 'mihomo-rule', request: { path: '', headers: { 'user-agent': 'mihomo/1.19' } } },
    { name: 'mihomo-extended', request: { path: '', headers: { 'user-agent': 'FlClashX/0.3' } } },
    { name: 'stash-rule', request: { path: '', headers: { 'user-agent': 'Stash/3.0' } } },
    { name: 'singbox-or', request: { path: '', headers: { 'user-agent': 'my sing-box 1.11' } } },
    { name: 'clash-rule', request: { path: '', headers: { 'user-agent': 'clash.meta' } } },
    { name: 'path-json', request: { path: '/json', headers: UA } },
    { name: 'path-v2ray-json-extended', request: { path: '/v2ray-json', headers: { 'user-agent': 'Happ/3.1' } } },
    { name: 'path-singbox', request: { path: '/singbox', headers: UA } },
    { name: 'path-mihomo', request: { path: '/mihomo', headers: UA } },
    { name: 'path-stash', request: { path: '/stash', headers: UA } },
    { name: 'path-clash', request: { path: '/clash', headers: UA } },
    { name: 'path-uppercase-blocked', request: { path: '/JSON', headers: UA } },
    { name: 'path-disabled', settings: { responseRules: { ...rules(), settings: { disableSubscriptionAccessByPath: true } } }, request: { path: '/json', headers: UA } },
    { name: 'serve-json-happ', settings: { serveJsonAtBaseSubscription: true }, request: { path: '', headers: { 'user-agent': 'Happ/3.1' } } },
    { name: 'serve-json-curl', settings: { serveJsonAtBaseSubscription: true }, request: { path: '', headers: UA } },
    { name: 'serve-json-v2rayng', settings: { serveJsonAtBaseSubscription: true }, request: { path: '', headers: { 'user-agent': 'v2rayNG/1.9.30' } } },
    {
        name: 'serve-json-ignored',
        settings: { serveJsonAtBaseSubscription: true, responseRules: rules([{ name: 'happ', enabled: true, operator: 'AND', conditions: [{ headerName: 'user-agent', operator: 'STARTS_WITH', value: 'Happ', caseSensitive: true }], responseType: 'XRAY_BASE64', responseModifications: { ignoreServeJsonAtBaseSubscription: true } }]) },
        request: { path: '', headers: { 'user-agent': 'Happ/3.1' } },
    },
    {
        name: 'srr-modifications',
        templates: [altTemplates.XRAY_JSON],
        settings: {
            responseRules: rules([{ name: 'mods', enabled: true, operator: 'AND', conditions: [{ headerName: 'x-app', operator: 'EQUALS', value: 'mods', caseSensitive: true }], responseType: 'XRAY_JSON', responseModifications: { headers: [{ key: 'x-early', value: '{{USERNAME}}' }, { key: 'support-url', value: 'early' }], subscriptionTemplate: 'Alt', ignoreHostXrayJsonTemplate: true, excludeHostsByTags: ['EU'], additionalExtendedClientsRegex: ['^curl/'] } }]),
        },
        request: { path: '', headers: { ...UA, 'x-app': 'mods' } },
    },
    {
        name: 'srr-headers-to-end',
        settings: { responseRules: rules([{ name: 'end', enabled: true, operator: 'AND', conditions: [{ headerName: 'x-app', operator: 'EQUALS', value: 'end', caseSensitive: true }], responseType: 'XRAY_BASE64', responseModifications: { headers: [{ key: 'support-url', value: 'late' }, { key: 'content-disposition', value: 'inline' }], applyHeadersToEnd: true } }]) },
        request: { path: '', headers: { ...UA, 'x-app': 'end' } },
    },
    {
        name: 'srr-template-missing-falls-back',
        settings: { responseRules: rules([{ name: 'tpl', enabled: true, operator: 'AND', conditions: [], responseType: 'MIHOMO', responseModifications: { subscriptionTemplate: 'Nope' } }]) },
        request: { path: '', headers: UA },
    },
    {
        name: 'srr-respond-with-remarks',
        settings: { responseRules: rules([{ name: 'r', enabled: true, operator: 'AND', conditions: [], responseType: 'XRAY_BASE64', responseModifications: { respondWithRemarks: ['Go away {{USERNAME}}', 'Second'] } }]) },
        request: { path: '', headers: UA },
    },
    {
        name: 'srr-status-codes',
        settings: { responseRules: rules([{ name: '404', enabled: true, operator: 'AND', conditions: [{ headerName: 'user-agent', operator: 'NOT_REGEX', value: '^CURL', caseSensitive: false }], responseType: 'STATUS_CODE_404', responseModifications: { headers: [{ key: 'x-before-error', value: '1' }] } }]) },
        request: { path: '', headers: { 'user-agent': 'wget' } },
    },
    {
        name: 'srr-451',
        settings: { responseRules: rules([{ name: '451', enabled: true, operator: 'AND', conditions: [{ headerName: 'host', operator: 'ENDS_WITH', value: 'example', caseSensitive: true }], responseType: 'STATUS_CODE_451' }]) },
        request: { path: '', headers: UA },
    },
    {
        name: 'srr-block-no-match',
        settings: { responseRules: { version: '1', rules: [{ name: 'only', enabled: true, operator: 'AND', conditions: [{ headerName: 'x-missing', operator: 'NOT_EQUALS', value: 'x', caseSensitive: true }], responseType: 'XRAY_BASE64' }] } },
        request: { path: '', headers: UA },
    },
    { name: 'randomized', settings: { randomizeHosts: true }, request: { path: '', headers: UA } },
    { name: 'expired-user', user: { status: 'EXPIRED' }, request: { path: '', headers: UA } },
    { name: 'limited-no-remarks', user: { status: 'LIMITED' }, settings: { isShowCustomRemarks: false }, request: { path: '/json', headers: UA } },
    { name: 'expire-2099', user: { expireAt: '2099-01-01T00:00:00.000Z', trafficLimitStrategy: 'NO_RESET' }, request: { path: '', headers: UA } },
    { name: 'month-rolling', user: { trafficLimitStrategy: 'MONTH_ROLLING' }, request: { path: '', headers: UA } },
    { name: 'squad', user: { externalSquadUuid: squad.uuid }, squad, request: { path: '', headers: { 'user-agent': 'Happ/3.1' } } },
    { name: 'squad-template', user: { externalSquadUuid: squad.uuid }, squad, squadTemplates: { XRAY_JSON: 'Alt', MIHOMO: 'Alt' }, templates: [altTemplates.XRAY_JSON, altTemplates.MIHOMO], request: { path: '', headers: { 'user-agent': 'Happ/3.1' } } },
    { name: 'squad-template-mihomo', user: { externalSquadUuid: squad.uuid }, squad, squadTemplates: { MIHOMO: 'Alt' }, templates: [altTemplates.MIHOMO], request: { path: '/mihomo', headers: UA } },
    {
        name: 'squad-hwid-and-remarks',
        user: { externalSquadUuid: squad.uuid },
        squad: { ...squad, hwidSettings: { enabled: true, fallbackDeviceLimit: 1, maxDevicesAnnounce: null }, customRemarks: { ...matrix.settings.customRemarks, HWIDNotSupported: ['Squad: install the app'] }, responseHeadersAdd: {}, responseHeadersRemove: [] },
        request: { path: '', headers: UA },
    },
    { name: 'hwid-not-supported', settings: { hwidSettings: { ...baseSettings.hwidSettings, enabled: true } }, request: { path: '', headers: UA } },
    { name: 'hwid-not-supported-no-remarks', settings: { isShowCustomRemarks: false, hwidSettings: { ...baseSettings.hwidSettings, enabled: true } }, request: { path: '/json', headers: UA } },
    {
        name: 'hwid-max-reached',
        settings: { hwidSettings: { ...baseSettings.hwidSettings, enabled: true } },
        devices: ['existing-device-01', 'existing-device-02'],
        request: { path: '', headers: { ...UA, 'x-hwid': 'new-device-0003', 'x-device-os': 'Android' } },
    },
    {
        name: 'hwid-known-device',
        settings: { hwidSettings: { ...baseSettings.hwidSettings, enabled: true } },
        devices: ['existing-device-01', 'existing-device-02'],
        request: { path: '', headers: { ...UA, 'x-hwid': 'existing-device-02' } },
    },
    {
        name: 'hwid-new-device',
        settings: { hwidSettings: { ...baseSettings.hwidSettings, enabled: true } },
        request: { path: '', headers: { ...UA, 'x-hwid': 'brand-new-device' } },
    },
    { name: 'hwid-bad-header', settings: { hwidSettings: { ...baseSettings.hwidSettings, enabled: true } }, request: { path: '', headers: { ...UA, 'x-hwid': 'short' } } },
    { name: 'hwid-user-limit-zero', user: { hwidDeviceLimit: 0 }, settings: { hwidSettings: { ...baseSettings.hwidSettings, enabled: true } }, request: { path: '', headers: UA } },
    { name: 'hwid-user-limit', user: { hwidDeviceLimit: 3 }, devices: ['existing-device-01', 'existing-device-02'], settings: { hwidSettings: { ...baseSettings.hwidSettings, enabled: true } }, request: { path: '', headers: { ...UA, 'x-hwid': 'third-device-0' } } },
    {
        name: 'hwid-disabled-by-rule',
        settings: { hwidSettings: { ...baseSettings.hwidSettings, enabled: true }, responseRules: rules([{ name: 'nohwid', enabled: true, operator: 'AND', conditions: [], responseType: 'XRAY_BASE64', responseModifications: { disableHwidCheck: true } }]) },
        request: { path: '', headers: UA },
    },
    { name: 'no-custom-headers', settings: { customResponseHeaders: null }, request: { path: '', headers: UA } },
    {
        name: 'encrypted',
        bodyCheck: 'armor',
        settings: { responseRules: rules([{ name: 'enc', enabled: true, operator: 'AND', conditions: [], responseType: 'XRAY_BASE64', responseModifications: { encryption: { method: 'age1', key: AGE_RECIPIENT } } }]) },
        request: { path: '', headers: UA },
    },
    {
        name: 'encrypted-bad-key',
        settings: { responseRules: rules([{ name: 'enc', enabled: true, operator: 'AND', conditions: [], responseType: 'XRAY_BASE64', responseModifications: { encryption: { method: 'age1pq1', key: 'not-a-key' } } }]) },
        request: { path: '', headers: UA },
    },
];

function scenarioState(s) {
    const user = { ...matrix.user, externalSquadUuid: null, ...(s.user ?? {}) };
    const settings = { ...baseSettings, ...(s.settings ?? {}) };
    const templates = [
        ...Object.entries(matrix.templates).map(([templateType, content]) => ({ name: 'Default', templateType, content })),
        ...(s.templates ?? []),
    ];
    return { user, settings, templates, squad: s.squad ?? null, squadTemplates: s.squadTemplates ?? {}, devices: [...(s.devices ?? [])] };
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

function buildApp(state) {
    const templateService = {
        async getCachedTemplateByType(type, name = 'Default') {
            const found = state.templates.find((t) => t.name === name && t.templateType === type) ?? state.templates.find((t) => t.name === 'Default' && t.templateType === type);
            if (!found) throw new Error('Template not found');
            const parsed = typeof found.content === 'string' ? load(found.content, { schema: YAML_MERGE_SCHEMA, maxAliases: -1, maxTotalMergeKeys: -1 }) : found.content;
            return JSON.parse(JSON.stringify(parsed));
        },
    };
    const resolver = new ResolveProxyConfigService(configService);
    const xray = new XrayGeneratorService();
    const render = new RenderTemplatesService(resolver, new MihomoGeneratorService(templateService), new ClashGeneratorService(templateService), xray, new SingBoxGeneratorService(templateService), new XrayJsonGeneratorService(templateService));
    const user = toUser(state.user);
    const queryBus = {
        async execute(query) {
            switch (query.constructor.name) {
                case 'GetCachedSubscriptionSettingsQuery':
                    return structuredClone(state.settings);
                case 'GetUserByUniqueFieldQuery':
                    return query.field.shortUuid === user.shortUuid ? ok(user) : fail({ code: 'A025', message: 'User not found', httpCode: 404 });
                case 'GetCachedTemplateNameQuery':
                    if (query.templateType === 'XRAY_BASE64') return fail({ code: 'X', message: 'x' });
                    return state.squadTemplates[query.templateType] ? ok(state.squadTemplates[query.templateType]) : fail({ code: 'X', message: 'x' });
                case 'GetCachedExternalSquadSettingsQuery':
                    return state.squad ? structuredClone(state.squad) : null;
                case 'GetHostsForUserQuery':
                    return ok(structuredClone(hosts.filter((h) => (query.returnDisabledHosts || !h.isDisabled) && (query.returnHiddenHosts || !h.isHidden))));
                case 'CheckHwidExistsQuery':
                    return ok({ exists: state.devices.includes(query.hwid) });
            }
            throw new Error(`unexpected query ${query.constructor.name}`);
        },
    };
    const commandBus = {
        async execute(command) {
            if (command.constructor.name !== 'CreateWithAdvisoryLockCommand') throw new Error(command.constructor.name);
            const { hwid } = command.hwidUserDevice;
            if (state.devices.includes(hwid)) return ok({ status: 'EXISTS', hwidDevice: command.hwidUserDevice });
            if (state.devices.length >= command.deviceLimit) return ok({ status: 'LIMIT_REACHED', hwidDevice: null });
            state.devices.push(hwid);
            return ok({ status: 'CREATED', hwidDevice: command.hwidUserDevice });
        },
    };
    const queues = { async checkAndUpsertHwidDevice() {}, async addSubscriptionRequestRecord() {} };
    const matcher = new ResponseRulesMatcherService();
    const service = new SubscriptionService(queryBus, configService, commandBus, { emit() {} }, render, resolver, xray, queues, matcher);
    const controller = new SubscriptionController(service, new ResponseRulesEncryptionService());
    const middleware = new ResponseRulesMiddleware(queryBus, matcher);

    const app = express();
    app.use((req, _res, next) => {
        req.clientIp = req.headers['x-forwarded-for'] ?? '127.0.0.1';
        next();
    });
    const srr = (req, res, next) => middleware.use(req, res, next);
    app.get('/api/sub/:shortUuid', srr, (req, res, next) => controller.getSubscription(req.srrContext, req.params, res).catch(next));
    app.get('/api/sub/:shortUuid/:clientType', srr, (req, res, next) => controller.getSubscriptionByClientType(req.srrContext, req.params, res).catch(next));
    // Nest's BaseExceptionFilter for errors raised in middleware.
    app.use((err, _req, res, _next) => {
        if (err instanceof HttpException || typeof err?.getStatus === 'function') {
            const body = err.getResponse();
            res.status(err.getStatus()).json(typeof body === 'object' ? body : { statusCode: err.getStatus(), message: body });
            return;
        }
        res.status(500).json({ statusCode: 500, message: 'Internal server error' });
    });
    return app;
}

const IGNORED = new Set(['date', 'connection', 'keep-alive', 'x-powered-by']);

const results = [];
for (const s of scenarios) {
    const state = scenarioState(s);
    const app = buildApp(state);
    const server = await new Promise((resolve) => {
        const srv = app.listen(0, '127.0.0.1', () => resolve(srv));
    });
    const shortUuid = s.request.shortUuid ?? matrix.user.shortUuid;
    const headers = { host: 'sub.example', 'x-forwarded-for': '203.0.113.7', ...s.request.headers };
    const response = await new Promise((resolve, reject) => {
        // Raw http so fetch's own user-agent never leaks into the request.
        const http = require('node:http');
        const req = http.request({ host: '127.0.0.1', port: server.address().port, path: `/api/sub/${shortUuid}${s.request.path}`, headers }, (res) => {
            const chunks = [];
            res.on('data', (c) => chunks.push(c));
            res.on('end', () => resolve({ status: res.statusCode, headers: res.headers, body: Buffer.concat(chunks).toString('utf8') }));
        });
        req.on('error', reject);
        req.end();
    });
    server.close();
    const kept = {};
    for (const [k, v] of Object.entries(response.headers)) if (!IGNORED.has(k)) kept[k] = Array.isArray(v) ? v.join(', ') : v;
    results.push({
        name: s.name,
        bodyCheck: s.bodyCheck ?? 'exact',
        state: { ...scenarioState(s) },
        request: { shortUuid, path: s.request.path, headers },
        response: { status: response.status, headers: kept, body: response.body },
    });
}
console.log(JSON.stringify({ now: matrix.now, subPublicDomain: matrix.subPublicDomain, hosts, results }, null, 1));
