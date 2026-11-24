type PlatformCount = {
    platform: string
    count: number
    byApp: { app: string; count: number }[]
}

const platformColors = [
    'teal-5',
    'blue-5',
    'cyan-5',
    'grape-5',
    'violet-5',
    'green-7',
    'yellow-6',
    'orange-6'
]

export function devicePlatforms(counts: PlatformCount[]) {
    return counts
        .filter(({ count }) => count > 0)
        .slice(0, platformColors.length)
        .map(({ platform, count, byApp }, index) => {
            const allApps = byApp.map(({ app, count }) => ({ name: app || 'Unknown', count }))
            return {
                name: platform || 'Unknown',
                count,
                color: `var(--mantine-color-${platformColors[index]})`,
                allApps,
                preview: allApps
                    .filter((app, index) => index === 0 || app.count >= count * 0.01)
                    .slice(0, 5)
            }
        })
}
