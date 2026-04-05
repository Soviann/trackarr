import { render } from 'preact'
import './reset.css'
import './tokens.css'
import { App } from './app'

render(<App />, document.getElementById('app')!)
