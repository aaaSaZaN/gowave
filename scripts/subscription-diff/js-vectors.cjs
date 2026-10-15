// Regenerates backend-go/internal/subtpl/js/testdata/vectors.json (run with TZ=UTC):
//   TZ=UTC node scripts/subscription-diff/js-vectors.cjs $UPSTREAM_DIR > backend-go/internal/subtpl/js/testdata/vectors.json
const U = process.argv[2];
const dayjs = require(U + '/node_modules/dayjs');
const xbytes = require(U + '/node_modules/xbytes');
const date = new Date('2026-03-04T15:06:07.089Z');
const formats = ['DD.MM.YYYY', 'YYYY-MM-DD HH:mm:ss', 'D/M/YY h:mm A a', 'MMM MMMM ddd dddd dd d', 'SSS Z ZZ', '[Day] DD [at] HH', 'X x Y YYY', 'Привет MM', ''];
const uri = ['plain', 'a b&c=d/é', "-_.!~*'()", '😀 #?', 'Россия 🇷🇺'];
const b64 = ['YWJj', 'YWJj===', 'YW Jj', 'YWJjZA', '-_8', 'YWJj$ZGVm', 'Zm9v.YmFy'];
const bytes = [1, 1023, 1024, 1536, 1073741824, 5e12, 123456789, -2048];
console.log(JSON.stringify({
    dayjs: formats.map((f) => ({ format: f, out: dayjs(date).format(f) })),
    uri: uri.map((s) => ({ in: s, out: encodeURIComponent(s) })),
    base64: b64.map((s) => ({ in: s, out: Buffer.from(s, 'base64').toString('hex') })),
    bytes: bytes.map((b) => ({ in: b, out: String(xbytes.parseBytes(b, { iec: true, sticky: false, prefixIndex: undefined }).size), sticky: String(xbytes.parseBytes(b, { iec: true, sticky: true, prefixIndex: 3 }).size) })),
}, null, 1));
