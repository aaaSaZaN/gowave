import { readFileSync } from 'node:fs'
import { join } from 'node:path'

const panel = '/dashboard/'
const token = 'browser-smoke-token'
const squadUuid = '11111111-1111-4111-8111-111111111111'
const externalUuid = '51515151-5151-4151-8151-515151515151'
const base64Uuid = '52525252-5252-4252-8252-525252525252'
const mihomoUuid = '53535353-5353-4353-8353-535353535353'
const hostUuid = '54545454-5454-4454-8454-545454545454'
const result = (response) => Response.json({ response })

const templateValidationCases = ['MIHOMO', 'STASH', 'CLASH', 'SINGBOX', 'STASH'].map(
    (type, index) => ({
        type,
        uuid: `53535353-5353-4353-8353-53535353535${index + 4}`,
        syntaxError: index === 4
    })
)

// Runs inside Chromium after the app entry module.
async function verify({ externalUuid, mihomoUuid, squadUuid, hostUuid, templateValidationCases }) {
    performance.setResourceTimingBufferSize(1500)

    async function until(read, message) {
        for (let attempt = 0; attempt < 150; attempt++) {
            const value = read()
            if (value) return value
            await new Promise((resolve) => setTimeout(resolve, 50))
        }
        throw new Error(message)
    }

    function expectDialogExit(dialog, mode = 'fade') {
        if (matchMedia('(prefers-reduced-motion: reduce)').matches) {
            if (dialog.open) throw new Error('A dialog animated with reduced motion enabled')
            return
        }
        const exiting = dialog.getAnimations().some((animation) => {
            const finalFrame = animation.effect?.getKeyframes().at(-1)
            return mode === 'slide'
                ? finalFrame?.transform?.includes('100%')
                : Number(finalFrame?.opacity) === 0
        })
        if (!dialog.open || !exiting) throw new Error(`The dialog did not animate its ${mode} exit`)
    }

    const select = (selector) => document.querySelector(selector)
    const setValue = (field, value) => {
        field.value = value
        field.dispatchEvent(new Event('input', { bubbles: true }))
    }
    const findButton = (text, parent = document) =>
        [...parent.querySelectorAll('button')].find(
            (button) =>
                (button.getAttribute('aria-label') ??
                    button.textContent.trim().replace(/\s+/g, ' ')) === text
        )
    const clickVisibleCenter = (element) => {
        const rect = element.getBoundingClientRect()
        document
            .elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2)
            ?.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }))
    }

    function dragBetween(handle, source, target) {
        const grip = handle.getBoundingClientRect()
        const from = source.getBoundingClientRect()
        const to = target.getBoundingClientRect()
        const x = grip.left + grip.width / 2
        const y = grip.top + grip.height / 2
        const destinationX = x + to.left + to.width / 2 - from.left - from.width / 2
        const destinationY = y + to.top + to.height / 2 - from.top - from.height / 2
        handle.setPointerCapture = () => {}
        const pointer = (type, clientX, clientY) =>
            handle.dispatchEvent(
                new PointerEvent(type, {
                    bubbles: true,
                    isPrimary: true,
                    pointerId: 7,
                    clientX,
                    clientY
                })
            )
        pointer('pointerdown', x, y)
        pointer('pointermove', destinationX, destinationY)
        return () => pointer('pointerup', destinationX, destinationY)
    }

    try {
        await until(
            () => select('main h1')?.textContent === 'Quick open',
            'Svelte dashboard failed to mount'
        )
        const description = select('.page-description')
        const writeText = navigator.clipboard.writeText
        let copiedText
        navigator.clipboard.writeText = (value) => {
            copiedText = value
            return Promise.resolve()
        }
        try {
            description.click()
            const notice = await until(
                () => select('.copy-notice [role=status]'),
                'Copying the page description did not show confirmation'
            )
            if (copiedText !== description.textContent.trim()) {
                throw new Error('Copying the page description wrote a different value')
            }
            findButton('Close', notice.parentElement).click()
            await until(() => !select('.copy-notice'), 'The copy confirmation did not close')
        } finally {
            navigator.clipboard.writeText = writeText
        }
        const scope = new URL(document.baseURI).pathname
        window.dispatchEvent(new Event('offline'))
        await until(
            () =>
                select('.connection-overlay [role=status]')?.textContent.includes(
                    'Connection lost'
                ),
            'The connection status did not report going offline'
        )
        await until(() => !select('.connection-overlay'), 'The connection probe did not recover')
        const target = select('input[name=user]')
        const submit = target?.closest('form')?.querySelector('[type=submit]')
        if (!target || !submit?.disabled) throw new Error('Empty entity ID was accepted')
        target.value = '42'
        target.dispatchEvent(new Event('input', { bubbles: true }))
        await until(() => !submit.disabled, 'Quick Open did not react to a valid entity ID')
        submit.click()
        await until(
            () =>
                location.pathname === `${scope}dashboard/open/user/42` &&
                select('dialog[open] h2')?.textContent === 'fixture-42',
            'Quick Open did not display the selected user'
        )
        findButton('Close', select('dialog[open]')).click()
        await until(() => !select('dialog[open]'), 'Quick Open user dialog did not close')

        const userLink = (id) => select(`tbody a.username[href$="/open/user/${id}"]`)
        const waitForUserDialog = (id) =>
            until(() => {
                const dialog = select('dialog[open]')
                return dialog?.querySelector('h2')?.textContent === `fixture-${id}` && dialog
            }, `The table did not open user ${id}`)
        const firstUserLink = await until(() => userLink(42), 'The first user row did not load')
        const secondUserLink = userLink(43)
        secondUserLink.click()
        const cachedUserDialog = await waitForUserDialog(43)
        const strategy = cachedUserDialog
            .querySelector('input[name=trafficLimitStrategy]')
            .closest('.select-field')
            .querySelector('[role=combobox]')
        document.getElementById(strategy.getAttribute('aria-labelledby')).click()
        if (strategy.getAttribute('aria-expanded') !== 'false')
            throw new Error('The traffic strategy caption opened the options')
        const backdrop = { bubbles: true, pointerId: 1, clientX: 0, clientY: 0 }
        cachedUserDialog.dispatchEvent(new PointerEvent('pointerdown', backdrop))
        cachedUserDialog.dispatchEvent(new PointerEvent('pointerup', backdrop))
        expectDialogExit(cachedUserDialog)
        cachedUserDialog.dispatchEvent(new PointerEvent('click', backdrop))
        await until(() => !cachedUserDialog.isConnected, 'The user dialog did not close')

        firstUserLink.click()
        const firstUserDialog = await waitForUserDialog(42)
        const hwidLabel = firstUserDialog.querySelector('.hwid-checkbox')
        const hwidCheckbox = hwidLabel.querySelector('input[type=checkbox]')
        const hwidLimit = firstUserDialog.querySelector('input[name=hwidDeviceLimit]')
        hwidLabel.click()
        await until(
            () => hwidCheckbox.checked && hwidLimit.disabled,
            'The HWID checkbox did not disable the numerical limit'
        )
        const { width: hwidWidth, height: hwidHeight } = hwidCheckbox.getBoundingClientRect()
        if (Math.abs(hwidWidth - hwidHeight) > 1 || hwidWidth > 24 || hwidWidth < 16)
            throw new Error(`The HWID checkbox grew to ${hwidWidth} × ${hwidHeight}`)
        hwidLabel.click()
        await until(
            () => !hwidCheckbox.checked && !hwidLimit.disabled,
            'The HWID limit did not become editable again'
        )

        const squadChoice = await until(
            () => firstUserDialog.querySelector('.squad input[type=checkbox]'),
            'The internal squad choice did not load'
        )
        const wasSelected = squadChoice.checked
        squadChoice.closest('label').click()
        await until(
            () => squadChoice.checked !== wasSelected,
            'The internal squad choice did not respond to its label'
        )
        squadChoice.closest('label').click()
        await until(
            () => squadChoice.checked === wasSelected,
            'The internal squad choice could not be restored'
        )
        firstUserDialog.scrollTop = firstUserDialog.scrollHeight
        if (firstUserDialog.scrollTop === 0) throw new Error('The user dialog did not scroll')
        secondUserLink.click()
        const secondUserDialog = await waitForUserDialog(43)
        if (secondUserDialog.scrollTop !== 0)
            throw new Error('The next user dialog inherited the previous scroll position')

        findButton('Close', secondUserDialog).click()
        firstUserLink.click()
        const reopenedUserDialog = await waitForUserDialog(42)
        await new Promise(requestAnimationFrame)
        if (!reopenedUserDialog.isConnected || !reopenedUserDialog.open)
            throw new Error('Closing the previous user dismissed the next dialog')
        findButton('Close', reopenedUserDialog).click()
        await until(() => !reopenedUserDialog.isConnected, 'The reopened user dialog did not close')

        secondUserLink.click()
        const unrelatedEdit = await waitForUserDialog(43)
        setValue(unrelatedEdit.querySelector('[name=email]'), 'edited@example.test')
        findButton('Save', unrelatedEdit).click()
        await until(() => !unrelatedEdit.isConnected, 'The unrelated user edit did not save')

        userLink(43).click()
        const clearEdit = await waitForUserDialog(43)
        setValue(clearEdit.querySelector('[name=trafficLimit]'), '0')
        setValue(clearEdit.querySelector('[name=description]'), '')
        const savedTag = clearEdit.querySelector('[name=tag]')
        findButton('Clear', savedTag.closest('.select-field')).click()
        await until(() => savedTag.value === '', 'Clearing the tag did not update the form')
        findButton('Save', clearEdit).click()
        await until(() => !clearEdit.isConnected, 'The user field clears did not save')

        const header = select('header.dashboard-header')
        const logo = header.querySelector('.brand img')
        await logo.decode()
        if (!logo.getAttribute('src')?.startsWith(`${scope}favicons/gowave.svg?`)) {
            throw new Error('Dashboard artwork did not preserve the panel base')
        }
        const github = header.querySelector('a[href="https://github.com/aaaSaZaN/gowave"]')
        const telegram = github?.nextElementSibling
        if (
            telegram?.getAttribute('aria-label') !== 'Telegram' ||
            telegram.getAttribute('target') !== '_blank' ||
            telegram.getAttribute('rel') !== 'noopener noreferrer' ||
            telegram.nextElementSibling !== findButton('Recap', header) ||
            !telegram.nextElementSibling?.nextElementSibling
                ?.getAttribute('aria-label')
                ?.includes('theme')
        ) {
            throw new Error('Dashboard links and theme controls are out of order')
        }
        const language = findButton('Language', header)
        language.click()
        const languageMenu = await until(
            () => select('[role=menu][aria-label=Language]'),
            'Language menu did not open'
        )
        if (document.activeElement !== languageMenu)
            throw new Error('Language menu did not take focus')
        languageMenu.dispatchEvent(
            new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true })
        )
        if (!document.activeElement?.textContent.includes('English'))
            throw new Error('ArrowDown did not focus the first language choice')
        document.activeElement.dispatchEvent(
            new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })
        )
        await until(
            () => !languageMenu.isConnected && document.activeElement === language,
            'Escape did not dismiss the language menu and restore focus'
        )

        const buildButton = await until(() => {
            const button = findButton('Build info', header)
            return button && !button.disabled && button
        }, 'Build metadata did not load')
        buildButton.click()
        const build = await until(
            () => select('dialog.build-info[open]'),
            'Build Info did not open'
        )
        const command = await until(
            () => build.querySelector('.command code'),
            'Missing update command'
        )
        const [stableChannel, devChannel] = build.querySelectorAll('input[type=radio]')
        const bounds = build.getBoundingClientRect()
        const controls = [
            devChannel,
            findButton('Copy command', build),
            findButton('Copy build info', build),
            build.querySelector('a.github')
        ]
        if (
            build.scrollWidth > build.clientWidth ||
            command.scrollWidth <= command.clientWidth ||
            controls.some((element) => {
                const box = element?.getBoundingClientRect()
                return !box || box.left < bounds.left || box.right > bounds.right
            })
        ) {
            throw new Error('A long update command overflowed the Build Info dialog')
        }
        devChannel.click()
        await until(
            () => build.querySelector('[role=status]')?.textContent === 'Checking for updates…',
            'Dev check did not enter its loading state'
        )
        stableChannel.click()
        await until(
            () => stableChannel.checked && build.querySelector('.command code'),
            'Could not return to Stable while checking Dev'
        )
        devChannel.click()
        await until(() => findButton('Switch', build), 'Dev channel did not show the switch action')
        findButton('Close', build).click()
        await until(() => !select('dialog.build-info'), 'Build Info did not close')
        findButton('Recap', header).click()
        const recap = await until(() => select('dialog.recap[open]'), 'Recap dialog did not open')
        const total = await until(
            () =>
                select('.recap-card [data-metric=totalUsers]')?.textContent === '14' &&
                select('.recap-card [data-metric=totalUsers]'),
            'Recap did not decode the response'
        )
        findButton('Grid', recap).click()
        await until(() => select('.recap-card[data-style=grid]'), 'Recap background did not change')
        const swatch = findButton('#a78bfa', recap)
        swatch.click()
        await until(
            () =>
                swatch.getAttribute('aria-pressed') === 'true' &&
                select('.recap-card').style.getPropertyValue('--accent') === '#a78bfa',
            'Accent picker did not update the export card'
        )
        const note = recap.querySelector('input[name=recap-note]')
        note.value = 'Browser test'
        note.dispatchEvent(new Event('input', { bubbles: true }))
        await until(
            () => select('.recap-card .customNote')?.textContent === 'Browser test',
            'Custom note did not reach the export card'
        )
        findButton('Users', recap).click()
        await until(() => total.textContent === '🙈', 'Masked Recap value remained visible')
        findButton('Close', recap).click()
        await until(() => !select('dialog.recap'), 'Recap did not close')

        const theme = header.querySelector('[aria-label="Switch to light theme"]')
        theme.click()
        await until(
            () => localStorage.getItem('mantine-color-scheme-value') === 'light',
            'Theme preference was not saved'
        )
        const userMenu = select('nav a[href$="/management/users"][aria-haspopup="menu"]')
        userMenu.parentElement.dispatchEvent(new PointerEvent('pointerenter'))
        const squadLink = await until(
            () => select('nav [role=menu]:not([inert]) a[href$="/management/internal-squads"]'),
            'The Users menu did not expose its squad links'
        )
        await until(() => {
            const menu = squadLink.closest('[role=menu]')
            const menuBox = menu.getBoundingClientRect()
            const triggerBox = userMenu.getBoundingClientRect()
            const gapTarget = document.elementFromPoint(menuBox.left + 16, triggerBox.bottom + 4)
            return (
                Math.abs(menuBox.top - triggerBox.bottom - 8) < 0.02 &&
                gapTarget?.closest('[role=menu]') === menu
            )
        }, 'The desktop menu gap did not preserve pointer access')
        squadLink.click()
        if (!matchMedia('(prefers-reduced-motion: reduce)').matches) {
            await until(
                () =>
                    location.pathname.endsWith('/management/internal-squads') &&
                    select('.dashboard-content')
                        .getAnimations()
                        .some(
                            (animation) =>
                                animation.id === 'dashboard-page-entry' &&
                                animation.effect.getTiming().duration === 300
                        ),
                'Changing dashboard pages did not play the entry fade'
            )
        }
        const squadActions = await until(() => findButton('Squad actions'), 'Squads did not load')
        squadActions.click()
        const availableNodes = await until(
            () => findButton('Available nodes'),
            'Squad actions did not load'
        )
        availableNodes.click()
        const nodes = await until(
            () =>
                select('dialog[open] details summary')?.textContent.includes('Demo node') &&
                select('dialog[open]'),
            'Squad accessible nodes did not load'
        )
        nodes.querySelector('summary').click()
        if (!nodes.querySelector('details[open] ul')?.textContent.includes('Demo inbound')) {
            throw new Error('Squad node did not expose its active inbound')
        }
        findButton('Close', nodes).click()
        await until(() => !select('dialog'), 'Accessible nodes did not close')

        findButton('Squad actions').click()
        await until(() => findButton('Usage stats'), 'Squad actions did not reopen').then(
            (button) => button.click()
        )
        const usage = await until(
            () =>
                select('dialog[open]')?.textContent.includes('No data available') &&
                select('dialog[open]'),
            'Squad usage did not apply its default threshold'
        )
        const threshold = usage.querySelector('input[name=minTrafficGib]')
        threshold.value = '1.5'
        threshold.dispatchEvent(new Event('input', { bubbles: true }))
        findButton('Update', usage).click()
        await until(
            () => usage.querySelector(`a[href="${scope}dashboard/open/user/42"]`),
            'Squad usage did not apply the threshold or preserve the user link base path'
        )
        const more = await until(() => {
            const button = findButton('Load more', usage)
            return button && !button.disabled && button
        }, 'Squad usage did not enable pagination')
        more.click()
        await until(
            () =>
                usage.querySelector(`a[href="${scope}dashboard/open/user/43"]`) &&
                !findButton('Load more', usage),
            'Squad usage did not apply the response cursor'
        )
        findButton('Close', usage).click()
        await until(() => !select('dialog'), 'Squad usage did not close')

        const nativeConfirm = window.confirm
        window.confirm = () => true
        try {
            findButton('Squad actions').click()
            const remove = await until(
                () => findButton('Remove users'),
                'Squad actions did not reopen'
            )
            remove.click()
            await until(
                () => select('.squad-list li [title=Users]')?.textContent.trim() === '0',
                'Removing members did not refresh the squad'
            )
            findButton('Squad actions').click()
            const add = await until(() => {
                const button = findButton('Add all users')
                return button && !button.disabled && button
            }, 'Squad membership mutation did not settle')
            add.click()
            await until(
                () => select('.squad-list li [title=Users]')?.textContent.trim() === '2',
                'Adding members did not refresh the squad'
            )
        } finally {
            window.confirm = nativeConfirm
        }

        clickVisibleCenter(select('.squad-list .card-icon'))
        const squadInbounds = await until(
            () => select('dialog.squad-inbounds[open] input[type=search]')?.closest('dialog'),
            'Opening the squad did not show the searchable inbound drawer'
        )
        const inboundSearch = squadInbounds.querySelector('input[type=search]')
        inboundSearch.value = 'Second profile'
        inboundSearch.dispatchEvent(new Event('input', { bubbles: true }))
        const profileToggle = await until(
            () =>
                squadInbounds.querySelectorAll('.profile-toggle').length === 1 &&
                findButton('Second profile', squadInbounds),
            'Searching the squad did not match the profile name'
        )
        profileToggle.click()
        await until(
            () => squadInbounds.querySelector('.profile-inbounds input:checked'),
            'The grouped drawer did not show the saved inbound'
        )
        inboundSearch.value = ''
        inboundSearch.dispatchEvent(new Event('input', { bubbles: true }))
        findButton('Flat list', squadInbounds).click()
        const unselected = await until(
            () => squadInbounds.querySelector('input[value=unselected]'),
            'The flat inbound list did not have its selection filters'
        )
        unselected.click()
        const secondInbound = await until(
            () =>
                squadInbounds
                    .querySelector('.flat-list label')
                    ?.textContent.includes('extra-inbound') &&
                squadInbounds.querySelector('.flat-list input[type=checkbox]'),
            'The Unselected filter did not show the other inbound'
        )
        secondInbound.click()
        await until(
            () => squadInbounds.textContent.includes('No inbounds found'),
            'Selecting the inbound did not update the filtered list'
        )
        squadInbounds.querySelector('input[value=selected]').click()
        await until(
            () =>
                squadInbounds.querySelectorAll('.flat-list input[type=checkbox]:checked').length ===
                2,
            'The Selected filter did not retain both drafts'
        )
        squadInbounds.querySelector('.flat-list .inbound-actions button').click()
        const inboundPreview = await until(
            () => select('dialog[open] .json-report')?.closest('dialog'),
            'The inbound metadata did not open a JSON preview'
        )
        findButton('Close', inboundPreview).click()
        await until(() => !inboundPreview.isConnected, 'The inbound JSON preview did not close')
        findButton('Save', squadInbounds).click()
        await until(
            () =>
                !squadInbounds.isConnected &&
                select('.squad-list li [title=Inbounds]')?.textContent.trim() === '2',
            'Saving selected inbounds did not refresh the squad card'
        )

        const squadCard = select('.squad-list li')
        if (!findButton('Tags', squadCard)) findButton('Squad actions', squadCard).click()
        const tagsAction = await until(
            () => findButton('Tags', squadCard),
            'The squad menu did not include tag editing'
        )
        tagsAction.click()
        const editor = await until(() => select('dialog[open]'), 'Squad editor did not open')
        const tags = editor.querySelector('input[name=tags]')
        tags.value = ' demo, DEMO '
        tags.dispatchEvent(new Event('input', { bubbles: true }))
        const saveTags = await until(() => {
            const button = findButton('Save · Tags', editor)
            return button && !button.disabled && button
        }, 'Tag editing did not enable Save')
        saveTags.click()
        await until(
            () => select('.squad-list li')?.textContent.includes('DEMO') && tags.value === 'DEMO',
            'Tag changes did not normalize and refresh the squad'
        )
        findButton('Cancel', editor).click()
        await until(() => !select('dialog'), 'Squad editor did not close')

        const demoTag = await until(
            () =>
                [...document.querySelectorAll('.tag-bar button')].find((button) =>
                    button.textContent.includes('DEMO')
                ),
            'The saved squad tag did not appear in the filter bar'
        )
        demoTag.click()
        await until(
            () => demoTag.getAttribute('aria-pressed') === 'true' && select('.squad-list li'),
            'Tag filter did not retain the matching squad'
        )

        select('nav a[href$="/management/external-squads"]').click()
        const externalCard = await until(
            () => select(`.squad-list a[href$="/external-squad/${externalUuid}"]`),
            'External squad did not load'
        )
        externalCard.click()
        const external = await until(
            () =>
                select('dialog.external-squad-drawer[open] input[name=MIHOMO]')?.closest('dialog'),
            'External squad template selectors did not load'
        )
        const templateTypes = [...external.querySelectorAll('input[type=hidden]')].map(
            (field) => field.name
        )
        const templateInput = (type) =>
            external
                .querySelector(`input[name=${type}]`)
                .parentElement.querySelector('[role=combobox]')
        if (
            document.activeElement !== external ||
            templateTypes.join(',') !== 'XRAY_JSON,MIHOMO,STASH,SINGBOX,CLASH' ||
            !templateInput('XRAY_JSON').disabled
        ) {
            throw new Error(
                'External squad initial focus or template controls differ from the drawer'
            )
        }
        templateInput('MIHOMO').click()
        const mihomoOption = await until(
            () => select('dialog.external-squad-drawer [role=option]'),
            'Mihomo template dropdown did not open'
        )
        mihomoOption.click()
        await until(
            () => external.querySelector('input[name=MIHOMO]').value === mihomoUuid,
            'Mihomo template was not selected'
        )
        findButton('Save', external).click()
        await until(
            () =>
                external
                    .querySelector('[role=status]')
                    ?.textContent.includes('Operation completed'),
            'Saving the visible template did not preserve the hidden Base64 template'
        )
        const templatesTab = await until(() => {
            const tab = findButton('Templates', external)
            return tab && !tab.disabled && tab
        }, 'Saving templates left the tabs disabled')
        templatesTab.focus()
        templatesTab.dispatchEvent(
            new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true })
        )
        const addOverride = await until(
            () => external.querySelector('[role=combobox][aria-label="Add override"]'),
            'External squad settings did not open by keyboard'
        )
        const settingsTab = findButton('Settings', external)
        if (
            document.activeElement !== settingsTab ||
            settingsTab.getAttribute('aria-selected') !== 'true'
        ) {
            throw new Error('Keyboard navigation did not select and focus the settings tab')
        }
        addOverride.click()
        const randomize = await until(
            () =>
                [...external.querySelectorAll('[role=option]')].find(
                    (option) => option.textContent.trim() === 'Randomize hosts'
                ),
            'Randomize hosts was missing from the override list'
        )
        randomize.click()
        const override = await until(
            () => external.querySelector('input[role=switch][name=randomizeHosts]'),
            'Adding an override did not display its switch'
        )
        override.click()
        findButton('Save', external).click()
        await until(
            () =>
                external
                    .querySelector('[role=status]')
                    ?.textContent.includes('Operation completed'),
            'Could not save a selected subscription override'
        )
        findButton('Close', external).click()
        const externalActions = await until(
            () =>
                !external.isConnected &&
                findButton('Squad actions', select('.squad-list li') ?? document),
            'Closing the drawer did not return to the external squads list'
        )
        externalActions.click()
        const renameExternal = await until(
            () => findButton('Rename', select('.squad-list li') ?? document),
            'The external squad actions did not open'
        )
        renameExternal.click()
        await until(() => {
            const field = select('dialog[open] input[name=name]')
            return (
                field?.placeholder === 'Fixture external squad' && field === document.activeElement
            )
        }, 'External squad rename action did not open its dialog')
        findButton('Close', select('dialog[open]')).click()
        await until(
            () =>
                !select('dialog.external-squad-drawer') &&
                location.pathname.endsWith('/management/external-squads'),
            'The identity form did not return to the squads list'
        )

        select('nav a[href$="/management/config-profiles"]').click()
        await until(() => select('.profile-list li'), 'Config profiles did not load')
        clickVisibleCenter(select('.profile-list .profile-icon'))
        await until(
            () => location.pathname.includes('/management/config-profiles/'),
            'Clicking a profile card outside its title did not open it'
        )
        const configSave = await until(() => {
            const actions = select('.editor-actions')
            if (!actions?.querySelector('button')) return undefined
            const save = findButton('Save', actions)
            return save && !save.disabled ? save : findButton('Save anyway', actions)
        }, 'Config editor had no save action')
        configSave.click()
        if (configSave.textContent.trim() === 'Save anyway') {
            const saveAnyway = await until(
                () => select('dialog[open]'),
                'Saving an unvalidated config did not ask for confirmation'
            )
            findButton('Save', saveAnyway).click()
        }
        const configError = await until(
            () => select('.mantine-Notification-root[role=alert]'),
            'Config save failure did not show a notification'
        )
        if (
            configError.querySelector('.mantine-Notification-title')?.textContent !==
                'Update Config Profile' ||
            !configError
                .querySelector('.mantine-Notification-description')
                ?.textContent.includes('FOREIGN KEY constraint failed (787)')
        )
            throw new Error('The profile save notification lost its operation or backend message')
        findButton('Close', configError).click()
        await until(() => !configError.isConnected, 'The profile save notification did not close')

        select('nav a[href$="/management/config-profiles"]').click()
        await until(() => select('.profile-list li'), 'Config profiles did not reopen')
        window.dispatchEvent(new KeyboardEvent('keydown', { key: 'f', ctrlKey: true }))
        const spotlight = await until(
            () => select('.entity-spotlight[open]'),
            'Config profile spotlight did not open with Ctrl+F'
        )
        const searchProfiles = spotlight.querySelector('input')
        if (document.activeElement !== searchProfiles)
            throw new Error('Spotlight search did not focus')
        const initialResult = spotlight.querySelector('[aria-selected=true]')
        searchProfiles.dispatchEvent(
            new KeyboardEvent('keydown', { key: 'ArrowDown', cancelable: true, bubbles: true })
        )
        await until(() => {
            const result = spotlight.querySelector('[aria-selected=true]')
            return result && result !== initialResult
        }, 'ArrowDown did not change the active spotlight result')
        searchProfiles.value = 'first'
        searchProfiles.dispatchEvent(new Event('input', { bubbles: true }))
        await until(
            () =>
                spotlight.querySelectorAll('[role=option]').length === 1 &&
                spotlight.querySelector('mark')?.textContent === 'First',
            'Spotlight did not filter and highlight the matching query'
        )
        spotlight.close()
        await until(() => !spotlight.open, 'Spotlight did not close')
        const profileNames = () =>
            [...document.querySelectorAll('.profile-list h2')].map((heading) =>
                heading.textContent.trim()
            )
        const profileCards = [...document.querySelectorAll('.profile-list li')]
        clickVisibleCenter(findButton('Profile actions', profileCards[0]))
        const profileActions = await until(
            () => select('[role=menu][aria-label="Profile actions"]:popover-open'),
            'Profile actions did not open'
        )
        if (!findButton('Quick view', profileActions) || !findButton('Download', profileActions)) {
            throw new Error('Profile menu is missing Quick view or Download')
        }
        profileActions.hidePopover()
        const profileHandle = findButton('Reorder First profile', profileCards[0])
        const releaseProfile = dragBetween(profileHandle, profileCards[0], profileCards[1])
        await until(
            () =>
                ['--reorder-x', '--reorder-y'].some(
                    (property) => parseFloat(profileCards[1].style.getPropertyValue(property)) < 0
                ),
            'The next profile did not move out of the way during the drag'
        )
        releaseProfile()
        await until(
            () =>
                profileNames().join(',') === 'Second profile,First profile' &&
                profileHandle.hasAttribute('data-reorder-handle'),
            'Config profile reorder did not persist'
        )
        const secondProfile = select('.profile-list li')
        clickVisibleCenter(findButton('Inbounds (1)', secondProfile))
        const inbound = await until(
            () =>
                select('dialog[open] summary')?.textContent.includes('fixture-inbound') &&
                select('dialog[open] summary'),
            'Config profile inbounds did not load'
        )
        inbound.click()
        await until(
            () =>
                select('dialog[open] a[href*="/open/internal-squad/"]')?.textContent ===
                'Demo squad',
            'Config profile inbound did not link its squad'
        )
        findButton('Close', select('dialog[open]')).click()
        await until(() => !inbound.isConnected, 'Inbound dialog did not close')
        findButton('Nodes (1)', secondProfile).click()
        await until(
            () => select('dialog[open] a[href*="/open/node/"]')?.textContent === 'GeoCheck node',
            'Config profile did not link its active node'
        )
        findButton('Close', select('dialog[open]')).click()
        await until(() => !select('dialog[open]'), 'Node dialog did not close')
        findButton('Profile actions', secondProfile).click()
        await until(
            () => findButton('View computed', secondProfile),
            'Profile actions did not open'
        ).then((button) => button.click())
        await until(
            () => select('dialog[open] pre')?.textContent.includes('fixture-computed'),
            'Computed config did not load'
        )
        findButton('Close', select('dialog[open]')).click()
        await until(() => !select('dialog[open]'), 'Computed config dialog did not close')
        findButton('Profile actions', secondProfile).click()
        await until(() => findButton('Rename', secondProfile), 'Profile actions did not open').then(
            (button) => button.click()
        )
        const renameDialog = await until(() => select('dialog[open]'), 'Rename dialog did not open')
        const name = renameDialog.querySelector('input[name=name]')
        name.value = 'Renamed profile'
        name.dispatchEvent(new Event('input', { bubbles: true }))
        findButton('Save', renameDialog).click()
        await until(
            () =>
                !select('dialog[open]') &&
                profileNames().join(',') === 'Renamed profile,First profile',
            'Config profile rename did not persist'
        )
        findButton('Profile actions', select('.profile-list li')).click()
        await until(
            () => findButton('Tags', select('.profile-list li')),
            'Profile actions did not open'
        ).then((button) => button.click())
        const profileTags = await until(
            () => select('dialog[open]'),
            'Profile tag editor did not open'
        )
        const profileTagInput = profileTags.querySelector('input[name=tags]')
        profileTagInput.value = ' stage, STAGE '
        profileTagInput.dispatchEvent(new Event('input', { bubbles: true }))
        findButton('Save', profileTags).click()
        await until(
            () =>
                !select('dialog[open]') &&
                select('.profile-list li')?.textContent.includes('STAGE'),
            'Profile tags did not refresh'
        )
        select('.tag-bar button:nth-child(2)').click()
        await until(
            () => profileNames().join(',') === 'Renamed profile',
            'Profile tag filter did not select the matching profile'
        )

        findButton('Xray snippets').click()
        const snippets = await until(() => {
            const dialog = select('dialog[open]')
            return dialog && findButton('Edit', dialog) && dialog
        }, 'Xray snippets did not load')
        findButton('Edit', snippets).click()
        await until(
            () => snippets.querySelector('.monaco-editor'),
            'Snippet editor did not load Monaco'
        )
        findButton('Save', snippets).click()
        const apply = await until(
            () => findButton('Apply now', snippets),
            'Editing a snippet did not offer to apply it to nodes'
        )
        apply.click()
        await until(
            () => !findButton('Apply now', snippets) && snippets.querySelector('[role=status]'),
            'Snippet sync did not finish'
        )
        window.confirm = () => true
        try {
            findButton('Delete', snippets).click()
        } finally {
            window.confirm = nativeConfirm
        }
        const applyDeletion = await until(
            () => !findButton('Edit', snippets) && findButton('Apply now', snippets),
            'Deleting a snippet did not refresh the list and offer to apply it to nodes'
        )
        applyDeletion.click()
        await until(
            () => !findButton('Apply now', snippets) && snippets.querySelector('[role=status]'),
            'Syncing a deleted snippet did not finish'
        )
        findButton('Close', snippets).click()
        await until(() => !select('dialog[open]'), 'Snippet dialog did not close')

        findButton('Create', select('main')).click()
        const profileName = await until(
            () => select('dialog[open] input[name=name]'),
            'The profile creation form did not open'
        )
        if (document.activeElement !== profileName || !profileName.required)
            throw new Error('The profile name was not focused and required')
        if (
            findButton('Create', profileName.closest('dialog')).disabled ||
            profileName.validity.valid
        )
            throw new Error('The empty profile form did not preserve required-field validation')
        profileName.value = 'New profile'
        profileName.dispatchEvent(new Event('input', { bubbles: true }))
        await until(
            () => !findButton('Create', profileName.closest('dialog')).disabled,
            'The profile name was rejected'
        )
        findButton('Create', profileName.closest('dialog')).click()
        await until(
            () =>
                location.pathname.endsWith(
                    '/management/config-profiles/44444444-4444-4444-8444-444444444441'
                ),
            'Creating a profile did not open its editor'
        )

        select('nav a[href$="/management/nodes"]').click()
        await until(
            () => select('main.nodes-page ul a[href*="/open/node/"]'),
            'Node cards did not render before the scroll container could measure them'
        )
        findButton('Table', select('main')).click()
        await until(
            () => select('main.nodes-page table a[href*="/open/node/"]'),
            'Switching to the node table did not render its rows'
        )
        findButton('Cards', select('main')).click()
        const nodeLink = await until(
            () => select('main.nodes-page ul a[href*="/open/node/"]'),
            'Switching back to cards left the node list empty'
        )
        const nodeCards = [...document.querySelectorAll('main.nodes-page .node-item')]
        const firstNodeHandle = findButton('Actions', nodeCards[0])
        const releaseNode = dragBetween(firstNodeHandle, nodeCards[0], nodeCards[1])
        await until(
            () => parseFloat(nodeCards[1].style.getPropertyValue('--reorder-y')) < 0,
            'The next node did not move out of the way during the drag'
        )
        window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', cancelable: true }))
        releaseNode()
        await until(
            () => !nodeCards[1].hasAttribute('data-reorder-moving'),
            'Cancelling did not restore the node cards'
        )
        if (select('main.nodes-page .node-item') !== nodeCards[0])
            throw new Error('Cancelling changed the saved node order')
        dragBetween(firstNodeHandle, nodeCards[0], nodeCards[1])()
        await until(
            () =>
                select('main.nodes-page .node-item') === nodeCards[1] &&
                firstNodeHandle.hasAttribute('data-reorder-handle'),
            'The node order did not persist'
        )
        dragBetween(firstNodeHandle, nodeCards[0], nodeCards[1])()
        await until(
            () =>
                select('main.nodes-page .node-item') === nodeCards[0] &&
                firstNodeHandle.hasAttribute('data-reorder-handle'),
            'The original node order did not return'
        )
        if (select('.notifications')?.textContent.includes('Operation completed'))
            throw new Error('Reordering nodes showed a success notification')
        findButton('Search', select('main')).click()
        const nodeSearch = await until(
            () => select('dialog.entity-spotlight[open]'),
            'Nodes did not open the search spotlight'
        )
        const nodeResult =
            findButton('GeoCheck node', nodeSearch) ??
            [...nodeSearch.querySelectorAll('[role=option]')].find((item) =>
                item.textContent.includes('GeoCheck node')
            )
        nodeResult.click()
        const foundNode = await until(
            () => select('dialog.node-details[open]'),
            'Node spotlight did not open the selected node'
        )
        if (!location.pathname.endsWith('/management/nodes'))
            throw new Error('Node spotlight changed the current route')
        findButton('Close', foundNode).click()
        await until(() => !foundNode.isConnected, 'Node spotlight details did not close')

        findButton('Create', select('main')).click()
        const country = await until(
            () => select('dialog.node-creation[open] [role=combobox]'),
            'Node creation did not display the country selector'
        )
        country.focus()
        country.value = 'Canada'
        country.dispatchEvent(new Event('input', { bubbles: true }))
        await until(
            () => select('dialog.node-creation [role=option]')?.textContent.includes('Canada'),
            'The country selector did not filter its options'
        )
        country.dispatchEvent(
            new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true })
        )
        country.dispatchEvent(
            new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })
        )
        await until(
            () => select('dialog.node-creation input[name=countryCode]')?.value === 'CA',
            'Enter did not select and serialize the country'
        )
        const wizard = country.closest('dialog')
        setValue(wizard.querySelector('input[name=name]'), 'Animation test node')
        setValue(wizard.querySelector('input[name=address]'), '192.0.2.40')
        findButton('Next', wizard).click()
        const selectProfile = await until(
            () => findButton('Select', wizard),
            'The node wizard did not show profile selection'
        )
        selectProfile.click()
        const profilePicker = await until(
            () => select('dialog[open] .profile-heading')?.closest('dialog'),
            'The profile picker did not open'
        )
        await until(() => findButton('Select all', profilePicker), 'No inbound was available').then(
            (button) => button.click()
        )
        findButton('Save', profilePicker).click()
        await until(() => !profilePicker.isConnected, 'The node profile selection did not close')
        findButton('Create node', wizard).click()
        const animateWizard = matchMedia('(prefers-reduced-motion: no-preference)').matches
        for (const [state, asset] of [
            ['connecting', 'satellite'],
            ['error', 'stop-v2'],
            ['connected', 'checkmark']
        ]) {
            await until(() => {
                const panel = wizard.querySelector(`.status-panel[data-state="${state}"]`)
                if (!animateWizard) return panel?.querySelector('.status-animation svg')
                return (
                    panel?.querySelector('canvas.animationReady') &&
                    performance
                        .getEntriesByType('resource')
                        .some(({ name }) => name.endsWith(`/lotties/${asset}.lottie`))
                )
            }, `The node wizard did not render the ${state} state`)
        }
        if (animateWizard && !wizard.querySelector('.step-bars .filled').getAnimations().length)
            throw new Error('The node wizard progress stripes did not animate')
        findButton('Close', wizard).click()
        await until(() => !select('dialog.node-creation'), 'The node creation dialog did not close')
        const nodeList = select('main.nodes-page')
        nodeLink.click()
        await until(
            () => select('dialog.node-details[open]'),
            'Clicking a node in the list did not open its settings'
        )
        if (
            nodeList !== select('main.nodes-page') ||
            !location.pathname.endsWith('/management/nodes')
        )
            throw new Error('Opening node settings replaced the list behind the dialog')
        const nodeActions = await until(
            () => findButton('More actions', select('.node-details') ?? document),
            'Node actions did not load'
        )
        nodeActions.click()
        const geoButton = await until(() => {
            const button = findButton('GeoCheck')
            return button && !button.disabled && button
        }, 'GeoCheck was unavailable on a supported node')
        geoButton.click()
        const geo = await until(
            () => select('.node-details dialog[open]'),
            'GeoCheck dialog did not open'
        )
        findButton('IP address', geo).click()
        const ip = await until(
            () => geo.querySelector('input[role=combobox]'),
            'GeoCheck IP field did not appear'
        )
        ip.value = '192.0.2.10'
        ip.dispatchEvent(new Event('input', { bubbles: true }))
        await until(
            () => !findButton('Run check', geo).disabled,
            'GeoCheck did not accept the selected IP'
        )
        findButton('Run check', geo).click()
        await until(
            () =>
                geo.querySelector('canvas.ready') &&
                geo.querySelector('[role=status]')?.textContent.includes('Running'),
            'GeoCheck did not show the animated progress state'
        )
        await until(
            () =>
                geo.querySelector('img[alt="GeoCheck"]')?.naturalWidth === 120 &&
                !findButton('Download', geo)?.disabled &&
                findButton('Copy', geo),
            'GeoCheck did not display the SVG report with image export controls'
        )
        let geocheckDownload
        const interceptGeocheckDownload = (event) => {
            if (!(event.target instanceof HTMLAnchorElement)) return
            if (!/^geocheck-.*\.png$/.test(event.target.download)) return
            event.preventDefault()
            geocheckDownload = fetch(event.target.href).then((response) => response.blob())
        }
        document.addEventListener('click', interceptGeocheckDownload, true)
        try {
            findButton('Download', geo).click()
            const png = await until(() => geocheckDownload, 'GeoCheck did not export a PNG')
            const bitmap = await createImageBitmap(png)
            try {
                if (png.type !== 'image/png' || bitmap.width <= 120 || bitmap.height <= 80)
                    throw new Error('GeoCheck exported an image without the screenshot frame')
            } finally {
                bitmap.close()
            }
        } finally {
            document.removeEventListener('click', interceptGeocheckDownload, true)
        }
        if (geo.querySelector('.trace')) throw new Error('The trace was visible on the image tab')
        findButton('Traceroute', geo).click()
        const targets = await until(() => {
            const entries = geo.querySelectorAll('.trace-target')
            return entries.length === 2 && entries
        }, 'GeoCheck did not display the trace targets')
        if (
            targets[0].getAttribute('aria-pressed') !== 'true' ||
            !geo.querySelector('.trace-detail')?.textContent.includes('Example trace')
        )
            throw new Error('The first route was not selected')
        targets[1].click()
        await until(
            () =>
                targets[1].getAttribute('aria-pressed') === 'true' &&
                geo.querySelector('.hop-segment')?.textContent.includes('AS64500') &&
                geo.querySelector('.hop-address')?.textContent.includes('192.0.2.1') &&
                !geo.querySelector('img[alt="GeoCheck"]'),
            'Changing the target did not show its grouped hops'
        )
        findButton('JSON', geo).click()
        await until(
            () => geo.querySelector('.json-report')?.textContent.includes('connectivity'),
            'GeoCheck did not show the JSON tree'
        )
        findButton('Report', geo).click()
        await until(
            () => geo.querySelector('img[alt="GeoCheck"]')?.naturalWidth === 120,
            'GeoCheck did not return to the report image'
        )
        findButton('Try again', geo).click()
        await until(() => findButton('Run check', geo), 'GeoCheck did not reset its form')
        findButton('Default', geo).click()
        findButton('Run check', geo).click()
        await until(
            () => geo.querySelector('[role=alert]')?.textContent.includes('node disabled'),
            'GeoCheck did not show the failed job message'
        )
        findButton('Close', geo).click()
        await until(() => !geo.isConnected, 'GeoCheck dialog did not close')

        findButton('Metadata').click()
        const metadata = await until(() => {
            const dialog = select('.node-details dialog[open]')
            return (
                dialog?.querySelector('.json-report')?.textContent.includes('fixture-location') &&
                dialog
            )
        }, 'Node metadata did not load')
        findButton('Close', metadata).click()
        await until(() => !metadata.isConnected, 'Metadata dialog did not close')

        findButton('Close', select('.node-details')).click()
        await until(() => !select('.node-details'), 'Node editor did not close')
        for (const [name, source] of [
            ['GeoCheck node', 'local'],
            ['Panel core node', 'panel'],
            ['Legacy node', undefined]
        ]) {
            const link = await until(
                () =>
                    [...document.querySelectorAll('main a[href*="/open/node/"]')].find((link) =>
                        link.textContent.trim().endsWith(name)
                    ),
                `Missing node: ${name}`
            )
            link.click()
            const coreVersion = await until(
                () => select('.node-details button.core-version'),
                'Missing Xray version control'
            )
            const managedLocally = 'Xray Core is managed on the node'
            if (source === 'local') {
                if (coreVersion.title !== managedLocally)
                    throw new Error('The local core version did not indicate local management')
                if (coreVersion.disabled) throw new Error('The local core details were unavailable')
                coreVersion.click()
                const versions = await until(
                    () => select('dialog.node-versions[open]'),
                    'Local core version dialog did not open'
                )
                await until(
                    () =>
                        versions.textContent.includes(managedLocally) &&
                        versions.textContent.includes('CUSTOM_CORE_URL') &&
                        versions.textContent.includes('gowave-node core update'),
                    'Missing local core notice'
                )
                if (findButton('Install now', versions))
                    throw new Error('Local core can be updated')
                findButton('Gowave Node Go', versions).click()
            } else {
                if (coreVersion.disabled)
                    throw new Error('Panel and legacy cores must offer updates')
                coreVersion.click()
            }
            const versions = await until(
                () => select('dialog.node-versions[open]'),
                'Version dialog did not open'
            )
            const confirm = await until(() => {
                const checkbox = versions.querySelector('input[name=confirm]')
                return checkbox && !checkbox.disabled && checkbox
            }, 'Update confirmation was unavailable')
            confirm.click()
            const install = await until(() => {
                const button = findButton('Install now', versions)
                return button && !button.disabled && button
            }, 'Confirmation did not enable the update')
            install.click()
            if (source === undefined) {
                await until(
                    () => versions.textContent.includes(managedLocally),
                    '409 did not show the local core notice'
                )
                if (findButton('Install now', versions))
                    throw new Error('409 did not disable core updates')
                findButton('Gowave Node Go', versions).click()
                await until(
                    () => versions.querySelector('input[name=confirm]:enabled'),
                    'Node update was unavailable after the core conflict'
                )
            } else {
                await until(() => {
                    const alert = versions.querySelector('[role=alert]')
                    if (alert) throw new Error(`Update for ${name} failed: ${alert.textContent}`)
                    return versions.textContent.includes('Update accepted.')
                }, `Update for ${name} did not report success`)
            }
            findButton('Close', versions).click()
            await until(() => !versions.isConnected, 'Version dialog did not close')
            findButton('Close', select('.node-details')).click()
            await until(() => !select('.node-details'), 'Node editor did not close')
        }
        select('nav a[href$="/management/settings"]').click()
        const layoutSwitch = await until(
            () => select('.layout-toggle input[type=checkbox]'),
            'Desktop layout setting did not appear'
        )
        const experimental = select('.experimental')
        if (experimental.querySelectorAll('input, [role=checkbox]').length !== 1) {
            throw new Error('Only the legacy layout option should remain')
        }
        layoutSwitch.click()
        const nodesToggle = await until(
            () =>
                [...document.querySelectorAll('.sidebar-navigation button[aria-controls]')].find(
                    (button) => button.textContent.trim() === 'Nodes'
                ),
            'The sidebar Nodes disclosure did not appear'
        )
        const { state: preferences } = JSON.parse(localStorage.getItem('viewPreferencesStore'))
        if (
            !preferences.experimental.legacyLayoutStyle ||
            Object.keys(preferences.experimental).length !== 1 ||
            'quickLinks' in preferences ||
            'launcherColumns' in preferences ||
            'launcherPosition' in preferences
        ) {
            throw new Error('Saving the layout retained retired frontend preferences')
        }
        const submenu = document.getElementById(nodesToggle.getAttribute('aria-controls'))
        if (!submenu.inert || submenu.getBoundingClientRect().height !== 0) {
            throw new Error('Closed Nodes links still occupy space or accept focus')
        }

        const animates = () =>
            submenu
                .getAnimations()
                .some((animation) => animation.transitionProperty === 'grid-template-rows')
        const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)').matches
        nodesToggle.click()
        if (!reducedMotion) await until(animates, 'Nodes did not animate open')
        await until(
            () => !submenu.inert && submenu.getBoundingClientRect().height > 0 && !animates(),
            'Nodes submenu did not finish opening'
        )
        nodesToggle.click()
        if (!reducedMotion) await until(animates, 'Nodes did not animate closed')
        await until(
            () => submenu.inert && submenu.getBoundingClientRect().height === 0,
            'Nodes submenu did not finish closing'
        )
        select('nav a[href$="/management/plugins"]').click()
        const openLists = await until(
            () => findButton('Shared lists'),
            'Shared lists button did not load'
        )
        openLists.click()
        const folder = await until(
            () => select('dialog[open]') && findButton('Network · 2', select('dialog[open]')),
            'Shared list folder did not load'
        )
        folder.click()
        const listEntry = await until(
            () => select('dialog[open] .list-entry'),
            'Shared list rows did not load'
        )
        if (!listEntry.textContent.includes('ext:Network/'))
            throw new Error('Shared list did not show its full plugin reference')
        findButton('Close', select('dialog[open]')).click()
        await until(() => !select('dialog[open]'), 'Shared lists did not close')
        select('main a[href*="/management/plugins/"]').click()
        const pluginEditor = await until(
            () => select('.plugin-editor .monaco-editor'),
            'Plugin Monaco input did not mount'
        )
        const editorLine = await until(
            () => pluginEditor.querySelector('.view-line'),
            'Plugin Monaco did not render the document'
        )
        editorLine.scrollIntoView({ block: 'center' })
        const { x, y, height } = editorLine.getBoundingClientRect()
        document.body.dataset.editorInput = JSON.stringify({
            text: 'dirty-check',
            x: x + 4,
            y: y + height / 2
        })
        await until(
            () => pluginEditor.querySelector('.view-lines')?.textContent?.includes('dirty-check'),
            'Plugin Monaco did not accept browser input'
        )
        const backFromPlugin = findButton('Back')
        backFromPlugin.click()
        const unsaved = await until(
            () =>
                [...document.querySelectorAll('dialog[open]')].find((entry) =>
                    findButton('Stay', entry)
                ),
            'Unsaved plugin edits did not open a panel confirmation'
        )
        findButton('Stay', unsaved).click()
        await until(() => !unsaved.isConnected, 'Stay did not close the confirmation')
        if (!select('.plugin-editor')) throw new Error('Stay discarded the editor')
        backFromPlugin.click()
        const leavePrompt = await until(
            () =>
                [...document.querySelectorAll('dialog[open]')].find((entry) =>
                    findButton('Leave', entry)
                ),
            'Second attempt to leave did not open confirmation'
        )
        findButton('Leave', leavePrompt).click()
        await until(
            () => location.pathname === `${scope}dashboard/management/plugins`,
            'Leave did not complete navigation'
        )
        const createPlugin = await until(
            () => select('main') && findButton('Create', select('main')),
            'Plugins did not reopen'
        )
        createPlugin.click()
        const pluginName = await until(
            () => select('dialog[open] input[placeholder="My Node Plugin"]'),
            'The plugin creation form did not open'
        )
        if (
            document.activeElement !== pluginName ||
            !pluginName.required ||
            pluginName.validity.valid
        )
            throw new Error('The plugin name was not focused and required')
        pluginName.value = 'x'
        pluginName.dispatchEvent(new Event('input', { bubbles: true }))
        findButton('Create', pluginName.closest('dialog')).click()
        await until(
            () =>
                pluginName
                    .closest('dialog')
                    ?.querySelector('[role=alert]')
                    ?.textContent.includes('at least 2'),
            'An invalid plugin name did not show a readable validation error'
        )
        pluginName.value = 'New plugin'
        pluginName.dispatchEvent(new Event('input', { bubbles: true }))
        findButton('Create', pluginName.closest('dialog')).click()
        await until(
            () =>
                location.pathname ===
                    `${scope}dashboard/management/plugins/11111111-4444-4111-8111-111111111111` &&
                select('.plugin-editor'),
            'Creating a plugin did not open its editor'
        )
        await until(() => select('.plugin-editor .view-lines'), 'Plugin editor did not load')
        if (!findButton('Save', select('.plugin-editor'))?.disabled)
            throw new Error('The plugin editor allowed saving an unchanged document')
        findButton('Back').click()
        await until(
            () => location.pathname === `${scope}dashboard/management/plugins`,
            'The created plugin editor did not return to the list'
        )
        const deviceInspector = await until(
            () => select('nav.sidebar-navigation a[href$="/tools/hwid-inspector"]'),
            'The sidebar did not expose the HWID Inspector'
        )
        deviceInspector.click()
        const platform = await until(
            () => select('#hwid-platforms + .platform-grid button'),
            'The device platform card did not render'
        )
        if (
            !platform.textContent.includes('Fixture client') ||
            platform.textContent.includes('Rare client')
        ) {
            throw new Error('The device platform card showed the wrong app preview')
        }
        platform.click()
        const platformDialog = await until(
            () => select('dialog[open]'),
            'The device app breakdown did not open'
        )
        if (!platformDialog.textContent.includes('Rare client')) {
            throw new Error('The device app breakdown omitted a rare app')
        }
        findButton('Close', platformDialog).click()
        await until(() => !platformDialog.isConnected, 'The device app breakdown did not close')
        const leaders = await until(
            () => select('section[aria-label="Top users by devices"]:has(.page-range)'),
            'The device leaderboard did not load'
        )
        if (leaders.querySelectorAll('tbody tr').length !== 5) {
            throw new Error('The device leaderboard did not show five users on the first page')
        }
        findButton('Next', leaders).click()
        await until(
            () => leaders.querySelector('tbody')?.textContent.includes('device-user-6'),
            'The device leaderboard did not load the second page'
        )
        const pageSize = leaders.querySelector('select')
        pageSize.value = '10'
        pageSize.dispatchEvent(new Event('change', { bubbles: true }))
        await until(
            () => leaders.querySelectorAll('tbody tr').length === 6,
            'Changing the leaderboard page size did not return to the first page'
        )
        const hwidPath = location.pathname
        findButton('View user', leaders).click()
        const leaderProfile = await until(() => {
            const dialog = select('dialog[open]')
            return dialog?.textContent.includes('fixture-42') ? dialog : null
        }, 'Opening an HWID leaderboard entry did not open its profile')
        if (location.pathname !== hwidPath)
            throw new Error('The HWID leaderboard navigated away from its page')
        findButton('Close', leaderProfile).click()
        await until(
            () => !leaderProfile.isConnected && leaders.querySelectorAll('tbody tr').length === 6,
            'Closing the user profile lost the leaderboard state'
        )

        select('nav.sidebar-navigation a[href$="/tools/srh-inspector"]').click()
        const hourly = await until(() => {
            const chart = select('#hourly-request-heading + .hourly-chart')
            return chart?.querySelectorAll('.highcharts-series .highcharts-point').length === 3
                ? chart
                : null
        }, 'The subscription history chart did not render three hours')
        await until(
            () =>
                [...hourly.querySelectorAll('.highcharts-axis-title')].some(
                    (title) => title.textContent === 'Requests'
                ) && hourly.querySelector('.highcharts-root')?.getAttribute('height') === '300',
            'The subscription history chart did not inherit the reference axes and height'
        )
        const historyRow = await until(
            () => select('.inspector-panel tbody tr:has(a[aria-label="IP info"])'),
            'The history row did not offer an IP information link'
        )
        const historyIp = historyRow.querySelector('a[aria-label="IP info"]')
        if (
            historyIp.href !== 'https://ipinfo.io/2001%3Adb8%3A%3A42' ||
            historyIp.target !== '_blank' ||
            !select('.inspector-panel button[aria-label="IP info"]:disabled')
        ) {
            throw new Error('Inspector IP controls did not handle IPv6 and missing addresses')
        }
        const historyPath = location.pathname
        findButton('View user', historyRow).click()
        const inspectorUser = await until(() => {
            const dialog = select('dialog[open]')
            return dialog?.textContent.includes('fixture-42') ? dialog : null
        }, 'Opening an Inspector row did not load its user profile')
        if (location.pathname !== historyPath || !historyRow.isConnected) {
            throw new Error('The user profile replaced the Inspector page')
        }
        findButton('Close', inspectorUser).click()
        await until(() => !inspectorUser.isConnected, 'The Inspector user profile did not close')

        select('nav.sidebar-navigation a[href$="/tools/torrent-blocker-reports"]').click()
        const rawButton = await until(
            () => findButton('Raw Report'),
            'The Torrent Blocker report did not have a JSON action'
        )
        const reportRow = rawButton.closest('tr')
        const reportedNode = reportRow.querySelector('button.cell[title="GeoCheck node"]')
        if (reportedNode?.querySelector('.cell-prefix')?.textContent !== '🇺🇸')
            throw new Error('The report did not show the node country flag')
        const nodeName = reportedNode.querySelector('.cell-value')
        if (nodeName?.textContent !== 'GeoCheck node')
            throw new Error('The report did not render the complete node name')
        if (nodeName.scrollWidth > nodeName.clientWidth)
            throw new Error('The report truncated a short node name beside its country flag')
        let copiedNode
        const originalClipboard = navigator.clipboard.writeText
        navigator.clipboard.writeText = (value) => {
            copiedNode = value
            return Promise.resolve()
        }
        try {
            reportedNode.click()
            await until(() => copiedNode, 'The report node name was not copied')
            if (copiedNode !== 'GeoCheck node')
                throw new Error('Copying the node name included its country flag')
        } finally {
            navigator.clipboard.writeText = originalClipboard
        }
        rawButton.click()
        const rawReport = await until(
            () => select('dialog[open] .json-report'),
            'The Torrent Blocker report did not open its JSON dialog'
        )
        const xrayBranch = [...rawReport.querySelectorAll('details > summary')].find((summary) =>
            summary.textContent.includes('xrayReport')
        )?.parentElement
        if (
            !xrayBranch?.open ||
            !xrayBranch.textContent.includes('bittorrent') ||
            !rawReport.textContent.includes('2026-05-01T11:00:00.000Z')
        ) {
            throw new Error(
                'The JSON report omitted fields or formatted its timestamps incorrectly'
            )
        }
        xrayBranch.querySelector('summary').click()
        if (xrayBranch.open) throw new Error('The JSON report branch did not collapse')
        findButton('Close', rawReport.closest('dialog')).click()
        await until(() => !rawReport.isConnected, 'The JSON report did not close')

        nodesToggle.click()
        const usageLink = await until(
            () =>
                select('nav.sidebar-navigation a[href$="/management/stats/nodes"]:not([inert] *)'),
            'The sidebar did not expose node statistics'
        )
        usageLink.click()
        const trafficBar = await until(() => {
            const point = select('.daily .highcharts-series .highcharts-point')
            return point?.getBoundingClientRect().width > 0 ? point : null
        }, 'Node traffic statistics did not render the stacked chart')
        trafficBar.scrollIntoView({ block: 'center' })
        const bar = trafficBar.getBoundingClientRect()
        const pointEvent = {
            bubbles: true,
            clientX: bar.left + bar.width / 2,
            clientY: bar.top + bar.height / 2
        }
        trafficBar.dispatchEvent(new MouseEvent('mouseover', pointEvent))
        await until(
            () => select('.daily .highcharts-tooltip')?.textContent.includes('GeoCheck node'),
            'The chart tooltip did not show the hovered node'
        )
        trafficBar.dispatchEvent(new MouseEvent('click', pointEvent))
        const trafficDetails = await until(
            () => select('dialog[open] .details')?.closest('dialog'),
            'Clicking a traffic bar did not open its day'
        )
        const trafficRows = [...trafficDetails.querySelectorAll('tbody tr')].map((row) =>
            row.textContent.trim()
        )
        if (
            trafficRows.length !== 2 ||
            !trafficRows.some((row) => row.includes('GeoCheck node')) ||
            !trafficRows.some((row) => row.includes('Panel core node'))
        ) {
            throw new Error('The day breakdown did not include both nodes in the stack')
        }
        findButton('Close', trafficDetails).click()
        await until(() => !trafficDetails.isConnected, 'Traffic detail did not close')

        const rangeRequests = () =>
            performance
                .getEntriesByType('resource')
                .map(({ name }) => new URL(name))
                .filter(({ pathname }) => pathname === '/api/bandwidth-stats/nodes/')
        const selectTrafficRange = async () => {
            select('.range > button').click()
            return await until(
                () => select('dialog.range-dialog[open]'),
                'The traffic date range did not open its calendar'
            )
        }
        const rangeDialog = await selectTrafficRange()
        const [earlierDate, laterDate] = rangeDialog.querySelectorAll('[data-date]:enabled')
        const requestsBefore = rangeRequests().length
        laterDate.click()
        await until(
            () => rangeDialog.open && laterDate.getAttribute('aria-pressed') === 'true',
            'The first calendar click did not mark the start of a draft range'
        )
        if (rangeRequests().length !== requestsBefore) {
            throw new Error('Selecting only the start date submitted a traffic range')
        }
        earlierDate.click()
        await until(
            () =>
                !rangeDialog.open &&
                rangeRequests()
                    .slice(requestsBefore)
                    .some(
                        ({ searchParams }) =>
                            searchParams.get('start') === earlierDate.dataset.date &&
                            searchParams.get('end') === laterDate.dataset.date
                    ),
            'The traffic calendar did not send the ordered date range'
        )
        const sameDayDialog = await selectTrafficRange()
        const sameDay = sameDayDialog.querySelector('[data-date]:enabled')
        sameDay.click()
        sameDay.click()
        await until(
            () =>
                !sameDayDialog.open &&
                rangeRequests().some(
                    ({ searchParams }) =>
                        searchParams.get('start') === sameDay.dataset.date &&
                        searchParams.get('end') === sameDay.dataset.date
                ),
            'The traffic calendar did not accept a single day'
        )
        const presetDialog = await selectTrafficRange()
        findButton('Current month', presetDialog).click()
        const now = new Date()
        const prefix = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-`
        await until(
            () =>
                !presetDialog.open &&
                rangeRequests().some(
                    ({ searchParams }) =>
                        searchParams.get('start') === `${prefix}01` &&
                        searchParams.get('end') ===
                            `${prefix}${String(now.getDate()).padStart(2, '0')}`
                ),
            'The current month preset did not apply through today'
        )

        const metricsLink = select('nav.sidebar-navigation a[href$="/management/metrics/nodes"]')
        metricsLink.click()
        const metricsCards = await until(() => {
            const cards = [...document.querySelectorAll('main.metrics-page article')]
            return cards.length === 3 && cards
        }, 'Node Metrics did not render the fixture nodes')
        const [geoMetrics, panelMetrics, legacyMetrics] = metricsCards
        await until(() => {
            const first = geoMetrics.getBoundingClientRect()
            const second = panelMetrics.getBoundingClientRect()
            const third = legacyMetrics.getBoundingClientRect()
            return matchMedia('(width > 64em)').matches
                ? first.top === second.top &&
                      first.left < second.left &&
                      third.left === second.left &&
                      third.top < first.bottom
                : first.top < second.top &&
                      second.top < third.top &&
                      first.left === second.left &&
                      second.left === third.left
        }, 'Node Metrics did not use the expected card layout')
        const metricsSource = select('.metrics-source')
        const metricsInfo = findButton('Additional Information', metricsSource)
        metricsInfo.click()
        await until(
            () =>
                getComputedStyle(metricsSource.querySelector('[role=tooltip]')).visibility ===
                    'visible' && metricsSource.textContent.includes('Node startup'),
            'The Prometheus tooltip did not open'
        )
        const inboundTraffic = geoMetrics.querySelector('[aria-label="Inbound Traffic"]')
        const outbound = geoMetrics.querySelector('[aria-label="Outbound Traffic"]')
        if (
            !geoMetrics.textContent.includes('Fixture Hosting') ||
            !inboundTraffic.textContent.includes('12.5 MiB') ||
            inboundTraffic.textContent.includes('idle-inbound') ||
            !outbound.textContent.includes('4 KiB') ||
            !panelMetrics.textContent.includes('No inbound traffic data')
        ) {
            throw new Error('The metrics cards omitted traffic, provider or empty state details')
        }
        findButton('Show zero values', geoMetrics).click()
        await until(
            () =>
                inboundTraffic.textContent.includes('idle-inbound') &&
                inboundTraffic.textContent.includes('0 B'),
            'The zero-value control did not reveal the idle inbound'
        )
        findButton('Hide zero values', geoMetrics).click()
        await until(
            () => !inboundTraffic.textContent.includes('idle-inbound'),
            'The zero-value control did not hide the idle inbound'
        )
        const metricsPath = location.pathname
        findButton('GeoCheck node', geoMetrics).click()
        const metricsNodeDialog = await until(
            () => select('dialog.node-details[open]'),
            'The metrics card did not open the node dialog'
        )
        findButton('Close', metricsNodeDialog).click()
        await until(
            () =>
                !metricsNodeDialog.open &&
                location.pathname === metricsPath &&
                geoMetrics.isConnected,
            'Closing the node dialog did not return to the metrics cards'
        )

        select('nav.sidebar-navigation a[href$="/crm/infra-billing"]').click()
        const billingDate = await until(
            () => select('section[aria-labelledby=billing-nodes] .billing-date'),
            'The billing node card did not load'
        )
        await until(
            () =>
                select(
                    'section[aria-labelledby=billing-history] .billing-card'
                )?.textContent.includes('$12.50') &&
                select(
                    'section[aria-labelledby=billing-providers] .billing-card'
                )?.textContent.includes('Fixture Hosting'),
            'Billing cards omitted the invoice or provider details'
        )
        billingDate.click()
        const dateInput = await until(
            () => select('dialog[open] input[name=date]'),
            'The billing date did not open its editor'
        )
        if (dateInput.value !== '2025-05-02') {
            throw new Error('The billing date editor opened on the wrong day')
        }
        const billingDialog = dateInput.closest('dialog')
        findButton('Next month', billingDialog).click()
        await until(
            () =>
                dateInput.value === '2025-06-02' &&
                billingDialog
                    .querySelector('[data-date="2025-06-02"]')
                    ?.getAttribute('aria-pressed') === 'true' &&
                billingDialog.querySelector('input[name=currentDate]').value === '2 May 2025',
            'The next month preset did not update the selection and retain the current date'
        )
        findButton('Update', billingDialog).click()
        await until(
            () =>
                !select('dialog[open]') &&
                select('.billing-date')?.textContent.includes('2 June 2025'),
            'The calendar did not save the new billing day and refresh the card'
        )
        findButton('Bill record').click()
        const providerSearch = await until(
            () => select('dialog[open] [role=combobox]'),
            'The bill record editor did not show the provider search'
        )
        providerSearch.click()
        providerSearch.value = 'Hosting'
        providerSearch.dispatchEvent(new Event('input', { bubbles: true }))
        const providerOption = await until(
            () => select('dialog[open] [role=option]'),
            'Searching billing providers did not show a matching option'
        )
        providerOption.click()
        await until(
            () => providerSearch.closest('dialog').querySelector('input[name=provider]').value,
            'Choosing a billing provider did not set its UUID'
        )
        findButton('Close', providerSearch.closest('dialog')).click()
        await until(() => !select('dialog[open]'), 'The bill record editor did not close')

        select('nav.sidebar-navigation a[href$="/management/subscription-settings"]').click()
        const toggle = await until(
            () => select('input[role=switch][name=randomizeHosts]'),
            'Subscription settings did not load'
        )
        toggle.labels[0].click()
        await until(
            () => toggle.checked && toggle.getAttribute('aria-checked') === 'true',
            'The visible label did not activate the subscription switch'
        )
        const tabs = [...document.querySelectorAll('main [role=tab]')]
        tabs[0].focus()
        tabs[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))
        const panelFor = (tab) => document.getElementById(tab.getAttribute('aria-controls'))
        await until(
            () => document.activeElement === tabs[1] && !panelFor(tabs[1]).hidden,
            'ArrowRight did not activate the next subscription tab'
        )
        const remarks = panelFor(tabs[1])
        const firstCategory = remarks.querySelector('.remark-groups section')
        findButton('Add', firstCategory).click()
        await until(
            () => firstCategory.querySelectorAll('input[name=HWIDMaxDevicesExceeded]').length === 2,
            'Adding a remark did not extend its category'
        )
        tabs[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'End', bubbles: true }))
        const headers = panelFor(tabs[2])
        await until(
            () => document.activeElement === tabs[2] && !headers.hidden,
            'End did not activate the response headers tab'
        )
        findButton('Add header', headers).click()
        await until(
            () => headers.querySelector('input[name=headerName]'),
            'Adding a response header from the footer did not create a field'
        )
        tabs[0].click()
        await until(
            () => !panelFor(tabs[0]).hidden && remarks.hidden && headers.hidden && toggle.checked,
            'Switching tabs discarded the subscription draft'
        )

        select('nav.sidebar-navigation a[href$="/management/users"]').click()
        await until(
            () => select('.navigation-loading [role=status]'),
            'A slow route change did not show navigation progress'
        )
        const previousView = toggle.closest('.route-content')
        if (
            previousView?.isConnected &&
            (!previousView.inert || getComputedStyle(previousView).visibility !== 'hidden')
        ) {
            throw new Error('The previous page remained visible while Users was loading')
        }
        await until(
            () => select('.users-panel .table-loading .loading-state[role=status]'),
            'The users panel did not show its initial loading state'
        )
        await until(
            () => select('.users-panel .user-row'),
            'The users table did not replace the initial loading state'
        )
        const refreshUsers = findButton('Update', select('.users-panel .table-title'))
        refreshUsers.click()
        await until(
            () => refreshUsers.disabled && refreshUsers.getAttribute('aria-busy') === 'true',
            'Refreshing users did not show a busy button'
        )
        await until(() => !refreshUsers.disabled, 'The refresh button remained busy')
        const bulkEdit = await until(() => findButton('Bulk edit'), 'User bulk edit did not load')
        bulkEdit.click()
        const bulkDialog = await until(
            () => select('dialog.bulk-edit[open]'),
            'User bulk edit did not open'
        )
        if (
            bulkDialog.querySelector('h2')?.textContent !== 'Update users' ||
            bulkDialog.querySelectorAll('.form-card').length !== 4 ||
            !bulkDialog.querySelector('input[name=trafficLimitBytes]')
        ) {
            throw new Error('The user bulk edit shortcut did not open the edit form')
        }
        bulkDialog.querySelector('input[name=trafficLimitBytes]').value = '1.5'
        bulkDialog.querySelector('select[name=trafficUnit]').value = 'MiB'
        const clearEmail = findButton('Clear Email', bulkDialog)
        clearEmail.click()
        await until(
            () => clearEmail.getAttribute('aria-pressed') === 'true',
            'The edit form did not keep an explicit clear'
        )
        const confirm = window.confirm
        try {
            window.confirm = () => true
            findButton('Update', bulkDialog).click()
        } finally {
            window.confirm = confirm
        }
        await until(() => !bulkDialog.isConnected, 'A successful bulk edit did not close')

        const usernameMenu = await until(
            () => findButton('Column actions for Username'),
            'Users table did not render its column actions'
        )
        const usernameHeader = usernameMenu.closest('th')
        usernameMenu.click()
        const columnActions = await until(
            () => select('[role=menu][aria-label="Column actions for Username"]:popover-open'),
            'Username column actions did not open'
        )
        findButton('Sort by Username ascending', columnActions).click()
        await until(
            () => usernameHeader.getAttribute('aria-sort') === 'ascending',
            'Sorting from the column menu did not update the header'
        )

        const width = usernameHeader.getBoundingClientRect().width
        const resize = findButton('Resize Username', usernameHeader)
        resize.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))
        await until(
            () => usernameHeader.getBoundingClientRect().width === width + 16,
            'Resizing did not change the column width'
        )
        if (
            JSON.parse(localStorage.getItem('x-rmnw-users-table'))?.state?.columnSize?.username !==
            width + 16
        ) {
            throw new Error('The column width was not persisted')
        }

        const status = select('thead #user-filter-status')
        status.click()
        const statusOptions = await until(
            () => select('[role=listbox][aria-label="Filter Status"]:popover-open'),
            'Status options did not open above the scrollable table'
        )
        const statusOrder = Array.from(statusOptions.querySelectorAll('[role=option]'), (option) =>
            option.textContent.trim()
        )
        if (statusOrder.join(',') !== 'ACTIVE,DISABLED,EXPIRED,LIMITED') {
            throw new Error(`Unexpected status option order: ${statusOrder}`)
        }
        status.focus()
        const press = (element, key) =>
            element.dispatchEvent(
                new KeyboardEvent('keydown', { key, cancelable: true, bubbles: true })
            )
        press(status, 'ArrowDown')
        await until(() => {
            const active = document.getElementById(status.getAttribute('aria-activedescendant'))
            return document.activeElement === status && active?.textContent.trim() === 'ACTIVE'
        }, 'The status combobox did not retain input focus and highlight Active')
        press(status, 'Enter')
        await until(
            () => status.value === 'ACTIVE' && !statusOptions.isConnected,
            'Selecting a status did not apply and close the filter'
        )
        if (
            !JSON.parse(localStorage.getItem('x-rmnw-users-table'))?.state?.columnFilter?.some(
                ({ id, value }) => id === 'status' && value === 'ACTIVE'
            )
        ) {
            throw new Error('The status filter was not persisted')
        }

        const userPageSize = select('input[aria-label="Rows per page"]')
        userPageSize.focus()
        userPageSize.click()
        const sizes = await until(
            () => document.getElementById(userPageSize.getAttribute('aria-controls')),
            'Rows per page options did not open'
        )
        if (
            Array.from(sizes.querySelectorAll('[role=option]'), (option) =>
                option.textContent.trim()
            ).join(',') !== '5,10,15,20,25,30,50,100'
        ) {
            throw new Error('Rows per page did not expose the React page sizes')
        }
        press(userPageSize, 'ArrowDown')
        press(userPageSize, 'ArrowDown')
        press(userPageSize, 'Enter')
        await until(
            () => userPageSize.value === '50' && document.activeElement === userPageSize,
            'Keyboard pagination did not change 25 to 50 and retain focus'
        )
        userPageSize.click()
        await until(
            () => userPageSize.getAttribute('aria-expanded') === 'true',
            'Page size did not reopen'
        )
        press(userPageSize, 'ArrowUp')
        press(userPageSize, 'ArrowUp')
        press(userPageSize, 'Enter')
        await until(() => userPageSize.value === '25', 'Page size did not return to 25')

        const columns = findButton('Show/Hide columns')
        columns.click()
        const columnList = await until(
            () => select('[role=menu][aria-label="Show/Hide columns"]:popover-open'),
            'Column visibility controls did not open'
        )
        if (document.activeElement !== columnList)
            throw new Error('Column visibility did not focus its menu')
        findButton('Hide all', columnList).click()
        await until(
            () => !usernameHeader.isConnected && select('thead input[type=checkbox]'),
            'Hiding data columns removed the selection control'
        )
        findButton('Show all', columnList).click()
        await until(
            () => findButton('Column actions for Status'),
            'Show all did not restore Status'
        )
        columns.click()
        const restoredStatus = select('thead #user-filter-status')

        const templatesButton = findButton('Templates', select('.table-title'))
        const findTemplates = () => select('[role=menu][aria-label=Templates]:popover-open')
        templatesButton.click()
        const templateMenu = await until(findTemplates, 'Templates menu did not open')
        findButton('Save', templateMenu).click()
        const saveView = await until(
            () => select('dialog.save-user-view[open]'),
            'Save template did not open'
        )
        const templateName = saveView.querySelector('input')
        if (document.activeElement !== templateName || !findButton('Save', saveView).disabled) {
            throw new Error('The template dialog did not focus its required name')
        }
        templateName.value = '  Active users  '
        templateName.dispatchEvent(new Event('input', { bubbles: true }))
        findButton('teal', saveView).click()
        await until(() => !findButton('Save', saveView).disabled, 'Template name was not accepted')
        findButton('Save', saveView).click()
        await until(() => !saveView.isConnected, 'Saving a template did not dismiss the modal')
        const readTemplates = () =>
            JSON.parse(localStorage.getItem('x-rmnw-users-table-templates'))?.state?.templates
        const [savedView] = readTemplates() ?? []
        if (savedView?.name !== 'Active users' || savedView.color !== 'teal') {
            throw new Error('The template did not persist the trimmed name and selected color')
        }
        findButton('Clear filter status').click()
        await until(() => !restoredStatus.value, 'Status did not clear')
        templatesButton.click()
        const applyView = await until(
            () => findTemplates() && findButton('Active users', findTemplates()),
            'Saved template was missing from the menu'
        )
        applyView.click()
        await until(
            () => restoredStatus.value === 'ACTIVE' && !findTemplates(),
            'Applying the template did not restore its filter'
        )
        templatesButton.click()
        const deleteView = await until(
            () => findTemplates() && findButton('Delete Active users', findTemplates()),
            'Template deletion control did not appear'
        )
        deleteView.click()
        await until(() => readTemplates()?.length === 0, 'Deleting the template did not persist')
        templatesButton.click()

        const firstUser = select('tbody input[aria-label="Toggle select row"]')
        firstUser.click()
        const selected = await until(
            () => select('.selection-summary'),
            'Selecting a user did not show the selection toolbar'
        )
        findButton('Select all', selected).click()
        await until(
            () => selected.querySelector('[role=status]').textContent.trim() === 'Selected: 2',
            'Select all did not add the other row'
        )
        findButton('Clear selection', selected).click()
        await until(
            () => !selected.isConnected && !firstUser.checked,
            'Clear selection did not clear the rows'
        )
        firstUser.click()
        const actions = await until(
            () => findButton('Actions', select('.users-panel')),
            'Selected actions did not appear'
        )
        actions.click()
        const actionDialog = await until(
            () => select('dialog.bulk-users[open]'),
            'Selected bulk actions did not open'
        )
        findButton('Reset traffic', actionDialog).click()
        await until(
            () => findButton('Reset traffic', actionDialog).getAttribute('aria-pressed') === 'true',
            'Reset traffic skipped the confirmation state'
        )
        findButton('Extend expiration date', actionDialog).click()
        const extend = await until(
            () => select('dialog[open] input[name=extendDays]'),
            'Extend expiration did not ask for a day count'
        )
        extend.value = '3'
        findButton('Extend', extend.closest('dialog')).click()
        await until(
            () => !actionDialog.isConnected && !firstUser.checked,
            'A successful selected action did not clear the selection'
        )

        function openPanelRoute(path) {
            const link = document.createElement('a')
            link.href = scope + path
            document.body.append(link)
            link.click()
            link.remove()
        }
        openPanelRoute('dashboard/management/')
        await until(
            () => location.pathname === `${scope}dashboard/management/users`,
            'The management index did not redirect'
        )
        openPanelRoute('dashboard/')
        await until(() => select('.home .metrics'), 'Home statistics did not load')
        if (location.pathname !== `${scope}dashboard/home`)
            throw new Error('The dashboard index did not redirect')
        await until(
            () =>
                performance.getEntriesByType('resource').filter(({ name }) => {
                    const url = new URL(name)
                    return url.pathname === '/api/system/stats'
                }).length >= 2,
            'Home statistics were not refreshed'
        )
        await new Promise((resolve) => setTimeout(resolve, 100))
        if (!select('.home .metrics'))
            throw new Error('A failed background refresh hid the loaded home statistics')

        for (const [path, expected] of [
            ['dashboard/open/unknown/42', 'dashboard/home'],
            ['dashboard/open/node/invalid', 'dashboard/management/nodes'],
            ['dashboard/templates/mihomo', 'dashboard/home'],
            ['dashboard/templates/unknown', 'dashboard/home']
        ]) {
            openPanelRoute(path)
            await until(() => location.pathname === scope + expected, `Wrong redirect for ${path}`)
        }
        for (const entity of ['nodes', 'hosts']) {
            openPanelRoute(`dashboard/management/${entity}/new`)
            await until(
                () => select('main h1')?.textContent === 'Nothing to see here',
                `A standalone create route still exists for ${entity}`
            )
            findButton('Take me back to home page').click()
            await until(
                () => location.pathname === `${scope}dashboard/home`,
                'Could not return home'
            )
        }

        openPanelRoute('dashboard/templates/MIHOMO')
        const createTemplate = await until(
            () =>
                location.pathname === `${scope}dashboard/templates/MIHOMO` &&
                findButton('Create', select('main')),
            'The Mihomo template list did not load'
        )
        createTemplate.click()
        const newTemplateName = await until(
            () => select('dialog[open] input[placeholder="My Mihomo template"]'),
            'The template creation form did not open'
        )
        if (
            document.activeElement !== newTemplateName ||
            !newTemplateName.required ||
            newTemplateName.validity.valid
        )
            throw new Error('The template name was not focused and required')
        newTemplateName.value = 'New template'
        newTemplateName.dispatchEvent(new Event('input', { bubbles: true }))
        findButton('Create', newTemplateName.closest('dialog')).click()
        await until(
            () =>
                location.pathname === `${scope}dashboard/templates/MIHOMO/${mihomoUuid}` &&
                select('main h1')?.textContent === 'New template',
            'Creating a template did not open its editor'
        )
        for (const { type, uuid, syntaxError } of templateValidationCases) {
            const path = `dashboard/templates/${type}/${uuid}`
            openPanelRoute(path)
            await until(() => location.pathname === scope + path, `Did not open ${type} template`)
            await until(
                () => {
                    const diagnostics = document.querySelectorAll('.editor-diagnostics li')
                    return [...diagnostics].some((item) => {
                        if (syntaxError) return !item.classList.contains('warning')
                        const expected = type === 'SINGBOX' ? 'warn' : '65535'
                        return (
                            /^\d+:\d+ /.test(item.textContent.trim()) &&
                            item.textContent.includes(expected)
                        )
                    })
                },
                `The ${type} editor did not show the ${syntaxError ? 'syntax' : 'schema'} diagnostic`
            )
        }

        openPanelRoute('dashboard/management/internal-squads')
        await until(
            () => location.pathname === `${scope}dashboard/management/internal-squads`,
            'Internal squads route did not open'
        )
        const createInternal = await until(
            () => findButton('Create'),
            'Internal squads did not load'
        )
        createInternal.click()
        const internalName = await until(
            () => select('dialog[open] input[name=name]'),
            'The internal squad creation form did not open'
        )
        if (!findButton('Create', internalName.closest('dialog')).disabled)
            throw new Error('An empty internal squad name was accepted')
        internalName.value = 'Invalid!'
        internalName.dispatchEvent(new Event('input', { bubbles: true }))
        await until(
            () => findButton('Create', internalName.closest('dialog')).disabled,
            'An invalid internal squad name was accepted'
        )
        internalName.value = 'New internal squad'
        internalName.dispatchEvent(new Event('input', { bubbles: true }))
        await until(
            () => !findButton('Create', internalName.closest('dialog')).disabled,
            'Internal squad name was rejected'
        )
        findButton('Create', internalName.closest('dialog')).click()
        const createdInternal = await until(
            () => select('dialog.squad-inbounds[open]'),
            'Creating an internal squad did not open inbound selection'
        )
        findButton('Close', createdInternal).click()
        await until(() => !createdInternal.isConnected, 'Internal squad editor did not close')

        openPanelRoute('dashboard/management/external-squads')
        await until(
            () =>
                location.pathname === `${scope}dashboard/management/external-squads` &&
                select('main h1')?.textContent === 'External Squads',
            'External squads route did not open'
        )
        const createExternal = await until(
            () => findButton('Create'),
            'External squads did not load'
        )
        createExternal.click()
        const externalName = await until(
            () => select('dialog[open] input[name=name]'),
            'The external squad creation form did not open'
        )
        externalName.value = 'New external squad'
        externalName.dispatchEvent(new Event('input', { bubbles: true }))
        await until(
            () => !findButton('Create', externalName.closest('dialog')).disabled,
            'External squad name was rejected'
        )
        findButton('Create', externalName.closest('dialog')).click()
        await until(
            () =>
                location.pathname === `${scope}dashboard/open/external-squad/${externalUuid}` &&
                select('dialog.external-squad-drawer[open]'),
            'Creating an external squad did not open its editor'
        )

        openPanelRoute('dashboard/management/hosts')
        const createHost = await until(
            () => select('main.hosts-page') && findButton('Create', select('main.hosts-page')),
            'Hosts did not load'
        )
        const [firstHost, secondHost] = await until(() => {
            const cards = document.querySelectorAll('.host-card')
            return cards.length === 2 && cards
        }, 'Host cards did not load')
        const originalTop = firstHost.getBoundingClientRect().top
        firstHost.querySelector('input[type=checkbox]').click()
        const selectionPanel = await until(
            () => select('.selection-panel'),
            'Selecting a host did not open the floating actions'
        )
        if (
            getComputedStyle(selectionPanel).position !== 'fixed' ||
            firstHost.getBoundingClientRect().top !== originalTop
        ) {
            throw new Error('Selecting a host moved the list')
        }
        findButton('Select all', selectionPanel).click()
        await until(
            () => secondHost.querySelector('input[type=checkbox]').checked,
            'Select all did not include the second host'
        )
        findButton('Clear selection', selectionPanel).click()
        await until(() => !selectionPanel.isConnected, 'Clearing selection did not close actions')

        const grip = firstHost.querySelector('.grip')
        const handle = grip.getBoundingClientRect()
        const firstBox = firstHost.getBoundingClientRect()
        const secondBox = secondHost.getBoundingClientRect()
        const fromY = handle.top + handle.height / 2
        const toY = fromY + secondBox.top - firstBox.top
        let captured = false
        grip.setPointerCapture = () => {
            captured = true
        }
        const pointer = (type, clientY) =>
            new PointerEvent(type, {
                bubbles: true,
                pointerId: 1,
                isPrimary: true,
                clientX: handle.left + handle.width / 2,
                clientY
            })
        grip.dispatchEvent(pointer('pointerdown', fromY))
        if (!captured) throw new Error('The host grip did not capture the pointer')
        grip.dispatchEvent(pointer('pointermove', toY))
        await until(
            () =>
                firstHost.classList.contains('dragging') &&
                secondHost.style.getPropertyValue('--host-offset') === '0px',
            'The cards did not rearrange during the drag'
        )
        window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
        await until(
            () =>
                !firstHost.classList.contains('dragging') &&
                firstHost.style.getPropertyValue('--host-offset') === '0px',
            'Cancelling the drag did not restore the original order'
        )
        grip.dispatchEvent(pointer('pointerdown', fromY))
        grip.dispatchEvent(pointer('pointermove', toY))
        grip.dispatchEvent(pointer('pointerup', toY))
        await until(
            () => select('.host-card') === secondHost && secondHost.querySelector('.grip:enabled'),
            'Dropping onto the second host card did not persist the order'
        )

        const hostView = findButton('Toggle view mode', select('main.hosts-page'))
        hostView.click()
        const hostHeaders = await until(() => {
            const headers = document.querySelectorAll('.hosts-page thead th[aria-sort] button')
            return headers.length > 0 && headers
        }, 'Sortable host columns did not load')
        for (const button of hostHeaders) {
            const style = getComputedStyle(button)
            if (style.backgroundColor !== 'rgba(0, 0, 0, 0)' || style.paddingInlineStart !== '0px')
                throw new Error('A host column header used the filled button style')
        }
        const [sortHost] = hostHeaders
        for (const direction of ['ascending', 'descending', 'none']) {
            sortHost.click()
            await until(
                () => sortHost.closest('th').getAttribute('aria-sort') === direction,
                `Host column did not cycle to ${direction}`
            )
        }
        hostView.click()
        await until(() => select('.host-card'), 'Host cards did not return')

        createHost.click()
        const hostEditor = await until(() => select('dialog[open]'), 'Host editor did not open')
        if (hostEditor.querySelector('.option-group:not([hidden])')) {
            throw new Error('A blank host editor displayed empty option groups')
        }
        const addOption = findButton('Add', hostEditor)
        addOption.click()
        const hostSearch = await until(
            () => hostEditor.querySelector('.option-picker [role=combobox]'),
            'Host option search did not open'
        )
        if (document.activeElement !== hostSearch)
            throw new Error('Host option search was not focused')
        hostSearch.value = 'Security Layer'
        hostSearch.dispatchEvent(new Event('input', { bubbles: true }))
        await until(
            () =>
                hostEditor
                    .querySelector('.option-picker [role=option]')
                    ?.textContent.includes('Security layer'),
            'Host option search did not filter the options'
        )
        hostSearch.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
        await until(
            () => hostSearch.getAttribute('aria-activedescendant'),
            'No option was highlighted'
        )
        hostSearch.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
        await until(() => !hostSearch.isConnected, 'Enter did not select the highlighted option')
        const visibleGroups = [...hostEditor.querySelectorAll('.option-group')].filter(
            (group) => getComputedStyle(group).display !== 'none'
        )
        if (
            visibleGroups.length !== 1 ||
            visibleGroups[0].querySelector('.option-label')?.textContent !== 'Security'
        ) {
            throw new Error('Adding an option did not reveal only its group')
        }
        if (document.activeElement !== addOption) throw new Error('Focus did not return to Add')

        async function addHostOption(label) {
            addOption.click()
            const search = await until(
                () => hostEditor.querySelector('.option-picker [role=combobox]'),
                'Host option search did not open'
            )
            setValue(search, label)
            const option = await until(
                () =>
                    [...hostEditor.querySelectorAll('.option-picker [role=option]')].find(
                        (item) => item.textContent.trim() === label
                    ),
                `Host option ${label} was not available`
            )
            option.click()
            await until(() => !search.isConnected, `Host option ${label} was not added`)
        }

        await addHostOption('Mihomo IP version')
        if (hostEditor.querySelector('select, datalist'))
            throw new Error('The host form still contains a browser-native picker')
        const ipValue = hostEditor.querySelector('input[type=hidden][name=mihomoIpVersion]')
        const ipControl = ipValue.parentElement.querySelector('[role=combobox]')
        ipControl.focus()
        ipControl.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }))
        const ipv6 = await until(
            () =>
                [...hostEditor.querySelectorAll('[popover]:popover-open [role=option]')].find(
                    (item) => item.textContent.trim() === 'ipv6'
                ),
            'Mihomo IP version did not open a panel list'
        )
        ipv6.click()
        await until(() => ipValue.value === 'ipv6', 'The chosen IP version did not reach the form')
        findButton('Clear', ipValue.parentElement).click()
        await until(() => ipValue.value === '', 'Clearing the IP version did not reach the form')

        await addHostOption('Tags')
        const tagControl = hostEditor.querySelector('.tags-control [role=combobox]')
        tagControl.focus()
        const fixtureTag = await until(
            () =>
                [...hostEditor.querySelectorAll('[popover]:popover-open [role=option]')].find(
                    (item) => item.textContent.trim() === 'FIXTURE'
                ),
            'Host tag suggestions did not open a panel list'
        )
        fixtureTag.click()
        await until(
            () => hostEditor.querySelector('input[type=hidden][name=tags]').value === 'FIXTURE',
            'Selecting a suggested tag did not update the host form'
        )

        const openPreview = (editor) => {
            findButton('Preview', editor).click()
            return until(() => {
                const preview = [...document.querySelectorAll('dialog[open]')].at(-1)
                return preview !== editor && preview
            }, 'Host preview did not open')
        }
        await until(
            () => hostEditor.querySelector('.inbound-select:enabled'),
            'Host inbound selector did not load'
        ).then((button) => button.click())
        const picker = await until(
            () => select('dialog[open] .profile-toggle')?.closest('dialog'),
            'Host inbound picker did not open'
        )
        picker.querySelector('.profile-toggle').click()
        await until(() => picker.querySelector('li input'), 'No inbound was available').then(
            (input) => input.click()
        )
        findButton('Save', picker).click()
        await until(() => !picker.isConnected, 'Host inbound selection did not close')
        setValue(hostEditor.querySelector('[name=remark]'), 'Draft preview host')
        setValue(hostEditor.querySelector('[name=address]'), 'draft.example.test')
        setValue(hostEditor.querySelector('[name=port]'), '8443')
        const draftPreview = await openPreview(hostEditor)
        const draftHeaders = draftPreview.querySelector('textarea:not([readonly])')
        setValue(draftHeaders, 'invalid header')
        findButton('Generate preview', draftPreview).click()
        await until(
            () => draftPreview.querySelector('[role=alert]')?.textContent.includes('Name: value'),
            'Malformed request headers did not show a validation error'
        )
        setValue(draftHeaders, 'X-Preview-Mode: draft:config')
        draftPreview.querySelector('input[type=checkbox]').click()
        findButton('Generate preview', draftPreview).click()
        await until(
            () =>
                draftPreview.querySelector('.body pre')?.textContent === 'draft.example.test:8443',
            'Preview did not send the draft host, selected inbound, and custom header'
        )
        findButton('Close', draftPreview).click()
        await until(
            () => !draftPreview.isConnected && hostEditor.open,
            'Closing the preview did not return to the draft'
        )
        findButton('Close', hostEditor).click()
        expectDialogExit(hostEditor, 'slide')
        await until(() => !hostEditor.isConnected, 'The host editor did not close')

        openPanelRoute(`dashboard/open/host/${hostUuid}`)
        const savedHost = await until(
            () => select('.host-details[open] input[name=address]')?.closest('dialog'),
            'Saved host editor did not open'
        )
        savedHost.querySelector('[aria-label="Delete"]').click()
        const deleteConfirmation = await until(
            () =>
                [...document.querySelectorAll('dialog[open]')].find(
                    (dialog) =>
                        dialog !== savedHost && dialog.querySelector('h2')?.textContent === 'Delete'
                ),
            'Deleting a host did not open the panel confirmation'
        )
        findButton('Cancel', deleteConfirmation).click()
        await until(
            () => !deleteConfirmation.isConnected && savedHost.open,
            'Cancelling deletion did not return to the host editor'
        )
        setValue(savedHost.querySelector('input[name=address]'), 'unsaved.example.test')
        const savedPreview = await openPreview(savedHost)
        setValue(savedPreview.querySelector('input:not([type=checkbox])'), 'v2rayN/7.0.0')
        setValue(savedPreview.querySelector('textarea:not([readonly])'), 'X-Preview-Mode: saved')
        savedPreview.querySelector('input[type=checkbox]').click()
        findButton('Generate preview', savedPreview).click()
        const previewStatus = await until(() => {
            const error = savedPreview.querySelector('[role=alert]')
            if (error) throw new Error(`Saved host preview failed: ${error.textContent}`)
            return savedPreview.querySelector('[role=status]')
        }, 'Saved host preview did not return a response')
        if (
            !/HTTP 200\s*·\s*XRAY_JSON/.test(previewStatus.textContent) ||
            !previewStatus.textContent.includes('Matched rule: Saved host rule') ||
            !savedPreview.querySelector('.headers pre')?.textContent.includes('X-Preview: saved') ||
            savedPreview.querySelector('.body pre')?.textContent !==
                '{\n  "address": "saved.example.test:443"\n}'
        ) {
            throw new Error(`Unexpected saved host preview: ${previewStatus.textContent}`)
        }
        setValue(savedPreview.querySelector('textarea:not([readonly])'), 'X-Preview-Mode: drop')
        findButton('Generate preview', savedPreview).click()
        await until(
            () => savedPreview.textContent.includes('Connection closed without a response'),
            'Socket drop did not clear the response and show its status'
        )
        if (savedPreview.querySelector('.headers, .body') || findButton('Copy', savedPreview))
            throw new Error('Socket drop displayed an empty response section')
        findButton('Close', savedPreview).click()
        await until(() => !savedPreview.isConnected, 'Saved preview did not close')
        const sni = savedHost.querySelector('input[name=sni]')
        if (sni?.value !== 'saved-sni.example.test') throw new Error('Missing saved SNI')
        findButton('Remove', sni.closest('.optional-field')).click()
        await until(() => !sni.isConnected, 'The SNI reset did not remove its option')
        findButton('Add', savedHost).click()
        const sniOption = await until(
            () =>
                [...savedHost.querySelectorAll('.option-picker [role=option]')].find(
                    (option) => option.textContent.trim() === 'SNI'
                ),
            'The removed SNI option did not return to Add'
        )
        sniOption.click()
        const emptySni = await until(
            () => savedHost.querySelector('input[name=sni]'),
            'SNI could not be added again'
        )
        if (emptySni.value) throw new Error('Re-adding SNI restored its removed value')
        findButton('Remove', emptySni.closest('.optional-field')).click()
        findButton('Save', savedHost).click()
        await until(() => !savedHost.isConnected, 'Saving the SNI reset did not close the editor')
        openPanelRoute(`dashboard/open/host/${hostUuid}`)
        const reopenedHost = await until(
            () => select('.host-details[open] input[name=address]')?.closest('dialog'),
            'The host did not reopen after the reset'
        )
        if (reopenedHost.querySelector('input[name=sni]')) throw new Error('SNI was not cleared')
        reopenedHost.dispatchEvent(new Event('cancel', { cancelable: true }))
        expectDialogExit(reopenedHost, 'slide')
        await until(() => !reopenedHost.isConnected, 'The host editor did not close on Escape')

        openPanelRoute(`dashboard/open/internal-squad/${squadUuid}`)
        const directSquad = await until(
            () => select('dialog.squad-inbounds[open]'),
            'The generic entity route shadowed the internal squad drawer'
        )
        findButton('Close', directSquad).click()
        await until(() => !directSquad.isConnected, 'The internal squad drawer did not close')

        openPanelRoute('dashboard/subpage/00000000-0000-0000-0000-000000000000')
        const ios = await until(
            () =>
                [...document.querySelectorAll('.platform-card')].find(
                    (card) => card.querySelector('summary strong')?.textContent === 'iOS'
                ),
            'The subpage editor did not mount its iOS platform'
        )
        const platformHeader = ios.querySelector('summary')
        platformHeader.click()
        const iconPicker = await until(
            () => ios.querySelector('button.choose'),
            'iOS did not expand'
        )
        iconPicker.click()
        const icon = await until(
            () => findButton('TV'),
            'The SVG picker did not show the icon library'
        )
        icon.click()
        await until(
            () => iconPicker.textContent.trim() === 'TV',
            'The platform icon did not update'
        )
        ios.querySelector('button.localized').click()
        const translation = await until(
            () => select('dialog[open] .translations input'),
            'The localized platform name did not open its editor'
        )
        translation.value = 'Edited iOS'
        translation.dispatchEvent(new Event('input', { bubbles: true }))
        findButton('Close', translation.closest('dialog')).click()
        platformHeader.click()
        await until(() => !iconPicker.isConnected, 'iOS did not collapse')
        platformHeader.click()
        await until(
            () => ios.querySelector('button.localized')?.textContent.includes('Edited iOS'),
            'Collapsing the platform discarded its translation draft'
        )

        findButton('Happ', ios).click()
        const appEditor = await until(
            () => select('dialog.app-editor-overlay[open]'),
            'App editor did not open'
        )
        const blockRows = appEditor.querySelectorAll('.block-row')
        blockRows[0].querySelector('.block-edit').click()
        const blockEditor = await until(
            () => select('dialog.modal[open]'),
            'Clicking a block did not open its editor'
        )
        const buttonCount = blockEditor.querySelectorAll('.button-card').length
        findButton('Add button', blockEditor).click()
        const addedButton = await until(
            () => blockEditor.querySelectorAll('.button-card')[buttonCount],
            'Adding a button did not create a collapsed row'
        )
        const buttonDetails = addedButton.querySelector('details')
        if (buttonDetails.open) throw new Error('The new button opened automatically')
        buttonDetails.querySelector('summary').click()
        findButton('Template variables', addedButton).click()
        const variableHelp = await until(
            () =>
                [...document.querySelectorAll('dialog[open]')].find(
                    (dialog) => dialog.querySelector('h2')?.textContent === 'Template variables'
                ),
            'The button link did not open the template variables'
        )
        if (
            ![...variableHelp.querySelectorAll('code')].some(
                (item) => item.textContent === '{{HAPP_CRYPT3_LINK}}'
            )
        )
            throw new Error('The link helper omitted the subscription page variables')
        findButton('Close', variableHelp).click()
        await until(() => !variableHelp.isConnected, 'Template variables did not close')
        const link = addedButton.querySelector('input[placeholder="https:// or app://"]')
        link.value = 'https://example.test'
        link.dispatchEvent(new Event('input', { bubbles: true }))
        buttonDetails.querySelector('summary').click()
        buttonDetails.querySelector('summary').click()
        if (link.value !== 'https://example.test')
            throw new Error('Collapsing discarded the button link')
        findButton('Remove', addedButton).click()
        await until(() => !addedButton.isConnected, 'Removing a button left it in the editor')
        findButton('Close', blockEditor).click()
        await until(() => !blockEditor.isConnected, 'The block editor did not close')
        if (!appEditor.open) throw new Error('Closing a block closed the app editor')
        blockRows[0].querySelector('[aria-label="Move block down"]').click()
        await until(
            () => appEditor.querySelector('.block-row') === blockRows[1],
            'Moving a block did not update the app editor order'
        )
        findButton('Close', appEditor).click()
        await until(() => !appEditor.isConnected, 'The app editor did not close')

        openPanelRoute('dashboard/subpage')
        const leaveSubpage = await until(
            () => findButton('Leave'),
            'The unsaved draft was not guarded'
        )
        leaveSubpage.click()
        const createSubpage = await until(() => findButton('Create'), 'Subpages did not load')
        createSubpage.click()
        const subpageName = await until(
            () => select('dialog[open] input[placeholder="My Subscription Page Config"]'),
            'The create form did not open'
        )
        if (document.activeElement !== subpageName) throw new Error('The name was not focused')
        if (!subpageName.required || subpageName.validity.valid)
            throw new Error('The subpage name was not required')
        subpageName.value = 'Smoke subscription'
        subpageName.dispatchEvent(new Event('input', { bubbles: true }))
        findButton('Create', subpageName.closest('dialog')).click()
        await until(
            () =>
                location.pathname ===
                    `${scope}dashboard/subpage/41414141-4141-4141-8141-414141414141` &&
                document.title.includes('Smoke subscription') &&
                !select('dialog[open]'),
            'Creating a subpage did not open the new editor'
        )

        openPanelRoute('dashboard/tools/sessions-explorer')
        const startScan = await until(() => {
            const button = findButton('Start exploring')
            return button && !button.disabled && button
        }, 'The Sessions Explorer did not find the online nodes')
        startScan.click()
        await until(
            () =>
                select('main [role="progressbar"]')?.getAttribute('aria-valuenow') === '0' &&
                select('main li')?.closest('section')?.textContent.includes('GeoCheck node'),
            'The scan did not show progress and active nodes'
        )
        const animationStage = select('main .scan .animation')
        if (animationStage.querySelector('svg') || !animationStage.querySelector('span')) {
            throw new Error('The scan displayed the legacy globe while its animation was loading')
        }
        await until(
            () =>
                animationStage.querySelector('canvas.ready') &&
                !animationStage.querySelector('span') &&
                performance
                    .getEntriesByType('resource')
                    .some(({ name }) =>
                        /\/dashboard\/lotties\/(duck-progress|globe|jail)\.lottie$/.test(name)
                    ),
            'The scan did not load a bundled progress animation under the panel base path'
        )
        const searchIp = await until(
            () => select('main input[type="search"]'),
            'The scan did not display results and IP search'
        )
        const explorer = searchIp.closest('main')
        if (
            !findButton('Restart scan', explorer.querySelector('header')) ||
            !findButton('Clear results', explorer.querySelector('header')) ||
            !explorer.querySelector('a[aria-label="IP Info: 192.0.2.11"]') ||
            !findButton('Copy 192.0.2.11', explorer)
        )
            throw new Error('The session results omitted header actions or IP actions')
        const resultHeader = explorer.querySelector('article header')
        const copyWrapper = explorer.querySelector('.ip-copy-wrap .input-wrapper')
        if (
            getComputedStyle(resultHeader).borderBlockEndWidth !== '0px' ||
            getComputedStyle(copyWrapper, '::after').content !== 'none'
        ) {
            throw new Error('The session cards still have the dividers or IP field overlay')
        }
        setValue(searchIp, '192.0.2.12')
        await until(
            () =>
                findButton('Copy 192.0.2.12', explorer) && !findButton('Copy 192.0.2.11', explorer),
            'IP search did not narrow the user cards'
        )
        findButton('Clear results', explorer).click()
        await until(() => findButton('Start exploring'), 'Clearing results did not reset the scan')

        findButton('Sign out', select('header')).click()
        await until(
            () => location.pathname === `${scope}auth/login`,
            'Sign out lost the panel base path'
        )
        if (JSON.parse(localStorage.getItem('sessionStore'))?.state?.token !== '') {
            throw new Error('Sign out did not clear the session')
        }
        await until(
            () => select('main input[name=username]') && select('main input[name=password]'),
            'Login did not display credential fields'
        )

        const username = select('main input[name=username]')
        const password = select('main input[name=password]')
        username.value = 'admin'
        password.value = 'wrong-password'
        for (const input of [username, password]) {
            input.dispatchEvent(new Event('input', { bubbles: true }))
        }
        findButton('Sign in', select('main')).click()
        await until(
            () => select('[role=alert] strong')?.textContent === 'Login',
            'Login failure did not raise a notification'
        )
        openPanelRoute('auth/')
        await until(
            () => location.pathname === `${scope}auth/login`,
            'The auth index did not redirect'
        )
        for (const [query, message] of [
            ['', 'The sign-in response is incomplete'],
            ['?error=access_denied&error_description=Sign-in+cancelled', 'Sign-in cancelled']
        ]) {
            openPanelRoute(`oauth2/callback/github${query}`)
            await until(
                () =>
                    location.pathname === `${scope}auth/login` &&
                    [...document.querySelectorAll('[role=alert]')].some((alert) =>
                        alert.textContent.includes(message)
                    ),
                'The failed OAuth callback did not return to login and explain the failure'
            )
        }
        openPanelRoute('oauth2/callback/generic?code=00123&state=00042')
        await until(
            () =>
                [...document.querySelectorAll('[role=alert]')].some((alert) =>
                    alert.textContent.includes('Opaque callback verified')
                ),
            'The callback did not submit the original code and state'
        )

        document.body.dataset.result = 'passed'
    } catch (error) {
        document.body.dataset.result = 'failed'
        const message = document.createElement('pre')
        const detail = error instanceof Error ? (error.stack ?? error.message) : String(error)
        const controls = [...document.querySelectorAll('main button')].map(
            (button) => `${button.textContent.trim()}${button.disabled ? ' (disabled)' : ''}`
        )
        message.textContent = `${detail} | ${controls.join(', ')} | ${select('main [role=alert]')?.textContent ?? ''}`
        document.body.replaceChildren(message)
    }
}

export function startSvelteFixture(output) {
    const defaultSubpageConfig = JSON.parse(
        readFileSync(
            new URL('../../backend/internal/database/default_subpage_config.json', import.meta.url),
            'utf8'
        )
    )
    const defaultSubpage = {
        uuid: '00000000-0000-0000-0000-000000000000',
        name: 'Default',
        viewPosition: 0,
        tags: [],
        config: defaultSubpageConfig
    }
    let createdSubpage
    let createdTemplate
    const entry = readFileSync(join(output, 'index.html'), 'utf8')
        .replace(/<base[^>]*data-gowave-base[^>]*>/, `<base href="${panel}" data-gowave-base>`)
        .replace(
            '</head>',
            `<script>if (!sessionStorage.getItem('fixture-seeded')) {
                sessionStorage.setItem('fixture-seeded', '1');
                localStorage.setItem('i18nextLng', 'en');
                localStorage.setItem('sessionStore', ${JSON.stringify(JSON.stringify({ version: 0, state: { token } }))});
                localStorage.setItem('viewPreferencesStore', JSON.stringify({
                    version: 2,
                    state: {
                        experimental: { sshTerminal: true, nodeIntegrations: true, quickLauncher: true },
                        quickLinks: [],
                        launcherColumns: 3,
                        launcherPosition: null
                    }
                }));
            }</script></head>`
        )
        .replace('</body>', `<script type="module" src="${panel}__verify.js"></script></body>`)
    let membersCount = 2
    let nextBillingDate = '2025-05-02T00:00:00Z'
    let tags = []
    const userDate = '2026-05-01T11:00:00Z'
    const users = [42, 43].map((id) => ({
        id,
        shortUuid: `user-${id}`,
        username: `fixture-${id}`,
        status: 'ACTIVE',
        trafficLimitBytes: id === 43 ? 1024 : 0,
        trafficLimitStrategy: 'NO_RESET',
        expireAt: userDate,
        telegramId: null,
        email: null,
        description: id === 43 ? 'Original description' : null,
        tag: id === 43 ? 'ORIGINAL' : null,
        hwidDeviceLimit: null,
        externalSquadUuid: null,
        trojanPassword: 'fixture',
        vlessUuid: `44444444-4444-4444-8444-4444444444${id}`,
        ssPassword: 'fixture',
        lastTriggeredThreshold: 0,
        subRevokedAt: null,
        lastTrafficResetAt: null,
        createdAt: userDate,
        updatedAt: userDate,
        subscriptionUrl: `https://example.test/api/sub/user-${id}`,
        activeInternalSquads: [],
        userTraffic: {
            usedTrafficBytes: 0,
            lifetimeUsedTrafficBytes: 0,
            onlineAt: null,
            firstConnectedAt: null,
            lastConnectedNodeUuid: null
        }
    }))
    const profiles = ['First profile', 'Second profile'].map((name, viewPosition) => ({
        uuid: `44444444-4444-4444-8444-44444444444${viewPosition}`,
        name,
        viewPosition,
        tags: [],
        config: {},
        inbounds: [],
        nodes: [],
        createdAt: '2025-05-01T11:00:00Z',
        updatedAt: '2025-05-01T11:00:00Z'
    }))
    const snippetName = 'Demo snippet'
    const fragments = [{ name: snippetName, snippet: [{ log: { loglevel: 'debug' } }] }]
    const profileList = () => result({ total: profiles.length, configProfiles: profiles })
    const geoNode = {
        uuid: '55555555-5555-4555-8555-555555555555',
        id: 1,
        name: 'GeoCheck node',
        address: '192.0.2.10',
        port: 2222,
        proxyUrl: null,
        isConnected: true,
        isDisabled: false,
        isConnecting: false,
        lastStatusChange: null,
        lastStatusMessage: null,
        isTrafficTrackingActive: false,
        trafficResetDay: null,
        trafficLimitBytes: null,
        trafficUsedBytes: null,
        notifyPercent: null,
        viewPosition: 0,
        countryCode: 'US',
        consumptionMultiplier: 1,
        nodeConsumptionMultiplier: 1,
        tags: [],
        integrationUuids: [],
        ips: [],
        createdAt: '2025-05-01T11:00:00Z',
        updatedAt: '2025-05-01T11:00:00Z',
        configProfile: { activeConfigProfileUuid: null, activeInbounds: [] },
        providerUuid: null,
        provider: null,
        activePluginUuid: null,
        system: null,
        versions: { node: '1.0.0 (Go)', xray: '26.0.0', coreSource: 'local' },
        xrayUptime: 0,
        usersOnline: 0,
        note: null
    }
    const panelNode = {
        ...geoNode,
        uuid: '55555555-5555-4555-8555-555555555556',
        id: 2,
        name: 'Panel core node',
        viewPosition: 1,
        versions: { ...geoNode.versions, coreSource: 'panel' }
    }
    const legacyNode = {
        ...geoNode,
        uuid: '55555555-5555-4555-8555-555555555557',
        id: 3,
        name: 'Legacy node',
        viewPosition: 2,
        versions: { node: '1.0.0 (Go)', xray: '26.0.0' }
    }
    const nodes = [geoNode, panelNode, legacyNode]
    const setupNode = {
        ...geoNode,
        uuid: '55555555-5555-4555-8555-555555555558',
        id: 4,
        name: 'Animation test node',
        address: '192.0.2.40',
        isConnected: false,
        isConnecting: true
    }
    let setupPolls = 0
    const inbound = {
        uuid: '66666666-6666-4666-8666-666666666666',
        profileUuid: profiles[1].uuid,
        tag: 'fixture-inbound',
        type: 'vless',
        network: 'tcp',
        security: null,
        port: 443,
        rawInbound: { tag: 'fixture-inbound', protocol: 'vless' },
        activeSquads: [squadUuid]
    }
    profiles[1].inbounds = [inbound]
    const extraInbound = {
        ...inbound,
        uuid: '66666666-6666-4666-8666-666666666667',
        profileUuid: profiles[0].uuid,
        tag: 'extra-inbound',
        rawInbound: { tag: 'extra-inbound', protocol: 'vless' },
        activeSquads: []
    }
    profiles[0].inbounds = [extraInbound]
    const savedHost = {
        uuid: hostUuid,
        remark: 'Saved preview host',
        address: 'saved.example.test',
        port: 443,
        viewPosition: 0,
        isDisabled: true,
        path: null,
        sni: 'saved-sni.example.test',
        host: null,
        alpn: null,
        fingerprint: null,
        xhttpExtraParams: null,
        muxParams: null,
        sockoptParams: null,
        finalMask: null,
        inbound: { configProfileUuid: inbound.profileUuid, configProfileInboundUuid: inbound.uuid },
        serverDescription: null,
        vlessRouteId: null,
        pinnedPeerCertSha256: null,
        verifyPeerCertByName: null,
        shuffleHost: false,
        mihomoX25519: false,
        mihomoIpVersion: null,
        nodes: [],
        xrayJsonTemplateUuid: null,
        excludeFromSubscriptionTypes: [],
        mapper: {},
        includeInSrrRules: [],
        tags: ['FIXTURE'],
        internalSquads: { mode: 'EXCLUDE', squads: [] }
    }
    const secondHost = {
        ...savedHost,
        uuid: '55555555-5555-4555-8555-555555555555',
        remark: 'Second host',
        viewPosition: 1,
        tags: []
    }
    let hosts = [savedHost, secondHost]
    let squadInbounds = [inbound]
    const savedSquad = () => ({
        uuid: squadUuid,
        viewPosition: 0,
        name: 'Demo squad',
        tags,
        info: { membersCount, inboundsCount: squadInbounds.length },
        inbounds: squadInbounds,
        createdAt: userDate,
        updatedAt: userDate
    })
    profiles[1].nodes = [
        { uuid: geoNode.uuid, name: geoNode.name, countryCode: geoNode.countryCode }
    ]
    let externalSquad = {
        uuid: externalUuid,
        name: 'Fixture external squad',
        viewPosition: 0,
        tags: [],
        info: { membersCount: 2 },
        templates: [{ templateType: 'XRAY_BASE64', templateUuid: base64Uuid }],
        subscriptionSettings: null,
        hostOverrides: null,
        responseHeadersAdd: {},
        responseHeadersRemove: [],
        hwidSettings: null,
        customRemarks: null,
        subpageConfigUuid: null,
        createdAt: userDate,
        updatedAt: userDate
    }
    let geoPolls = 0
    let homeStatsRequests = 0
    let savedUserEdits = 0
    let userListRequests = 0
    const server = Bun.serve({
        hostname: '127.0.0.1',
        port: 0,
        async fetch(request) {
            const { pathname, searchParams } = new URL(request.url)
            if (pathname === '/api/auth/login' && request.method === 'POST')
                return Response.json({ message: 'Forbidden' }, { status: 403 })
            if (pathname === '/api/auth/oauth2/callback' && request.method === 'POST') {
                const { provider, code, state } = await request.json()
                const valid = provider === 'generic' && code === '00123' && state === '00042'
                return Response.json(
                    { message: valid ? 'Opaque callback verified' : 'Incorrect callback values' },
                    { status: 403 }
                )
            }
            if (pathname === '/api/auth/status') {
                return result({
                    isLoginAllowed: true,
                    isRegisterAllowed: false,
                    authentication: {
                        password: { enabled: true },
                        passkey: { enabled: false },
                        oauth2: {
                            providers: {
                                github: false,
                                pocketid: false,
                                yandex: false,
                                keycloak: false,
                                generic: false,
                                telegram: false
                            }
                        }
                    },
                    branding: { title: null, logoUrl: null }
                })
            }
            if (pathname.startsWith('/api/')) {
                if (request.headers.get('Authorization') !== `Bearer ${token}`)
                    return new Response(null, { status: 401 })
                if (pathname.replace(/\/$/, '') === '/api/hosts' && request.method === 'PATCH') {
                    const patch = await request.json()
                    if (
                        patch.uuid !== hostUuid ||
                        patch.sni !== null ||
                        patch.address !== 'unsaved.example.test'
                    )
                        return Response.json({ message: 'Invalid host reset' }, { status: 400 })
                    return result(Object.assign(savedHost, patch))
                }
                if (pathname === '/api/hosts/' && request.method === 'GET') return result(hosts)
                if (pathname === '/api/hosts/actions/reorder' && request.method === 'POST') {
                    const { hosts: order } = await request.json()
                    if (
                        order.length !== 2 ||
                        order[0].uuid !== secondHost.uuid ||
                        order[0].viewPosition !== 1 ||
                        order[1].uuid !== hostUuid ||
                        order[1].viewPosition !== 2
                    )
                        return new Response(null, { status: 400 })
                    hosts = [secondHost, savedHost]
                    return result({ isUpdated: true })
                }
                if (pathname === `/api/hosts/${hostUuid}`) return result(savedHost)
                if (pathname === '/api/hosts/tags') return result({ tags: ['FIXTURE'] })
                if (pathname === '/api/hosts/preview' && request.method === 'POST') {
                    const {
                        host,
                        hostUuid: savedUuid,
                        userAgent,
                        requestHeaders,
                        previewAsEnabled
                    } = await request.json()
                    const mode = requestHeaders[0]?.value
                    const badRequest = () =>
                        Response.json({ message: 'Invalid preview input' }, { status: 400 })
                    if (
                        requestHeaders.length !== 1 ||
                        requestHeaders[0].name !== 'X-Preview-Mode' ||
                        !previewAsEnabled
                    )
                        return badRequest()
                    if (mode === 'draft:config') {
                        if (
                            savedUuid ||
                            userAgent !== 'ClashMeta' ||
                            host?.remark !== 'Draft preview host' ||
                            host.address !== 'draft.example.test' ||
                            host.port !== 8443 ||
                            !host.isDisabled ||
                            !profiles.some(
                                (profile) =>
                                    profile.uuid === host.inbound.configProfileUuid &&
                                    profile.inbounds.some(
                                        (item) =>
                                            item.uuid === host.inbound.configProfileInboundUuid
                                    )
                            )
                        )
                            return badRequest()
                        return result({
                            clientType: 'MIHOMO',
                            contentType: 'text/plain',
                            ruleName: 'Draft host rule',
                            status: 200,
                            headers: {},
                            previewContent: `${host.address}:${host.port}`
                        })
                    }
                    if (host || savedUuid !== hostUuid || userAgent !== 'v2rayN/7.0.0')
                        return badRequest()
                    if (mode === 'drop')
                        return result({
                            clientType: 'SOCKET_DROP',
                            contentType: '',
                            ruleName: 'Socket drop rule',
                            status: null,
                            headers: {},
                            previewContent: ''
                        })
                    if (mode !== 'saved') return badRequest()
                    return result({
                        clientType: 'XRAY_JSON',
                        contentType: 'application/json',
                        ruleName: 'Saved host rule',
                        status: 200,
                        headers: { 'X-Preview': mode },
                        previewContent: JSON.stringify({
                            address: `${savedHost.address}:${savedHost.port}`
                        })
                    })
                }
                if (pathname === '/api/subscription-page-configs/') {
                    if (request.method === 'POST') {
                        const { name } = await request.json()
                        createdSubpage = {
                            ...defaultSubpage,
                            name,
                            uuid: '41414141-4141-4141-8141-414141414141',
                            viewPosition: 1
                        }
                        return result(createdSubpage)
                    }
                    const configs = createdSubpage
                        ? [defaultSubpage, createdSubpage]
                        : [defaultSubpage]
                    return result({ total: configs.length, configs })
                }
                if (pathname === `/api/subscription-page-configs/${defaultSubpage.uuid}`)
                    return result(defaultSubpage)
                if (
                    createdSubpage &&
                    pathname === `/api/subscription-page-configs/${createdSubpage.uuid}`
                )
                    return result(createdSubpage)
                if (pathname === '/api/system/nodes/metrics') {
                    return result({
                        nodes: [
                            {
                                nodeUuid: geoNode.uuid,
                                nodeName: geoNode.name,
                                countryEmoji: '🇺🇸',
                                providerName: 'Fixture Hosting',
                                usersOnline: 3,
                                inboundsStats: [
                                    {
                                        tag: 'active-inbound',
                                        upload: '12.5 MiB',
                                        download: '32 KiB'
                                    },
                                    { tag: 'idle-inbound', upload: '0', download: '0' }
                                ],
                                outboundsStats: [{ tag: 'direct', upload: '0', download: '4 KiB' }]
                            },
                            {
                                nodeUuid: panelNode.uuid,
                                nodeName: panelNode.name,
                                countryEmoji: '🇺🇸',
                                providerName: 'unknown',
                                usersOnline: 0,
                                inboundsStats: [],
                                outboundsStats: []
                            },
                            {
                                nodeUuid: legacyNode.uuid,
                                nodeName: legacyNode.name,
                                countryEmoji: '🇺🇸',
                                providerName: 'unknown',
                                usersOnline: 0,
                                inboundsStats: [],
                                outboundsStats: [{ tag: 'direct', upload: '0', download: '2 KiB' }]
                            }
                        ]
                    })
                }
                if (pathname === '/api/system/stats') {
                    if (++homeStatsRequests > 1)
                        return Response.json({ message: 'Temporary failure' }, { status: 500 })
                    return result({
                        cpu: { cores: 2 },
                        memory: { total: 1024, free: 512, used: 512 },
                        uptime: 100,
                        timestamp: 1,
                        users: {
                            totalUsers: 2,
                            statusCounts: { ACTIVE: 2, DISABLED: 0, EXPIRED: 0, LIMITED: 0 }
                        },
                        onlineStats: { lastDay: 2, lastWeek: 2, neverOnline: 0, onlineNow: 1 },
                        nodes: { totalOnline: 1, totalBytesLifetime: '1024' }
                    })
                }
                if (pathname === '/api/system/stats/bandwidth') {
                    const period = { current: '1 KiB', previous: '0 B', difference: '0%' }
                    return result({
                        bandwidthLastTwoDays: period,
                        bandwidthLastSevenDays: period,
                        bandwidthLast30Days: period,
                        bandwidthCalendarMonth: period,
                        bandwidthCurrentYear: period
                    })
                }
                if (pathname === '/api/system/health') {
                    await Bun.sleep(250)
                    return result({ runtimeMetrics: [] })
                }
                if (pathname === '/api/subscription-settings/') {
                    return result({
                        uuid: squadUuid,
                        serveJsonAtBaseSubscription: false,
                        randomizeHosts: false,
                        isShowCustomRemarks: true,
                        customRemarks: {
                            expiredUsers: ['Expired'],
                            limitedUsers: ['Limited'],
                            disabledUsers: ['Disabled'],
                            emptyHosts: ['No hosts'],
                            HWIDMaxDevicesExceeded: ['Device limit reached'],
                            HWIDNotSupported: ['Unsupported']
                        },
                        customResponseHeaders: null,
                        responseRules: null,
                        hwidSettings: null,
                        createdAt: '2025-05-01T11:00:00Z',
                        updatedAt: '2025-05-01T11:00:00Z'
                    })
                }
                if (pathname === '/api/users/' && request.method === 'GET') {
                    if (++userListRequests <= 2) await Bun.sleep(250)
                    return result({ total: users.length, users })
                }
                if (pathname.replace(/\/$/, '') === '/api/users' && request.method === 'PATCH') {
                    const body = await request.json()
                    const valid =
                        body.id === 43 &&
                        body.email === 'edited@example.test' &&
                        !('trafficLimitStrategy' in body) &&
                        (savedUserEdits === 0
                            ? ['trafficLimitBytes', 'description', 'tag'].every(
                                  (field) => !(field in body)
                              )
                            : body.trafficLimitBytes === 0 &&
                              body.description === null &&
                              body.tag === null)
                    if (!valid)
                        return Response.json({ message: 'Incorrect user edit' }, { status: 400 })
                    savedUserEdits++
                    return result(Object.assign(users[1], body))
                }
                if (request.method === 'GET') {
                    const user = users.find(({ id }) => pathname === `/api/users/${id}`)
                    if (user) return result(user)
                }
                if (pathname === '/api/users/bulk/all/update' && request.method === 'POST') {
                    const body = await request.json()
                    if (
                        body.trafficLimitBytes !== 1_572_864 ||
                        body.email !== null ||
                        'status' in body
                    ) {
                        return Response.json(
                            { message: 'Bulk edit serialized the wrong fields' },
                            { status: 400 }
                        )
                    }
                    return new Response(null, { status: 204 })
                }
                if (
                    pathname === '/api/users/bulk/extend-expiration-date' &&
                    request.method === 'POST'
                ) {
                    const body = await request.json()
                    if (
                        body.extendDays !== 3 ||
                        body.userIds?.length !== 1 ||
                        body.userIds[0] !== 42
                    ) {
                        return Response.json(
                            { message: 'Bulk action targeted the wrong selection' },
                            { status: 400 }
                        )
                    }
                    return new Response(null, { status: 204 })
                }
                if (pathname === '/api/subscription-request-history/stats') {
                    return result({
                        byParsedApp: [{ app: 'Fixture client', count: 19 }],
                        hourlyRequestStats: [5, 11, 3].map((requestCount, hour) => ({
                            dateTime: new Date(Date.UTC(2025, 4, 1, hour)).toISOString(),
                            requestCount
                        }))
                    })
                }
                if (pathname.replace(/\/$/, '') === '/api/subscription-request-history') {
                    return result({
                        total: 2,
                        records: ['2001:db8::42', null].map((requestIp, index) => ({
                            id: 2 - index,
                            userId: 42,
                            requestIp,
                            requestAt: userDate,
                            userAgent: 'Fixture client',
                            srrResponseType: 'MIHOMO',
                            srrRuleName: null
                        }))
                    })
                }
                if (pathname.replace(/\/$/, '') === '/api/node-plugins/torrent-blocker') {
                    return result({
                        total: 1,
                        records: [
                            {
                                id: 1,
                                userId: 42,
                                nodeId: 2,
                                user: { username: users[0].username },
                                node: {
                                    uuid: geoNode.uuid,
                                    name: geoNode.name,
                                    countryCode: geoNode.countryCode
                                },
                                createdAt: userDate,
                                report: {
                                    actionReport: {
                                        blocked: true,
                                        ip: '192.0.2.42',
                                        blockDuration: 60,
                                        willUnblockAt: userDate,
                                        userId: '42',
                                        processedAt: userDate
                                    },
                                    xrayReport: {
                                        email: null,
                                        level: null,
                                        protocol: 'bittorrent',
                                        network: 'tcp',
                                        source: null,
                                        destination: 'example.test',
                                        routeTarget: null,
                                        originalTarget: null,
                                        inboundTag: null,
                                        inboundName: null,
                                        inboundLocal: null,
                                        outboundTag: null,
                                        ts: 0
                                    }
                                }
                            }
                        ]
                    })
                }
                if (pathname === '/api/node-plugins/torrent-blocker/stats') {
                    return result({
                        stats: {
                            totalReports: 1,
                            reportsLast24Hours: 1,
                            distinctNodes: 1,
                            distinctUsers: 1
                        },
                        topUsers: [],
                        topNodes: []
                    })
                }
                if (pathname === '/api/bandwidth-stats/nodes/') {
                    const gigabyte = 1024 ** 3
                    const series = [geoNode, panelNode].map(
                        ({ uuid, name, countryCode }, index) => {
                            const data = (index ? [1, 5, 2] : [2, 1, 4]).map(
                                (amount) => amount * gigabyte
                            )
                            return {
                                uuid,
                                name,
                                countryCode,
                                color: index ? '#be4bdb' : '#339af0',
                                total: data.reduce((total, bytes) => total + bytes, 0),
                                data
                            }
                        }
                    )
                    return result({
                        categories: ['2025-05-01', '2025-05-02', '2025-05-03'],
                        sparklineData: [3, 6, 6].map((amount) => amount * gigabyte),
                        series,
                        topNodes: series
                    })
                }
                if (pathname.startsWith('/api/infra-billing/')) {
                    const date = '2025-05-02T00:00:00Z'
                    const billingProvider = {
                        uuid: '12121212-1212-4212-8212-121212121212',
                        name: 'Fixture Hosting',
                        faviconLink: null,
                        loginUrl: null
                    }
                    if (pathname === '/api/infra-billing/nodes') {
                        if (request.method === 'PATCH') {
                            const body = await request.json()
                            if (
                                body.nextBillingAt !== '2025-06-02T00:00:00.000Z' ||
                                body.uuids?.join() !== '23232323-2323-4232-8232-232323232323'
                            ) {
                                return Response.json(
                                    { message: 'Incorrect billing date payload' },
                                    { status: 400 }
                                )
                            }
                            nextBillingDate = body.nextBillingAt
                        }
                        return result({
                            totalBillingNodes: 1,
                            billingNodes: [
                                {
                                    uuid: '23232323-2323-4232-8232-232323232323',
                                    nodeUuid: geoNode.uuid,
                                    name: null,
                                    providerUuid: billingProvider.uuid,
                                    provider: billingProvider,
                                    node: {
                                        uuid: geoNode.uuid,
                                        name: geoNode.name,
                                        countryCode: geoNode.countryCode
                                    },
                                    nextBillingAt: nextBillingDate,
                                    createdAt: date,
                                    updatedAt: date
                                }
                            ],
                            availableBillingNodes: [],
                            totalAvailableBillingNodes: 0,
                            stats: {
                                upcomingNodesCount: 0,
                                currentMonthPayments: 12.5,
                                totalSpent: 12.5
                            }
                        })
                    }
                    if (pathname === '/api/infra-billing/providers') {
                        return result({
                            total: 1,
                            providers: [
                                {
                                    ...billingProvider,
                                    createdAt: date,
                                    updatedAt: date,
                                    billingHistory: { totalAmount: 12.5, totalBills: 1 },
                                    billingNodes: [
                                        {
                                            name: geoNode.name,
                                            details: {
                                                nodeUuid: geoNode.uuid,
                                                countryCode: geoNode.countryCode
                                            }
                                        }
                                    ]
                                }
                            ]
                        })
                    }
                    if (pathname === '/api/infra-billing/history') {
                        return result({
                            total: 1,
                            records: [
                                {
                                    uuid: '34343434-3434-4343-8343-343434343434',
                                    providerUuid: billingProvider.uuid,
                                    provider: billingProvider,
                                    amount: 12.5,
                                    billedAt: date
                                }
                            ]
                        })
                    }
                }
                if (pathname === '/api/hwid/devices/stats') {
                    return result({
                        byPlatform: [
                            {
                                platform: 'Android',
                                count: 200,
                                byApp: [
                                    { app: 'Fixture client', count: 199 },
                                    { app: 'Rare client', count: 1 }
                                ]
                            }
                        ],
                        stats: {
                            totalUniqueDevices: 200,
                            totalHwidDevices: 200,
                            averageHwidDevicesPerUser: 1
                        }
                    })
                }
                if (pathname === '/api/hwid/devices') return result({ devices: [], total: 0 })
                if (pathname === '/api/hwid/devices/top-users') {
                    const start = Number(searchParams.get('start') ?? 0)
                    const size = Number(searchParams.get('size') ?? 5)
                    const users = Array.from({ length: 6 }, (_, index) => ({
                        id: index === 0 ? 42 : 100 + index,
                        username: `device-user-${index + 1}`,
                        devicesCount: 10 - index
                    }))
                    return result({ users: users.slice(start, start + size), total: users.length })
                }
                if (pathname === '/api/subscription-templates/') {
                    if (request.method === 'POST') {
                        const body = await request.json()
                        if (body.name !== 'New template' || body.templateType !== 'MIHOMO')
                            return new Response(null, { status: 400 })
                        createdTemplate = {
                            uuid: mihomoUuid,
                            viewPosition: 0,
                            name: body.name,
                            tags: [],
                            templateType: body.templateType,
                            templateJson: null,
                            encodedTemplateYaml: null
                        }
                        return result(createdTemplate)
                    }
                    return result({
                        total: 1,
                        templates: [
                            {
                                uuid: mihomoUuid,
                                viewPosition: 0,
                                name: createdTemplate?.name ?? 'Fixture Mihomo',
                                tags: [],
                                templateType: 'MIHOMO',
                                templateJson: null,
                                encodedTemplateYaml: null
                            }
                        ]
                    })
                }
                if (pathname === `/api/subscription-templates/${mihomoUuid}` && createdTemplate)
                    return result(createdTemplate)
                const validationCase = templateValidationCases.find(
                    ({ uuid }) => pathname === `/api/subscription-templates/${uuid}`
                )
                if (validationCase) {
                    const { type, uuid, syntaxError } = validationCase
                    return result({
                        uuid,
                        viewPosition: 0,
                        name: `Diagnostic ${type}`,
                        tags: [],
                        templateType: type,
                        templateJson: type === 'SINGBOX' ? { log: { level: 'quiet' } } : null,
                        encodedTemplateYaml:
                            type === 'SINGBOX'
                                ? null
                                : btoa(
                                      syntaxError
                                          ? 'rules: [foo'
                                          : 'mixed-port: 7890\nredir-port: 78921231231212\n'
                                  )
                    })
                }
                if (pathname === `/api/external-squads/${externalUuid}`)
                    return result(externalSquad)
                if (pathname === '/api/external-squads/') {
                    if (request.method === 'POST') {
                        const body = await request.json()
                        if (body.name !== 'New external squad')
                            return new Response(null, { status: 400 })
                        return result(externalSquad)
                    }
                    if (request.method === 'PATCH') {
                        const { uuid, templates, subscriptionSettings } = await request.json()
                        const preservesTemplates =
                            templates?.length === 2 &&
                            templates.some(
                                ({ templateType, templateUuid }) =>
                                    templateType === 'XRAY_BASE64' && templateUuid === base64Uuid
                            ) &&
                            templates.some(
                                ({ templateType, templateUuid }) =>
                                    templateType === 'MIHOMO' && templateUuid === mihomoUuid
                            )
                        if (
                            uuid !== externalUuid ||
                            (!preservesTemplates && subscriptionSettings?.randomizeHosts !== true)
                        ) {
                            return new Response(null, { status: 400 })
                        }
                        externalSquad = {
                            ...externalSquad,
                            templates: templates ?? externalSquad.templates,
                            subscriptionSettings:
                                subscriptionSettings ?? externalSquad.subscriptionSettings
                        }
                        return result(externalSquad)
                    }
                    return result({ total: 1, externalSquads: [externalSquad] })
                }
                if (pathname === '/api/internal-squads/' || pathname === '/api/internal-squads') {
                    if (request.method === 'POST') {
                        const body = await request.json()
                        if (
                            body.name !== 'New internal squad' ||
                            JSON.stringify(body.inbounds) !== '[]'
                        )
                            return new Response(null, { status: 400 })
                        return result(savedSquad())
                    }
                    if (request.method === 'PATCH') {
                        const { uuid, inbounds, name } = await request.json()
                        if (
                            uuid !== squadUuid ||
                            name !== undefined ||
                            inbounds?.length !== 2 ||
                            !inbounds.includes(inbound.uuid) ||
                            !inbounds.includes(extraInbound.uuid)
                        ) {
                            return new Response(null, { status: 400 })
                        }
                        squadInbounds = [inbound, extraInbound]
                        return result(savedSquad())
                    }
                    return result({ total: 1, internalSquads: [savedSquad()] })
                }
                if (pathname === `/api/internal-squads/${squadUuid}`) return result(savedSquad())
                if (pathname === '/api/config-profiles/' || pathname === '/api/config-profiles') {
                    if (request.method === 'POST') {
                        const { name, config } = await request.json()
                        if (
                            name !== 'New profile' ||
                            config?.inbounds?.[0]?.protocol !== 'shadowsocks' ||
                            config?.outbounds?.length !== 2
                        )
                            return new Response(null, { status: 400 })
                        return result(profiles.find(({ uuid }) => uuid === inbound.profileUuid))
                    }
                    if (request.method === 'PATCH') {
                        const { uuid, name, config } = await request.json()
                        if (config)
                            return Response.json(
                                {
                                    message:
                                        'constraint failed: FOREIGN KEY constraint failed (787)'
                                },
                                { status: 500 }
                            )
                        const profile = profiles.find((item) => item.uuid === uuid)
                        if (!profile || !name) return new Response(null, { status: 400 })
                        profile.name = name
                        return result(profile)
                    }
                    return profileList()
                }
                if (request.method === 'GET') {
                    const profile = profiles.find(
                        ({ uuid }) => pathname === `/api/config-profiles/${uuid}`
                    )
                    if (profile) return result(profile)
                }
                if (pathname === `/api/config-profiles/${inbound.profileUuid}/inbounds`) {
                    return result({ total: 1, inbounds: [inbound] })
                }
                if (pathname === `/api/config-profiles/${inbound.profileUuid}/computed-config`) {
                    return result({
                        ...profiles.find(({ uuid }) => uuid === inbound.profileUuid),
                        config: { fixture: 'fixture-computed' }
                    })
                }
                if (
                    pathname === '/api/config-profiles/actions/reorder' &&
                    request.method === 'POST'
                ) {
                    const { items } = await request.json()
                    if (
                        items.length !== 2 ||
                        items[0].uuid !== profiles[1].uuid ||
                        items[1].uuid !== profiles[0].uuid ||
                        items[0].viewPosition !== 0 ||
                        items[1].viewPosition !== 1
                    ) {
                        return new Response(null, { status: 400 })
                    }
                    profiles.reverse().forEach((profile, index) => {
                        profile.viewPosition = index
                    })
                    return profileList()
                }
                if (pathname === '/api/config-profiles/tags' && request.method === 'PATCH') {
                    const body = await request.json()
                    if (
                        body.uuid !== profiles[0].uuid ||
                        JSON.stringify(body.tags) !== '["STAGE"]'
                    ) {
                        return new Response(null, { status: 400 })
                    }
                    profiles[0].tags = body.tags
                    return result({ uuid: body.uuid, tags: body.tags })
                }
                if (pathname === '/api/snippets/' || pathname === '/api/snippets') {
                    if (request.method === 'PATCH') {
                        const body = await request.json()
                        if (JSON.stringify(body) !== JSON.stringify(fragments[0])) {
                            return new Response(null, { status: 400 })
                        }
                    } else if (request.method === 'DELETE') {
                        const body = await request.json()
                        if (body.name !== fragments[0]?.name)
                            return new Response(null, { status: 400 })
                        fragments.pop()
                        return new Response(null, { status: 204 })
                    } else if (request.method !== 'GET') {
                        return new Response(null, { status: 405 })
                    }
                    return result({ total: fragments.length, snippets: fragments })
                }
                if (pathname === '/api/snippets/actions/sync' && request.method === 'POST') {
                    const body = await request.json()
                    return new Response(null, {
                        status: body.name === snippetName ? 204 : 400
                    })
                }
                if (pathname === '/api/internal-squads/tags' && request.method === 'PATCH') {
                    const body = await request.json()
                    if (body.uuid !== squadUuid || JSON.stringify(body.tags) !== '["DEMO"]') {
                        return new Response(null, { status: 400 })
                    }
                    tags = body.tags
                    return result({ uuid: squadUuid, tags })
                }
                if (
                    pathname === `/api/internal-squads/${squadUuid}/bulk-actions/add-users` &&
                    request.method === 'POST'
                ) {
                    membersCount = 2
                    return new Response(null, { status: 204 })
                }
                if (
                    pathname === `/api/internal-squads/${squadUuid}/bulk-actions/remove-users` &&
                    request.method === 'DELETE'
                ) {
                    membersCount = 0
                    return new Response(null, { status: 204 })
                }
                if (pathname === `/api/internal-squads/${squadUuid}/accessible-nodes`) {
                    return result({
                        squadUuid,
                        accessibleNodes: [
                            {
                                uuid: '22222222-2222-4222-8222-222222222222',
                                nodeName: 'Demo node',
                                countryCode: 'US',
                                configProfileUuid: '33333333-3333-4333-8333-333333333333',
                                configProfileName: 'Demo profile',
                                activeInbounds: ['Demo inbound']
                            }
                        ]
                    })
                }
                if (pathname === `/api/bandwidth-stats/internal-squads/${squadUuid}/usage`) {
                    if (
                        !searchParams.get('start') ||
                        !searchParams.get('end') ||
                        searchParams.get('limit') !== '250'
                    ) {
                        return new Response(null, { status: 400 })
                    }
                    const threshold = searchParams.get('minTotalBytes')
                    if (threshold === '107374182400') {
                        return result({ squadUuid, users: [], nextCursor: null, hasMore: false })
                    }
                    if (threshold !== '1610612736') return new Response(null, { status: 400 })
                    const cursor = searchParams.get('cursor')
                    if (cursor === '42') {
                        return result({
                            squadUuid,
                            users: [{ id: 43, totalBytes: 2 ** 31 }],
                            nextCursor: null,
                            hasMore: false
                        })
                    }
                    if (cursor !== null) return new Response(null, { status: 400 })
                    return result({
                        squadUuid,
                        users: [{ id: 42, totalBytes: 2 ** 31 }],
                        nextCursor: '42',
                        hasMore: true
                    })
                }
                if (pathname === '/api/node-plugins/shared-lists') {
                    return result({
                        total: 2,
                        sharedLists: [
                            { name: 'Network/office', type: 'ipList', itemsCount: 3 },
                            { name: 'Network/providers', type: 'asList', itemsCount: 2 }
                        ]
                    })
                }
                const fixturePlugin = {
                    uuid: '11111111-4444-4111-8111-111111111111',
                    viewPosition: 0,
                    name: 'Fixture plugin',
                    tags: [],
                    pluginConfig: {}
                }
                if (pathname === '/api/node-plugins/' || pathname === '/api/node-plugins') {
                    if (request.method === 'POST') {
                        const body = await request.json()
                        if (body.name !== 'New plugin') return new Response(null, { status: 400 })
                        return result(fixturePlugin)
                    }
                    return result({ total: 1, nodePlugins: [fixturePlugin] })
                }
                if (pathname === `/api/node-plugins/${fixturePlugin.uuid}`)
                    return result(fixturePlugin)
                if (pathname.startsWith('/api/connections/by-node/')) {
                    const node = nodes.find(({ uuid }) => pathname.endsWith(`/${uuid}`))
                    if (!node) return new Response(null, { status: 404 })
                    if (request.method === 'POST')
                        return Response.json({ response: { jobId: node.uuid } }, { status: 201 })
                    return result({
                        isCompleted: true,
                        isFailed: false,
                        result: {
                            success: true,
                            nodeUuid: node.uuid,
                            users: [
                                {
                                    userId: node.id,
                                    ips: [
                                        {
                                            ip: `192.0.2.${10 + node.id}`,
                                            lastSeen: new Date().toISOString()
                                        }
                                    ]
                                }
                            ]
                        }
                    })
                }
                if (pathname === '/api/nodes' || pathname === '/api/nodes/') {
                    if (request.method === 'POST') {
                        const body = await request.json()
                        return body.name === setupNode.name && body.address === setupNode.address
                            ? Response.json({ response: setupNode }, { status: 201 })
                            : new Response(null, { status: 400 })
                    }
                    return result(nodes)
                }
                if (pathname === `/api/nodes/${setupNode.uuid}`) {
                    const attempt = setupPolls++
                    if (attempt === 0) return result(setupNode)
                    return result({
                        ...setupNode,
                        isConnecting: false,
                        isConnected: attempt > 1,
                        lastStatusMessage: attempt === 1 ? 'Fixture connection failed' : null
                    })
                }
                if (pathname === '/api/nodes/actions/reorder' && request.method === 'POST') {
                    const { nodes: order } = await request.json()
                    const reordered = order.map(({ uuid, viewPosition }) => {
                        const node = nodes.find((item) => item.uuid === uuid)
                        node.viewPosition = viewPosition
                        return node
                    })
                    nodes.splice(0, nodes.length, ...reordered)
                    return result(nodes)
                }
                if (pathname === `/api/metadata/node/${geoNode.uuid}`) {
                    return result({ metadata: { location: 'fixture-location' } })
                }
                const node = nodes.find(({ uuid }) => pathname === `/api/nodes/${uuid}`)
                if (node) return result(node)
                const updateNode = nodes.find(({ uuid }) =>
                    pathname.startsWith(`/api/nodes/${uuid}/updates`)
                )
                if (updateNode && pathname.endsWith('/apply') && request.method === 'POST') {
                    const body = await request.json()
                    if (updateNode === legacyNode && body.type === 'core') {
                        return Response.json({ errorCode: 'GW_NODE_LOCAL_CORE' }, { status: 409 })
                    }
                    if (
                        (updateNode === geoNode && body.type === 'node') ||
                        (updateNode === panelNode && body.type === 'core')
                    ) {
                        return result({ accepted: true })
                    }
                    return Response.json(
                        { message: `Unexpected ${body.type} update for ${updateNode.name}` },
                        { status: 400 }
                    )
                }
                if (updateNode && request.method === 'GET') {
                    const core = searchParams.get('repo') === 'XTLS/Xray-core'
                    if (core && updateNode === geoNode) {
                        return Response.json(
                            { message: 'Local core requested Xray releases' },
                            { status: 400 }
                        )
                    }
                    return result({
                        architecture: 'arm64',
                        os: 'linux',
                        allReleases: [
                            {
                                tag_name: 'v1.0.0',
                                prerelease: false,
                                assets: [
                                    {
                                        name: core
                                            ? 'Xray-linux-arm64-v8a.zip'
                                            : 'gowave-node-linux-arm64',
                                        browser_download_url: 'https://example.invalid/release',
                                        size: 1
                                    }
                                ]
                            }
                        ]
                    })
                }
                if (
                    pathname === `/api/connections/geocheck/${geoNode.uuid}` &&
                    request.method === 'POST'
                ) {
                    const body = await request.json()
                    if (!body.ip && !body.interface) return result({ jobId: 'browser-geo-failed' })
                    if (body.ip !== geoNode.address) return new Response(null, { status: 400 })
                    return result({ jobId: 'browser-geo' })
                }
                if (
                    pathname === '/api/connections/geocheck/browser-geo-failed' &&
                    request.method === 'GET'
                ) {
                    return result({
                        isCompleted: false,
                        isFailed: true,
                        result: {
                            success: false,
                            nodeUuid: geoNode.uuid,
                            image: null,
                            rawReport: null,
                            message: 'node disabled'
                        }
                    })
                }
                if (
                    pathname === '/api/connections/geocheck/browser-geo' &&
                    request.method === 'GET'
                ) {
                    if (geoPolls++ < 2)
                        return result({ isCompleted: false, isFailed: false, result: null })
                    return result({
                        isCompleted: true,
                        isFailed: false,
                        result: {
                            success: true,
                            nodeUuid: geoNode.uuid,
                            image: {
                                format: 'svg',
                                media_type: 'image/svg+xml',
                                encoding: 'base64',
                                data: btoa(
                                    '<svg xmlns="http://www.w3.org/2000/svg" width="120" height="80"><rect width="120" height="80" fill="teal"/></svg>'
                                )
                            },
                            rawReport: {
                                connectivity: {
                                    targets: [
                                        {
                                            name: 'Example trace',
                                            verdict: 'direct',
                                            rtt_ms: 12,
                                            hops: []
                                        },
                                        {
                                            name: 'Transit trace',
                                            host: 'example.test',
                                            verdict: 'transit',
                                            rtt_ms: 21,
                                            hops: [
                                                {
                                                    ttl: 1,
                                                    addr: '192.0.2.1',
                                                    asn: 64500,
                                                    as_name: 'Example network',
                                                    avg_ms: 3,
                                                    loss: 0,
                                                    recv: 3,
                                                    sent: 3
                                                }
                                            ]
                                        }
                                    ]
                                }
                            },
                            message: null
                        }
                    })
                }
                if (pathname === '/api/system/metadata') {
                    const commit = {
                        commitSha: '1234567890abcdef',
                        commitUrl: 'https://github.com/aaaSaZaN/gowave/commit/1234567890abcdef'
                    }
                    return result({
                        version: '1.2.3',
                        build: { time: '2025-05-01T11:00:00Z', number: '1' },
                        git: { backend: { ...commit, branch: 'main' }, frontend: commit }
                    })
                }
                if (pathname === '/api/system/updates/status') {
                    return Response.json({ status: null, requestPending: false })
                }
                if (pathname === '/api/system/updates/check') {
                    const channel = searchParams.get('channel') || 'stable'
                    const manualUpdate = channel === 'stable'
                    if (!manualUpdate) await Bun.sleep(250)
                    return Response.json({
                        channel: 'stable',
                        targetChannel: channel,
                        currentVersion: '1.2.3',
                        latestVersion: manualUpdate ? '1.2.4' : 'dev',
                        hasUpdate: true,
                        checkedAt: '2025-05-01T11:00:00Z',
                        isDocker: false,
                        canAutoUpdate: !manualUpdate,
                        updateCommand: manualUpdate
                            ? `download https://example.test/${'release/'.repeat(40)}install.sh`
                            : undefined
                    })
                }
                if (pathname === '/api/system/stats/recap') {
                    return result({
                        thisMonth: { users: 2, traffic: '512' },
                        total: {
                            users: 14,
                            nodes: 2,
                            traffic: '1024',
                            nodesRam: '0',
                            nodesCpuCores: 0,
                            distinctCountries: 1
                        },
                        version: 'v0.0.0-browser-test',
                        initDate: '2025-05-01T11:00:00Z'
                    })
                }
                return new Response(null, { status: 404 })
            }
            if (!pathname.startsWith(panel)) return new Response(null, { status: 404 })
            if (pathname === `${panel}__verify.js`) {
                return new Response(
                    `(${verify.toString()})(${JSON.stringify({ externalUuid, mihomoUuid, squadUuid, hostUuid, templateValidationCases })});`,
                    {
                        headers: { 'Content-Type': 'text/javascript' }
                    }
                )
            }
            const relative = pathname.slice(panel.length)
            const asset = Bun.file(join(output, relative))
            if (relative.includes('.')) {
                if (/^assets\/Users-[^/]+\.js$|^lotties\/[^/]+\.lottie$/.test(relative)) {
                    await Bun.sleep(250)
                }
                if (await asset.exists()) return new Response(asset)
                if (relative === 'assets/mihomo.schema.json')
                    return Response.json({
                        type: 'object',
                        properties: { 'redir-port': { type: 'integer', maximum: 65535 } }
                    })
                if (relative === 'assets/singbox.schema.json')
                    return Response.json({
                        type: 'object',
                        properties: {
                            log: {
                                type: 'object',
                                properties: { level: { enum: ['debug', 'info', 'warn'] } }
                            }
                        }
                    })
                return new Response(null, { status: 404 })
            }
            return new Response(entry, { headers: { 'Content-Type': 'text/html' } })
        }
    })
    return { server, url: new URL(`${panel}dashboard/tools/quick-open`, server.url).href }
}
