import { render } from 'preact'
import './reset.css'
import './tokens.css'
import { initTheme } from './utils/theme'
import { App } from './app'

initTheme()

if ('scrollRestoration' in history) {
  history.scrollRestoration = 'manual'
}

render(<App />, document.getElementById('app')!)
