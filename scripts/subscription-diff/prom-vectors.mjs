// Replays metric operations on upstream's METRIC_PROVIDERS with prom-client
// and records the exposition text after each snapshot:
//   UPSTREAM_DIR=... $NODE24 scripts/subscription-diff/prom-vectors.mjs > backend-go/internal/metrics/testdata/prom-vectors.json
import { createRequire } from 'node:module';
import path from 'node:path';
import { loadUpstream } from './load-upstream.mjs';

const dir = process.env.UPSTREAM_DIR;
const require = createRequire(path.join(dir, 'package.json'));
const client = require('prom-client');
const jiti = loadUpstream(dir);
const { METRIC_PROVIDERS } = await jiti.import(`${dir}/src/scheduler/metrics-providers.ts`);

const metrics = {};
for (const provider of METRIC_PROVIDERS) {
    const metric = provider.useFactory({ customMetricPrefix: 'remnawave' });
    metrics[metric.name] = metric;
}
client.register.setDefaultLabels({ app: 'remnawave' });

const n1 = '11111111-1111-4111-8111-111111111111';
const n2 = '22222222-2222-4222-8222-222222222222';
const ops = [
    ['snapshot'],
    ['set', 'remnawave_users_status', { status: 'ACTIVE' }, 10],
    ['set', 'remnawave_users_status', { status: 'DISABLED' }, 0],
    ['set', 'remnawave_users_online_stats', { metricType: 'onlineNow' }, 3],
    ['set', 'remnawave_users_total', { type: 'all' }, 10],
    ['set', 'remnawave_node_basic_info', { node_uuid: n1, node_name: 'Node "one"\\path', node_country_emoji: '🇩🇪', provider_name: 'unknown', tags: 'A,B' }, 1],
    ['set', 'remnawave_node_status', { node_uuid: n1 }, 1],
    ['set', 'remnawave_node_status', { node_uuid: n2 }, 0],
    ['set', 'remnawave_node_cpu_load_avg_1m', { node_uuid: n1 }, 0.25],
    ['set', 'remnawave_node_uptime_seconds', { node_uuid: n1 }, 12345.67],
    ['set', 'remnawave_node_memory_total_bytes', { node_uuid: n1 }, 17179869184],
    ['inc', 'remnawave_node_inbound_upload_bytes', { node_uuid: n1, tag: 'vless' }, 100],
    ['inc', 'remnawave_node_inbound_upload_bytes', { node_uuid: n1, tag: 'vless' }, 50],
    ['inc', 'remnawave_node_outbound_download_bytes', { tag: 'direct', node_uuid: n2 }, 1e21],
    ['set', 'remnawave_process_rss_bytes', { instance_id: '0', instance_name: 'api' }, 52428800],
    ['set', 'remnawave_node_system_info', { node_uuid: n1, arch: 'x64', cpu_model: 'AMD\nEPYC', hostname: 'h', platform: 'linux', release: '6.1', version: '#1' }, 1],
    ['snapshot'],
    ['set', 'remnawave_node_status', { node_uuid: n1 }, 0],
    ['remove', 'remnawave_node_status', { node_uuid: n1 }],
    ['set', 'remnawave_node_status', { node_uuid: n1 }, 1],
    ['reset', 'remnawave_node_basic_info'],
    ['remove', 'remnawave_node_cpu_load_avg_1m', { node_uuid: 'missing' }],
    ['snapshot'],
];

const snapshots = [];
for (const [op, name, labels, value] of ops) {
    if (op === 'snapshot') snapshots.push(await client.register.metrics());
    else if (op === 'set') metrics[name].set(labels, value);
    else if (op === 'inc') metrics[name].inc(labels, value);
    else if (op === 'remove') metrics[name].remove(labels);
    else if (op === 'reset') metrics[name].reset();
}
console.log(JSON.stringify({ ops, snapshots, contentType: client.register.contentType }, null, 1));
