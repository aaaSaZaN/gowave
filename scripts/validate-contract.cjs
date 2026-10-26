#!/usr/bin/env bun
// Validates responses captured by the Go probe test against the pinned
// @remnawave/backend-contract zod schemas.
//   GOWAVE_PROBE_OUT=/tmp/probe.json go test ./internal/server -run Probe
//   bun scripts/validate-contract.cjs /tmp/probe.json
const path = require('path');
const fs = require('fs');
const contract = require(path.resolve(__dirname, '../frontend/node_modules/@remnawave/backend-contract'));
const {
    SubscriptionSettingsResponseSchema,
    TestSrrMatcherResponseSchema,
} = require('../frontend/src/shared/api/schemas/subscription-settings.schema.ts');

// GoWave's documented extension makes Response Rules headers objects. Keep all
// other fields checked against the pinned contract through the shared schemas.
const responseSchemas = {
    GetSubscriptionSettingsCommand: SubscriptionSettingsResponseSchema,
    UpdateSubscriptionSettingsCommand: SubscriptionSettingsResponseSchema,
    TestSrrMatcherCommand: TestSrrMatcherResponseSchema,
};

const results = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
let failures = 0;
for (const result of results) {
    const command = contract[result.name];
    const label = `${result.method} ${result.path} (${result.name})`;
    if (result.expectedStatus && result.status === result.expectedStatus) continue;
    if (result.status >= 400) {
        console.log(`STATUS ${result.status} ${label}: ${JSON.stringify(result.body).slice(0, 200)}`);
        failures++;
        continue;
    }
    if (!command || !command.ResponseSchema) continue;
    const parsed = (responseSchemas[result.name] ?? command.ResponseSchema).safeParse(result.body);
    if (!parsed.success) {
        failures++;
        const issues = parsed.error.issues
            .slice(0, 5)
            .map((issue) => `    ${issue.path.join('.')}: ${issue.message}`)
            .join('\n');
        console.log(`SCHEMA ${label}\n${issues}`);
    }
}
console.log(`${results.length - failures}/${results.length} responses match the contract`);
process.exit(failures ? 1 : 0);
