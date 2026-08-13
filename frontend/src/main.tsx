import { render } from 'preact'
import './reset.css'
import './tokens.css'
import { App } from './app'

if ('scrollRestoration' in history) {
  history.scrollRestoration = 'manual'
}

render(<App />, document.getElementById('app')!)
