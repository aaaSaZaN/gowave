#!/usr/bin/env node
// Dependency identifiers and attribution are allowed. Other retained identifiers
// are recorded by file and content digest so additions require explicit review.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { execFileSync } = require('node:child_process');
const root = path.resolve(__dirname, '..');
const allowlistPath = 'scripts/branding-allowlist.json';
const files = [...new Set(execFileSync('git', ['ls-files', '-co', '--exclude-standard', '-z'], { cwd: root }).toString().split('\0').filter(Boolean))].sort();
const exceptions = {};
for (const file of files) {
    if (file === allowlistPath || /(?:^|\/)(?:LICENSE|LICENCE)$/.test(file) || !fs.existsSync(path.join(root, file))) continue;
    const content = fs.readFileSync(path.join(root, file));
    if (content.includes(0)) continue;
    const lines = content.toString().split(/\r?\n/).filter(line => {
        const withoutDependencies = line
            .replace(/@remnawave\/[\w-]+/g, '')
            .replace(/\b(?:Get|Update)Remnawave\w*Command\b/g, '')
            .replace(/\bREMNAWAVE_(?:BYPASS_HTTPS_RESTRCTIONS|CLIENT_TYPE_BROWSER|CLIENT_TYPE_HEADER)\b/g, '');
        return /remna/i.test(withoutDependencies);
    }).map(line => line.trim());
    if (lines.length || /remna/i.test(file)) {
        exceptions[file] = {
            count: lines.length,
            sha256: crypto.createHash('sha256').update(lines.sort().join('\n')).digest('hex')
        };
        if (process.argv.includes('--inspect')) console.log(file + '\n' + lines.join('\n') + '\n');
    }
}
if (process.argv.includes('--inspect')) process.exit(0);
const allowed = JSON.parse(fs.readFileSync(path.join(root, allowlistPath), 'utf8')).files;
let failed = false;
for (const file of new Set([...Object.keys(exceptions), ...Object.keys(allowed)])) {
    const actual = exceptions[file];
    const expected = allowed[file];
    if (!actual || !expected || actual.count !== expected.count || actual.sha256 !== expected.sha256) {
        console.error(`${file}: unreviewed legacy references or stale exception; inspect with --inspect`);
        failed = true;
    }
}
if (failed) process.exit(1);
console.log('Branding references match reviewed dependency, attribution, and compatibility exceptions.');
