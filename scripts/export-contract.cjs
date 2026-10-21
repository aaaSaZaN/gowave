#!/usr/bin/env node
// Exports every command from the pinned @remnawave/backend-contract as
// {name, method, path, scope, kind, description} so Go tests can verify parity
// without Node.js at runtime. Run from the repository root:
//   node scripts/export-contract.cjs > backend-go/internal/contracts/commands.json
// With --openapi it prints an OpenAPI 3.1 document built from the zod schemas:
//   node scripts/export-contract.cjs --openapi > backend-go/internal/contracts/openapi.json
const path = require('path');

(function main() {
const contract = require(path.resolve(__dirname, '../frontend/node_modules/@remnawave/backend-contract'));
const pkg = require(path.resolve(__dirname, '../frontend/node_modules/@remnawave/backend-contract/package.json'));

const commands = [];
for (const [name, value] of Object.entries(contract)) {
    if (!name.endsWith('Command') || !value || typeof value !== 'object' || !value.endpointDetails) continue;
    const details = value.endpointDetails;
    commands.push({
        name,
        method: String(details.REQUEST_METHOD).toUpperCase(),
        path: value.TSQ_url.replace(/\/+$/, '') || '/',
        scope: details.SCOPE || '',
        kind: details.SCOPE_KIND || '',
        description: details.METHOD_DESCRIPTION || '',
        hasBody: Boolean(value.RequestBodySchema),
        hasQuery: Boolean(value.RequestQuerySchema),
    });
}
commands.sort((a, b) => (a.path + a.method).localeCompare(b.path + b.method));

if (!process.argv.includes('--openapi')) {
    process.stdout.write(JSON.stringify({ version: pkg.version, commands }, null, 2) + '\n');
    return;
}

const { z } = require(path.resolve(__dirname, '../frontend/node_modules/zod'));
const toSchema = (schema, io) => {
    try {
        const json = z.toJSONSchema(schema, { unrepresentable: 'any', io });
        delete json.$schema;
        return json;
    } catch {
        return {};
    }
};
const paths = {};
for (const command of commands) {
    const value = contract[command.name];
    const openapiPath = command.path.replace(/:(\w+)/g, '{$1}');
    const parameters = [...command.path.matchAll(/:(\w+)/g)].map(([, name]) => ({
        name,
        in: 'path',
        required: true,
        schema: { type: 'string' },
    }));
    if (value.RequestQuerySchema) {
        const query = toSchema(value.RequestQuerySchema, 'input');
        for (const [name, schema] of Object.entries(query.properties || {})) {
            parameters.push({ name, in: 'query', required: (query.required || []).includes(name), schema });
        }
    }
    const operation = {
        operationId: command.name.replace(/Command$/, ''),
        summary: command.description,
        tags: [command.path.split('/')[2]],
        security: [{ bearerAuth: [] }],
        parameters,
        responses: {},
    };
    if (value.endpointDetails.METHOD_LONG_DESCRIPTION) {
        operation.description = value.endpointDetails.METHOD_LONG_DESCRIPTION;
    }
    if (value.RequestBodySchema) {
        operation.requestBody = {
            required: true,
            content: { 'application/json': { schema: toSchema(value.RequestBodySchema, 'input') } },
        };
    }
    operation.responses.default = value.ResponseSchema
        ? { description: 'Success', content: { 'application/json': { schema: toSchema(value.ResponseSchema, 'output') } } }
        : { description: 'Success' };
    paths[openapiPath] = paths[openapiPath] || {};
    paths[openapiPath][command.method.toLowerCase()] = operation;
}
process.stdout.write(
    JSON.stringify({
        openapi: '3.1.0',
        info: { title: 'GoWave API', version: pkg.version },
        paths,
        components: { securitySchemes: { bearerAuth: { type: 'http', scheme: 'bearer', bearerFormat: 'JWT' } } },
    }) + '\n',
);
})();
