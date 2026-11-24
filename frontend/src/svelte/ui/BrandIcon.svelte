<script lang="ts">
    import { panelUrl } from '@shared/lib/panel-path'

    let { size = 24 }: { size?: number } = $props()
    const source = panelUrl(`favicons/gowave.svg?v=${__BUILD_ID__}`)
</script>

<img src={source} alt="" width={size} height={size}>
