import { panelUrl } from '@shared/lib/panel-path'
export const app = {
    name: 'GoWave',
    githubRepo: 'https://github.com/aaaSaZaN/gowave',
    githubStars: 'https://github.com/aaaSaZaN/gowave/stargazers',
    githubIssues: 'https://github.com/aaaSaZaN/gowave/issues',
    githubOrg: 'https://github.com/aaaSaZaN',
    configEditor: {
        wasmUrl: panelUrl('/assets/main.wasm'),
        wasmJsUrl: panelUrl('/assets/wasm_exec.js'),
        jsonSchemaUrl: panelUrl('/assets/xray.schema.json'),
        jsonSchemaCnUrl: panelUrl('/assets/xray.schema.cn.json')
    },
    templateEditor: {
        singboxJsonSchemaUrl: panelUrl('/assets/singbox.schema.json'),
        mihomoYamlSchemaUrl: panelUrl('/assets/mihomo.schema.json')
    }
}
