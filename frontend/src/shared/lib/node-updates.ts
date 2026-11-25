export interface IReleaseAsset {
    name: string
    browser_download_url: string
    size: number
}

export interface IReleaseInfo {
    tag_name: string
    name: string
    prerelease: boolean
    published_at: string
    assets: IReleaseAsset[]
}

export interface IUpdatesCheckResponse {
    currentVersion: string
    latestRelease?: IReleaseInfo
    latestPrerelease?: IReleaseInfo
    allReleases: IReleaseInfo[]
    architecture: string
    os: string
    currentCoreVersion: string
    error?: string
}

export type ReleaseChannel = 'stable' | 'prerelease' | 'version'

export const NODE_REPOSITORY = 'aaaSaZaN/gowave-node'
export const XRAY_REPOSITORY = 'XTLS/Xray-core'

const archAliases: Record<string, string[]> = {
    amd64: ['amd64', 'x86_64', 'linux-64', '64bit'],
    arm64: ['arm64-v8a', 'arm64v8', 'arm64', 'aarch64'],
    arm: ['arm32-v7a', 'armv7', 'arm32-v6', 'armv6', 'armhf', 'arm'],
    mipsle: ['mips32le', 'mipsle', 'mipsel'],
    mips: ['mips32', 'mips'],
    mips64le: ['mips64le'],
    mips64: ['mips64'],
    '386': ['linux-32', '386', 'i386', '32bit', 'x86']
}

const osAliases = [
    ['linux'],
    ['windows', 'win'],
    ['darwin', 'macos', 'osx', 'mac'],
    ['freebsd'],
    ['openbsd'],
    ['netbsd'],
    ['android']
]

function assetToken(name: string, token: string): boolean {
    const normalize = (value: string) => value.toLowerCase().replace(/[^a-z0-9]+/g, '-')
    return `-${normalize(name)}-`.includes(`-${normalize(token)}-`)
}

export function getCompatibleAsset(
    release: IReleaseInfo,
    os: string,
    architecture: string,
    binaryName: 'gowave-node' | 'rw-core'
) {
    const aliases = archAliases[architecture] || [architecture]
    const assets = release.assets || []
    if (!architecture) return undefined
    const platforms = osAliases.find((options) => options.includes(os.toLowerCase())) ?? [os]
    const matches = (asset: IReleaseAsset, alias: string, requireOS: boolean) => {
        const name = asset.name.toLowerCase()
        if (
            ['.dgst', '.sha256', '.sha512', '.sha1', '.sig', '.asc', '.txt', '.json'].some(
                (suffix) => name.endsWith(suffix)
            )
        ) {
            return false
        }
        const matchingOS = platforms.some((platform) => assetToken(name, platform))
        if (
            !matchingOS &&
            (requireOS ||
                osAliases.some((options) => options.some((option) => assetToken(name, option))))
        ) {
            return false
        }
        if (
            (architecture === 'mips' || architecture === 'mips64') &&
            /mips(?:32|64)?[-_]?(?:le|el)/.test(name)
        ) {
            return false
        }
        if (
            architecture === '386' &&
            archAliases['amd64']?.some((value) => assetToken(name, value))
        ) {
            return false
        }
        return assetToken(name, alias) && (binaryName === 'rw-core' || name.includes(binaryName))
    }

    for (const alias of aliases) {
        const asset = assets.find((candidate) => matches(candidate, alias, true))
        if (asset) return asset
    }
    for (const alias of aliases) {
        const asset = assets.find((candidate) => matches(candidate, alias, false))
        if (asset) return asset
    }
    return undefined
}

export type UpdateComponent = 'node' | 'core'

export function updateRepository(component: UpdateComponent): string {
    return component === 'node' ? NODE_REPOSITORY : XRAY_REPOSITORY
}

export function compatibleReleases(component: UpdateComponent, info: IUpdatesCheckResponse) {
    if (!info.architecture) return []
    const binary = component === 'node' ? 'gowave-node' : 'rw-core'
    return info.allReleases.filter((release) =>
        getCompatibleAsset(release, info.os || 'linux', info.architecture, binary)
    )
}

export function chooseRelease(releases: IReleaseInfo[], channel: ReleaseChannel, version: string) {
    return releases.find((release) =>
        channel === 'version'
            ? release.tag_name === version
            : release.prerelease === (channel === 'prerelease')
    )
}
