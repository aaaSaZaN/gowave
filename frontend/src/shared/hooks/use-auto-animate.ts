import { useEffect, useEffectEvent, useState } from 'react'

interface Options {
    duration?: number
    easing?: string
    disrespectUserMotionPreference?: boolean
}

type AnimationFactory = (element: Element, action: 'add' | 'remove' | 'remain') => KeyframeEffect

export function useAutoAnimate<T extends HTMLElement = HTMLDivElement>(
    options: Options | AnimationFactory = {}
) {
    const [element, setElement] = useState<T | null>(null)
    const duration = typeof options === 'function' ? 250 : (options.duration ?? 250)
    const easing = typeof options === 'function' ? 'ease-in-out' : (options.easing ?? 'ease-in-out')
    const ignorePreference = typeof options !== 'function' && options.disrespectUserMotionPreference
    const createEffect = useEffectEvent(
        (target: Element, action: 'add' | 'remove' | 'remain', frames: Keyframe[]) =>
            typeof options === 'function'
                ? options(target, action)
                : new KeyframeEffect(target, frames, { duration, easing })
    )

    useEffect(() => {
        if (!element?.animate) return
        const preference = window.matchMedia('(prefers-reduced-motion: reduce)')
        let rectangles = new Map<Element, { rect: DOMRect; font: string; color: string }>()
        const animations = new Set<Animation>()
        const ghosts = new Set<HTMLElement>()
        const snapshot = () => {
            rectangles = new Map(
                Array.from(element.children, (child) => {
                    const style = getComputedStyle(child)
                    return [
                        child,
                        {
                            rect: child.getBoundingClientRect(),
                            font: style.font,
                            color: style.color
                        }
                    ]
                })
            )
        }
        const play = (effect: KeyframeEffect, finish?: () => void) => {
            const animation = new Animation(effect, document.timeline)
            animations.add(animation)
            animation.onfinish = () => {
                animations.delete(animation)
                finish?.()
            }
            animation.play()
        }
        snapshot()
        const observer = new MutationObserver(() => {
            if (preference.matches && !ignorePreference) {
                snapshot()
                return
            }
            for (const child of element.children) {
                const previous = rectangles.get(child)?.rect
                const current = child.getBoundingClientRect()
                const frames = previous
                    ? [
                          {
                              transform: `translate(${previous.left - current.left}px, ${previous.top - current.top}px)`
                          },
                          { transform: 'translate(0, 0)' }
                      ]
                    : [
                          { opacity: 0, transform: 'scale(.98)' },
                          { opacity: 1, transform: 'scale(1)' }
                      ]
                play(createEffect(child, previous ? 'remain' : 'add', frames))
            }
            for (const [child, { rect, font, color }] of rectangles) {
                if (child.parentElement === element || !(child instanceof HTMLElement)) continue
                const ghost = child.cloneNode(true) as HTMLElement
                ghost.removeAttribute('id')
                ghost.inert = true
                ghost.setAttribute('aria-hidden', 'true')
                Object.assign(ghost.style, {
                    position: 'fixed',
                    left: `${rect.left}px`,
                    top: `${rect.top}px`,
                    width: `${rect.width}px`,
                    height: `${rect.height}px`,
                    margin: '0',
                    pointerEvents: 'none',
                    zIndex: '1000',
                    font,
                    color
                })
                ghosts.add(ghost)
                document.body.appendChild(ghost)
                play(createEffect(ghost, 'remove', [{ opacity: 1 }, { opacity: 0 }]), () => {
                    ghost.remove()
                    ghosts.delete(ghost)
                })
            }
            snapshot()
        })
        observer.observe(element, { childList: true })
        const resize = new ResizeObserver(snapshot)
        resize.observe(element)
        return () => {
            observer.disconnect()
            resize.disconnect()
            for (const animation of animations) animation.cancel()
            for (const ghost of ghosts) ghost.remove()
        }
    }, [element, ignorePreference])

    return [setElement] as const
}
