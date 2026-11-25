import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
    chooseRelease,
    compatibleReleases,
    getCompatibleAsset,
    type IReleaseInfo
} from './node-updates'

function release(tag: string, prerelease: boolean, ...files: string[]): IReleaseInfo {
    return {
        tag_name: tag,
        name: tag,
        prerelease,
        published_at: '2026-01-01T00:00:00Z',
        assets: files.map((name) => ({
            name,
            browser_download_url: `https://example.invalid/${name}`,
            size: 1
        }))
    }
}

test('picks actual binaries for the selected platform and architecture', () => {
    const assets = release(
        'v1',
        false,
        'gowave-node-linux-arm64',
        'gowave-node-linux-arm.tar.gz.sha256',
        'gowave-node-windows-arm.zip',
        'gowave-node-linux-arm.tar.gz'
    )
    assert.equal(
        getCompatibleAsset(assets, 'linux', 'arm', 'gowave-node')?.name,
        'gowave-node-linux-arm.tar.gz'
    )
    assert.equal(
        getCompatibleAsset(assets, 'linux', 'arm64', 'gowave-node')?.name,
        'gowave-node-linux-arm64'
    )
    assert.equal(getCompatibleAsset(assets, 'freebsd', 'arm', 'gowave-node'), undefined)
    const core = release(
        'v2',
        false,
        'Xray-linux-64.zip',
        'Xray-linux-x86_64.zip',
        'Xray-linux-32.zip'
    )
    assert.equal(getCompatibleAsset(core, 'linux', '386', 'rw-core')?.name, 'Xray-linux-32.zip')
    assert.equal(
        getCompatibleAsset(core, 'linux', 'amd64', 'rw-core')?.name,
        'Xray-linux-x86_64.zip'
    )
})

test('keeps incompatible versions out of stable, pre-release, and explicit selections', () => {
    const allReleases = [
        release('v3', false, 'gowave-node-linux-arm64'),
        release('v2-beta', true, 'gowave-node-linux-amd64'),
        release('v1', false, 'gowave-node-amd64')
    ]
    const compatible = compatibleReleases('node', {
        currentVersion: 'v0',
        currentCoreVersion: 'v0',
        architecture: 'amd64',
        os: 'linux',
        allReleases
    })
    assert.equal(chooseRelease(compatible, 'stable', '')?.tag_name, 'v1')
    assert.equal(chooseRelease(compatible, 'prerelease', '')?.tag_name, 'v2-beta')
    assert.equal(chooseRelease(compatible, 'version', 'v2-beta')?.tag_name, 'v2-beta')
    assert.equal(chooseRelease(compatible, 'version', 'v3'), undefined)
})
