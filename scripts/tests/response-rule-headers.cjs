// Shared frontend/API schema regressions. Run with Bun from the repository root.
const assert = require('node:assert/strict');
const {
    ResponseRulesConfigSchema,
    UpdateSubscriptionSettingsBodySchema,
} = require('../../frontend/src/shared/api/schemas/subscription-settings.schema.ts');

const config = (headers) => ({
    version: '1',
    rules: [{
        name: 'headers',
        enabled: true,
        operator: 'AND',
        conditions: [],
        responseType: 'XRAY_BASE64',
        responseModifications: { headers },
    }],
});
const parsedHeaders = (input) =>
    ResponseRulesConfigSchema.parse(config(input)).rules[0].responseModifications.headers;

assert.deepEqual(parsedHeaders({ 'X-Test': 'value' }), { 'X-Test': 'value' });
assert.deepEqual(parsedHeaders([{ key: 'X-Test', value: 'value' }]), { 'X-Test': 'value' });
assert.deepEqual(parsedHeaders([
    { key: 'X-Test', value: 'first' },
    { key: 'x-test', value: 'second' },
    { key: 'X-Test', value: 'last' },
]), { 'X-Test': 'last' });
assert.deepEqual(parsedHeaders([]), {});
assert.deepEqual(parsedHeaders({}), {});
assert.equal(Object.getPrototypeOf(parsedHeaders(JSON.parse('{"__proto__":"value"}'))), Object.prototype);

for (const headers of [{ 'bad header': 'value' }, { 'X-Test': 1 }, [{ key: 'X-Test' }], 'invalid']) {
    assert.equal(ResponseRulesConfigSchema.safeParse(config(headers)).success, false);
}

const saved = UpdateSubscriptionSettingsBodySchema.parse({
    uuid: '11111111-1111-4111-8111-111111111111',
    responseRules: config([{ key: 'X-Test', value: 'value' }]),
});
assert.deepEqual(saved.responseRules.rules[0].responseModifications.headers, { 'X-Test': 'value' });

const schema = ResponseRulesConfigSchema.toJSONSchema({ target: 'draft-07', io: 'input' });
const rule = schema.properties.rules.items;
const headers = rule.properties.responseModifications.properties.headers;
assert.equal(headers.anyOf[0].type, 'object');
assert.equal(headers.anyOf[1].type, 'array');
assert.equal(headers.anyOf[1].deprecated, true);
assert.deepEqual(headers.defaultSnippets[0].body, { 'X-Custom-Header': 'CustomValue' });
assert.deepEqual(rule.defaultSnippets[0].body.responseModifications.headers, {});
process.stdout.write('Response Rules header schema checks passed\n');
