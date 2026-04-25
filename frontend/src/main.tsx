import { render } from 'preact'
import './reset.css'
import './tokens.css'
import { App } from './app'

if ('scrollRestoration' in history) {
  history.scrollRestoration = 'manual'
}

// Some WebKit/iOS contexts ignore `scrollRestoration = 'manual'` and still
// jump to the previously saved scroll once async content reaches that height.
// Cancel any non-user-initiated scroll for a short window after first paint.
let userInteracted = false
const markInteraction = () => { userInteracted = true }
const onScroll = () => {
  if (!userInteracted && window.scrollY !== 0) window.scrollTo(0, 0)
}
const interactionEvents = ['touchstart', 'wheel', 'keydown', 'pointerdown'] as const
interactionEvents.forEach(e => addEventListener(e, markInteraction, { passive: true, once: true }))
addEventListener('scroll', onScroll, { passive: true })
setTimeout(() => {
  removeEventListener('scroll', onScroll)
  interactionEvents.forEach(e => removeEventListener(e, markInteraction))
}, 1500)

render(<App />, document.getElementById('app')!)
