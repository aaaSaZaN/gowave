import { MantineProvider } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { requireValue } from '@shared/lib/invariant'
import { runAsync } from '@shared/lib/run-async'
import Highcharts from 'highcharts'
import { use, useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import { createRoot } from 'react-dom/client'

import { languageDetector, localeBackend } from '../src/app/i18n/plugins'
import { removeToken, setToken } from '../src/entities/auth'
import { logoutEvents } from '../src/shared/emitters'
import { AuthContext, AuthProvider } from '../src/shared/hocs/auth-provider/auth-provider'
import { ErrorBoundary } from '../src/shared/hocs/error-boundary/error-boundary-hoc'
import { useAutoAnimate } from '../src/shared/hooks/use-auto-animate'
import { useTime } from '../src/shared/hooks/use-time'
import { useVirtualList } from '../src/shared/hooks/use-virtual-list'
import isSvg from '../src/shared/lib/is-svg'
import { Chart } from '../src/shared/ui/chart'
import CountryFlag from '../src/shared/ui/country-flag'
import { CreateableTagInputShared } from '../src/shared/ui/createable-tag-input/createable-tag-input'
import Masonry from '../src/shared/ui/masonry'
import { NavigationProgress, nprogress } from '../src/shared/ui/navigation-progress'
import { SafeSvg } from '../src/shared/ui/safe-svg'
import { Sparkline } from '../src/shared/ui/sparkline'
import { RemarksManager } from '../src/widgets/dashboard/subscription-settings/settings/cards/managers/remarks-manager.widget'

function assert(condition: unknown, message: string): void {
    if (!condition) throw new Error(message)
}

const wait = (duration = 100) => new Promise<void>((resolve) => setTimeout(resolve, duration))
const container = document.createElement('div')
document.body.appendChild(container)
const root = createRoot(container)

function VirtualListFixture({ count }: { count: number }) {
    const scroll = useRef<HTMLDivElement>(null)
    const list = useVirtualList({
        count,
        estimateSize: () => 40,
        getScrollElement: () => scroll.current,
        getItemKey: (index) => `row-${index}`,
        overscan: 1
    })
    return (
        <div
            ref={scroll}
            data-total-size={list.totalSize}
            style={{ height: 100, overflow: 'auto' }}
        >
            <div style={{ height: list.totalSize, position: 'relative' }}>
                {list.virtualItems.map((item) => (
                    <div
                        key={item.key}
                        data-index={item.index}
                        ref={list.measureElement}
                        style={{
                            position: 'absolute',
                            width: '100%',
                            height: 40,
                            transform: `translateY(${item.start}px)`
                        }}
                    >
                        {item.key}
                    </div>
                ))}
            </div>
        </div>
    )
}

function WindowListFixture() {
    const list = useVirtualList({
        count: 100,
        estimateSize: () => 40,
        scrollMargin: 150,
        overscan: 1
    })
    return (
        <>
            <div style={{ height: 150 }} />
            <div style={{ height: list.totalSize, position: 'relative' }}>
                {list.virtualItems.map((item) => (
                    <div
                        key={item.key}
                        data-window-index={item.index}
                        style={{
                            position: 'absolute',
                            height: item.size,
                            transform: `translateY(${item.start - list.scrollMargin}px)`
                        }}
                    >
                        {item.key}
                    </div>
                ))}
            </div>
        </>
    )
}

function ClockFixture() {
    return <span>{useTime(20)}</span>
}
function AnimatedList() {
    const [items, setItems] = useState(['one', 'two'])
    const [parent] = useAutoAnimate({ duration: 30 })
    return (
        <>
            <button type="button" onClick={() => setItems(['two', 'three'])}>
                Update
            </button>
            <div ref={parent}>
                {items.map((item) => (
                    <div key={item}>{item}</div>
                ))}
            </div>
        </>
    )
}
function Fails({ failing }: { failing: boolean }): null {
    if (failing) throw new Error('Expected test failure')
    return null
}
function ErrorBoundaryFixture() {
    const [failing, setFailing] = useState(true)
    return (
        <ErrorBoundary
            FallbackComponent={({ error, resetErrorBoundary }) => (
                <button
                    type="button"
                    onClick={() => {
                        setFailing(false)
                        resetErrorBoundary()
                    }}
                >
                    {String(error)}
                </button>
            )}
        >
            <Fails failing={failing} />
        </ErrorBoundary>
    )
}

function AuthStatus() {
    const auth = requireValue(use(AuthContext), 'Auth context is missing')
    return <output>{`${auth.isInitialized}:${auth.isAuthenticated}`}</output>
}

async function run() {
    flushSync(() => root.render(<VirtualListFixture count={100} />))
    await wait()
    const viewport = container.firstElementChild as HTMLDivElement
    assert(
        container.querySelectorAll('[data-index]').length < 10,
        'virtual list bounds rendered rows'
    )
    assert(container.querySelector('[data-index="0"]'), 'virtual list renders first row')
    viewport.scrollTop = 800
    viewport.dispatchEvent(new Event('scroll'))
    await wait()
    assert(!container.querySelector('[data-index="0"]'), 'virtual list drops rows outside viewport')
    const visibleRow = requireValue(
        container.querySelector<HTMLElement>('[data-index]'),
        'Visible row is missing'
    )
    const beforeResize = Number(viewport.dataset['totalSize'])
    visibleRow.style.height = '120px'
    await wait()
    assert(
        Number(viewport.dataset['totalSize']) > beforeResize,
        'virtual list observes variable row heights'
    )
    flushSync(() => root.render(<VirtualListFixture count={2} />))
    await wait()
    assert(
        container.querySelectorAll('[data-index]').length === 2,
        'virtual list handles shrinking data'
    )
    flushSync(() => root.render(<WindowListFixture />))
    await wait()
    window.scrollTo(0, 1000)
    await wait()
    assert(
        !container.querySelector('[data-window-index="0"]'),
        'window virtualization tracks scroll and margin'
    )
    assert(
        container.querySelectorAll('[data-window-index]').length < 30,
        'window virtualization bounds rows'
    )
    window.scrollTo(0, 0)

    let lastRemarks: string[] = []
    const onRemarksChange = (remarks: string[]) => {
        lastRemarks = remarks
    }
    flushSync(() =>
        root.render(
            <MantineProvider>
                <RemarksManager
                    title="Remarks"
                    icon={<span />}
                    iconColor="teal"
                    initialRemarks={['First']}
                    onChange={onRemarksChange}
                />
            </MantineProvider>
        )
    )
    assert(container.querySelector('input')?.value === 'First', 'remarks initialize from props')
    const initialRemarkInput = requireValue(
        container.querySelector('input'),
        'Initial remark input missing'
    )
    const addRemark = requireValue(container.querySelector('button'), 'Add remark button missing')
    flushSync(() => addRemark.click())
    const secondRemarkInput = requireValue(
        container.querySelectorAll('input').item(1),
        'Second remark input missing'
    )
    secondRemarkInput.focus()
    const removeFirst = requireValue(
        container.querySelector<HTMLButtonElement>('button[aria-label="common.action.delete"]'),
        'Remove remark button missing'
    )
    flushSync(() => removeFirst.click())
    assert(
        container.querySelector('input') === secondRemarkInput &&
            document.activeElement === secondRemarkInput,
        'removing a remark preserves surviving input identity'
    )
    assert(!initialRemarkInput.isConnected, 'removed remark input detaches')
    flushSync(() =>
        root.render(
            <MantineProvider>
                <RemarksManager
                    title="Remarks"
                    icon={<span />}
                    iconColor="teal"
                    initialRemarks={['Second']}
                    onChange={onRemarksChange}
                />
            </MantineProvider>
        )
    )
    assert(container.querySelector('input')?.value === 'Second', 'remarks reset on new data')
    await wait(350)
    assert(lastRemarks[0] === 'Second', 'remarks report debounced changes')
    flushSync(() =>
        root.render(
            <MantineProvider>
                <RemarksManager
                    title="Remarks"
                    icon={<span />}
                    iconColor="teal"
                    onChange={onRemarksChange}
                />
            </MantineProvider>
        )
    )
    await wait()
    assert(
        container.querySelector('input')?.value === '',
        'omitted remarks have a stable empty default'
    )
    flushSync(() =>
        root.render(
            <MantineProvider>
                <CreateableTagInputShared tags={['ONE']} value="ONE" />
            </MantineProvider>
        )
    )
    assert(container.querySelector('input')?.value === 'ONE', 'tag input initializes from props')
    flushSync(() =>
        root.render(
            <MantineProvider>
                <CreateableTagInputShared tags={['TWO']} value="TWO" />
            </MantineProvider>
        )
    )
    assert(container.querySelector('input')?.value === 'TWO', 'tag input follows changed values')
    flushSync(() => root.render(<ClockFixture />))
    const firstTime = Number(container.textContent)
    await wait(60)
    assert(
        Number(container.textContent) > firstTime,
        'session clock advances without render-time reads'
    )

    assert(isSvg('<svg xmlns="http://www.w3.org/2000/svg"><path /></svg>'), 'valid SVG')
    assert(isSvg('<!-- icon --><svg />'), 'SVG with comments')
    assert(!isSvg('<svg><path></svg>'), 'reject malformed SVG')
    assert(!isSvg('<html><svg /></html>'), 'reject non-SVG root')

    const unsafeSvg =
        '<svg onload="window.svgExecuted=true"><script>window.svgExecuted=true</script><foreignObject><div>unsafe</div></foreignObject><path d="M0 0L1 1"/></svg>'
    flushSync(() => root.render(<SafeSvg svg={unsafeSvg} />))
    assert(container.querySelector('svg path'), 'SVG without xmlns retains SVG namespace')
    assert(!container.querySelector('script, foreignObject, [onload]'), 'SVG strips active content')
    flushSync(() => root.render(<SafeSvg svg="<div>unexpected HTML</div>" />))
    assert(container.firstElementChild?.children.length === 0, 'non-SVG roots are rejected')

    removeToken()
    flushSync(() =>
        root.render(
            <AuthProvider>
                <AuthStatus />
            </AuthProvider>
        )
    )
    await wait()
    assert(container.textContent === 'true:false', 'auth initializes without token')
    flushSync(() => setToken({ token: 'browser-token' }))
    assert(container.textContent === 'true:true', 'auth reacts to token changes')
    flushSync(() => logoutEvents.emit())
    assert(container.textContent === 'true:false', 'logout updates context and removes token')
    flushSync(() => logoutEvents.emit())
    assert(container.textContent === 'true:false', 'repeated logout remains safe')

    flushSync(() =>
        root.render(
            <MantineProvider>
                <Notifications />
            </MantineProvider>
        )
    )
    let unhandledRejections = 0
    const onUnhandledRejection = () => {
        unhandledRejections++
    }
    window.addEventListener('unhandledrejection', onUnhandledRejection)
    runAsync(Promise.reject(new Error('Expected async failure')))
    runAsync(Promise.resolve())
    runAsync(undefined)
    await wait()
    assert(unhandledRejections === 0, 'UI operations observe rejected promises')
    assert(
        document.body.textContent?.includes('Expected async failure'),
        'UI operations surface asynchronous errors'
    )
    window.removeEventListener('unhandledrejection', onUnhandledRejection)

    flushSync(() => root.render(<CountryFlag countryCode="us" />))
    assert(container.textContent === '🇺🇸', 'country flag Unicode')
    assert(container.querySelector('[aria-label="US"]'), 'country flag accessibility')

    flushSync(() => root.render(<NavigationProgress />))
    nprogress.start()
    await wait()
    assert(container.querySelector('[role="progressbar"]'), 'progress starts')
    nprogress.complete()
    await wait()
    assert(!container.querySelector('[role="progressbar"]'), 'progress completes')

    flushSync(() =>
        root.render(
            <Masonry columns={2} gap={16}>
                <div style={{ height: 100 }}>A</div>
                <div style={{ height: 160 }}>B</div>
                <div style={{ height: 70 }}>C</div>
            </Masonry>
        )
    )
    await wait()
    const items = requireValue(container.firstElementChild, 'Masonry root is missing').children
    assert(
        requireValue(items.item(0), 'Masonry item 0 is missing').getBoundingClientRect().height >=
            116,
        'masonry measures card height'
    )
    assert(
        requireValue(items.item(1), 'Masonry item 1 is missing').getBoundingClientRect().left >
            requireValue(items.item(0), 'Masonry item 0 is missing').getBoundingClientRect().left,
        'masonry columns'
    )
    assert(
        requireValue(items.item(2), 'Masonry item 2 is missing').getBoundingClientRect().top >=
            requireValue(items.item(0), 'Masonry item 0 is missing').getBoundingClientRect().bottom,
        'masonry avoids overlaps'
    )

    flushSync(() =>
        root.render(
            <Masonry columns={2} gap={16}>
                <div style={{ height: 100 }}>A</div>
                <div style={{ height: 160 }}>B</div>
                <div style={{ height: 70 }}>C</div>
                <div style={{ height: 230 }}>D</div>
            </Masonry>
        )
    )
    await wait()
    const addedMasonryItem = requireValue(
        container.firstElementChild?.children.item(3),
        'New masonry item is missing'
    )
    assert(
        addedMasonryItem.getBoundingClientRect().height >= 246,
        'masonry measures added children'
    )

    flushSync(() => root.render(<AnimatedList />))
    await wait()
    requireValue(container.querySelector('button'), 'Expected button').click()
    await wait(200)
    assert(container.textContent === 'Updatetwothree', 'animated list updates')
    assert(!document.querySelector('[aria-hidden="true"][inert]'), 'exit animation cleans up')

    const chartOptions: Highcharts.Options = {
        accessibility: { enabled: false },
        chart: { animation: false },
        series: [{ type: 'line', data: [1, 2, 3] }]
    }
    flushSync(() => root.render(<Chart title="First" options={chartOptions} />))
    await wait()
    assert(container.querySelector('.highcharts-root'), 'chart creates SVG')
    flushSync(() => root.render(<Chart title="Second" options={{ ...chartOptions, series: [] }} />))
    await wait()
    assert(container.querySelector('.highcharts-title')?.textContent === 'Second', 'chart updates')
    assert(Highcharts.charts.filter(Boolean).length === 1, 'chart reuses instance')

    flushSync(() =>
        root.render(
            <MantineProvider>
                <Sparkline
                    data={[1, 3, 2]}
                    h={100}
                    w="100%"
                    curveType="bump"
                    trendColors={{ positive: 'teal.6', negative: 'red.6', neutral: 'gray.5' }}
                />
            </MantineProvider>
        )
    )
    await wait()
    assert(Highcharts.charts.filter(Boolean).length === 0, 'chart cleans up')
    assert(container.querySelectorAll('path').length === 2, 'sparkline renders line and area')

    flushSync(() => root.render(<ErrorBoundaryFixture />))
    await wait()
    assert(container.textContent?.includes('Expected test failure'), 'error boundary catches')
    requireValue(container.querySelector('button'), 'Expected button').click()
    await wait()
    assert(!container.textContent, 'error boundary retries children')

    const originalFetch = window.fetch
    let request = ''
    window.fetch = (input) => {
        request = String(input)
        return Promise.resolve(
            new Response('{"hello":"world"}', {
                headers: { 'Content-Type': 'application/json' }
            })
        )
    }
    await new Promise<void>((resolve, reject) =>
        localeBackend.read('en', 'gowave', (error, data) => {
            if (error) return reject(error)
            assert(typeof data === 'object' && data?.['hello'] === 'world', 'locale JSON')
            resolve()
        })
    )
    assert(
        request.includes('/locales/en/gowave.json?v=browser-test'),
        'locale URL and cache busting'
    )
    window.fetch = async () => new Response('', { status: 404 })
    await new Promise<void>((resolve) =>
        localeBackend.read('missing', 'gowave', (error, retry) => {
            assert(error && retry === false, 'missing locale fails without retry')
            resolve()
        })
    )
    window.fetch = async () => new Response('', { status: 503 })
    await new Promise<void>((resolve) =>
        localeBackend.read('en', 'gowave', (error, retry) => {
            assert(error && retry === true, 'server failures allow retry')
            resolve()
        })
    )
    window.fetch = () => Promise.reject(new TypeError('Offline'))
    await new Promise<void>((resolve) =>
        localeBackend.read('en', 'gowave', (error, retry) => {
            assert(error && retry === true, 'network failures allow retry')
            resolve()
        })
    )
    window.fetch = async () => new Response('invalid json')
    await new Promise<void>((resolve) =>
        localeBackend.read('en', 'gowave', (error, retry) => {
            assert(error && retry === false, 'malformed translations fail without retry')
            resolve()
        })
    )
    window.fetch = () => Promise.resolve(new Response('42'))
    await new Promise<void>((resolve) =>
        localeBackend.read('en', 'gowave', (error, retry) => {
            assert(error && retry === false, 'scalar locale resource fails without retry')
            resolve()
        })
    )
    window.fetch = originalFetch
    languageDetector.cacheUserLanguage?.('ru-RU')
    assert(languageDetector.detect()?.[0] === 'ru', 'language cache and regional normalization')
    root.unmount()
}

run()
    .then(() => {
        document.body.dataset['result'] = 'passed'
        document.body.textContent = 'Browser smoke tests passed'
    })
    .catch((error: unknown) => {
        document.body.dataset['result'] = 'failed'
        document.body.textContent = String(error)
    })
