import { installClipboardFallback } from '@shared/lib/clipboard'
import { polyfillCountryFlagEmojis } from 'country-flag-emoji-polyfill'
import { mount } from 'svelte'

import App from './App.svelte'
import './ui/vendor/mantine-core.css'
import './ui/theme.css'
import './ui/dashboard.css'
import './styles.css'

const target = document.getElementById('root')
if (!target) throw new Error('Gowave root element is missing')

installClipboardFallback()
polyfillCountryFlagEmojis()
mount(App, { target })
