// Dumps tricky scalars with upstream's js-yaml so the Go port of dump() can
// be checked byte for byte:
//   UPSTREAM_DIR=... $NODE24 scripts/subscription-diff/yaml-vectors.mjs > backend-go/internal/subtpl/testdata/yaml-vectors.json
import { loadUpstream } from './load-upstream.mjs';

const jiti = loadUpstream(process.env.UPSTREAM_DIR);
const { dump } = await jiti.import('js-yaml');

const strings = [
    '', ' ', '  ', '\t', 'plain', 'with space', ' leading', 'trailing ', 'a\tb',
    'yes', 'Yes', 'YES', 'no', 'No', 'on', 'Off', 'y', 'n', 'true', 'False', 'null', 'Null', '~', '=', '<<',
    '0', '00', '08', '0o17', '0x1F', '0x_', '0b101', '0b_', '1_000', '-1', '+1', '1:30', '190:20:30', '1.5', '.5', '1.', '.', '-.inf', '.NaN', '1e10', '1e+10', '1.0e+10', '6.8523015e+5',
    '2024-01-01', '2024-13-01', '2024-02-30', '2001-12-14t21:59:43.10-05:00', '2001-12-14 21:59:43.10', '2024-1-1 1:02:03', '12:30',
    '-', '- a', '-a', '?', '? a', '?a', ':', ':a', 'a:', 'a: b', 'a:b', 'http://x.example/a?b=c#d', '#comment', 'a #b', 'a#b', 'a #',
    '[x]', '{x}', 'a,b', 'a, b', '&anchor', '*alias', '!tag', '|', '>', "'q'", '"q"', "it's", 'say "hi"', '%x', '@x', '`x`',
    '---', '--- a', '...', 'a\nb', 'a\n\nb', 'a\n', 'a\n\n', '\n', '\na', ' a\nb', 'a \nb', 'a\n b', 'line\nwith trailing\n',
    'x'.repeat(100), ('word '.repeat(30)).trim(), ' ' + 'word '.repeat(30), 'word '.repeat(30) + '\nsecond line', 'a'.repeat(90) + ' b',
    'tab\there', '\u0007bell', '\u001bEsc', '\u0085nel', ' nbsp', ' ls', '﻿bom', 'del\u007f', 'кириллица', 'emoji 🇩🇪 flag',
    '→ Remnawave', 'MATCH,→ Remnawave', 'DOMAIN-SUFFIX,example.com,DIRECT', '1.1.1.1', '127.0.0.1:9090', '198.18.0.1/16', '*.lan', '+.xboxlive.com',
    'stun.*.*', 'h2', 'http/1.1', '/path?ed=2048', 'chacha20-ietf-poly1305', '2022-blake3-aes-256-gcm', 'c2VydmVyLXBhc3N3b3Jk==', '100-1000', '16-32',
    '600', '-0', '+0', '1_', '_1', '0.0.0.0', '::1', '[::1]:443', 'a: ', 'key:', ' : ', 'x\r\ny', 'é', 'Ünïcödé', '"', "'",
    'very long single-quoted-needed: ' + 'word '.repeat(20),
];

const numbers = [0, 1, -1, 1.5, 0.1, 1e21, 1e-7, 123456789012, 2 ** 53, 1.5e300, -2.5e-10];

const nested = {};
strings.forEach((s, i) => {
    nested[`k${i}`] = s;
});
const vectors = [];
const push = (input) => vectors.push({ input, output: dump(input) });

for (const s of strings) push({ [s === '' ? 'empty' : s]: s });
push(nested);
push({ deep: { deeper: { deepest: strings } } });
push({ list: strings.map((s) => ({ name: s, value: [s, s] })) });
push({ numbers, bools: [true, false], nil: null, empty: [], obj: {} });
push(strings);
push({ 'multi\nline key': 1, ['k'.repeat(1030)]: 2 });
push({ a: [[1, 2], [], [{}], [[3]]], b: [{ c: [{ d: 'e' }] }] });

console.log(JSON.stringify(vectors, null, 1));
