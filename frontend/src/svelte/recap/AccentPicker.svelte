<script lang="ts">
    import { untrack } from 'svelte'
    import Icon from '../ui/Icon.svelte'

    let {
        swatches,
        disabled,
        onchange
    }: {
        swatches: string[]
        disabled: boolean
        onchange: (hex: string) => void
    } = $props()

    // Keep hue even at zero saturation so dragging back from gray restores the selected hue.
    let hue = $state(0)
    let saturation = $state(0)
    let brightness = $state(0)
    untrack(() => setFromHex(swatches[0] ?? '#15aabf'))
    let color = $derived(hexValue())

    function hexValue(): string {
        const channel = (offset: number) => {
            const k = (offset + hue / 60) % 6
            return Math.round(
                255 * brightness * (1 - saturation * Math.max(0, Math.min(k, 4 - k, 1)))
            )
                .toString(16)
                .padStart(2, '0')
        }
        return `#${channel(5)}${channel(3)}${channel(1)}`
    }

    function setFromHex(hex: string): void {
        const rgb = Number.parseInt(hex.slice(1), 16)
        const r = (rgb >> 16) / 255
        const g = ((rgb >> 8) & 255) / 255
        const b = (rgb & 255) / 255
        brightness = Math.max(r, g, b)
        const delta = brightness - Math.min(r, g, b)
        saturation = brightness ? delta / brightness : 0
        if (delta) {
            const sector =
                brightness === r
                    ? (g - b) / delta
                    : brightness === g
                      ? (b - r) / delta + 2
                      : (r - g) / delta + 4
            hue = (sector * 60 + 360) % 360
        }
    }

    function clamp(value: number): number {
        return Math.max(0, Math.min(1, value))
    }

    function position(event: PointerEvent & { currentTarget: HTMLButtonElement }): void {
        const bounds = event.currentTarget.getBoundingClientRect()
        saturation = clamp((event.clientX - bounds.left) / bounds.width)
        brightness = clamp(1 - (event.clientY - bounds.top) / bounds.height)
        onchange(hexValue())
    }
</script>

<div class="picker">
    <button
        class="plane"
        type="button"
        aria-label="Accent saturation and brightness"
        {disabled}
        style:--hue={hue}
        onpointerdown={(event) => {
            event.currentTarget.setPointerCapture(event.pointerId)
            position(event)
        }}
        onpointermove={(event) => {
            if (event.currentTarget.hasPointerCapture(event.pointerId)) position(event)
        }}
        onkeydown={(event) => {
            if (event.key === 'ArrowLeft') saturation = clamp(saturation - 0.01)
            else if (event.key === 'ArrowRight') saturation = clamp(saturation + 0.01)
            else if (event.key === 'ArrowDown') brightness = clamp(brightness - 0.01)
            else if (event.key === 'ArrowUp') brightness = clamp(brightness + 0.01)
            else return
            event.preventDefault()
            onchange(hexValue())
        }}
    >
        <span
            style:left={`${saturation * 100}%`}
            style:top={`${(1 - brightness) * 100}%`}
            style:background={color}
        ></span>
    </button>
    <input
        aria-label="Accent hue"
        class="hue"
        type="range"
        min="0"
        max="360"
        step="0.1"
        value={hue}
        {disabled}
        style:--hue={hue}
        oninput={(event) => {
            hue = event.currentTarget.valueAsNumber
            onchange(hexValue())
        }}
    >
    <div class="swatches">
        {#each swatches as swatch (swatch)}
            <button
                type="button"
                {disabled}
                aria-label={swatch}
                aria-pressed={swatch === color}
                style:background={swatch}
                onclick={() => {
                    setFromHex(swatch)
                    onchange(swatch)
                }}
            >
                {#if swatch === color}
                    <Icon name="TbCheck" size={14} />
                {/if}
            </button>
        {/each}
    </div>
</div>

<style>
    .picker {
        display: grid;
        gap: 6px;
    }
    .plane {
        position: relative;
        height: 140px;
        touch-action: none;
        border: 0;
        overflow: hidden;
        cursor: crosshair;
        border-radius: 4px;
        background:
            linear-gradient(to top, black, transparent),
            linear-gradient(to right, white, hsl(var(--hue) 100% 50%));
    }
    .plane span,
    .hue::-webkit-slider-thumb {
        width: 14px;
        height: 14px;
        border: 2px solid white;
        border-radius: 50%;
        box-shadow: 0 0 2px #777;
    }
    .plane span {
        position: absolute;
        transform: translate(-50%, -50%);
        pointer-events: none;
    }
    .hue {
        appearance: none;
        border: 0;
        width: 100%;
        height: 16px;
        min-height: 0;
        padding: 0;
        border-radius: 1rem;
        background: linear-gradient(to right, red, yellow, lime, cyan, blue, magenta, red);
    }
    .hue::-webkit-slider-thumb {
        appearance: none;
        background: hsl(var(--hue) 100% 50%);
    }
    .swatches {
        display: grid;
        grid-template-columns: repeat(8, minmax(0, 1fr));
        gap: 4px;
    }
    .swatches button {
        display: grid;
        place-items: center;
        min-height: 0;
        aspect-ratio: 1;
        padding: 0;
        border: 0;
        border-radius: 4px;
        color: white;
    }
</style>
