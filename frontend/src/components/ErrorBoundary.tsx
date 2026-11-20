import { Component } from 'preact'
import type { ComponentChildren } from 'preact'
import { getLocale, translate } from '../i18n'
import s from './ErrorBoundary.module.css'

interface Props {
  children: ComponentChildren
}

interface State {
  hasError: boolean
}

export class ErrorBoundary extends Component<Props, State> {
  state = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  componentDidCatch(error: Error, info: unknown): void {
    console.error('[ErrorBoundary]', error, info)
    try {
      fetch('/api/client-errors', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: error.message, stack: error.stack }),
      }).catch(() => { /* non-blocking */ })
    } catch {
      // non-blocking
    }
  }

  handleReload = () => {
    window.location.reload()
  }

  render() {
    if (!this.state.hasError) return this.props.children

    const locale = getLocale()

    return (
      <div className={s.container}>
        <div className={s.emoticon}>:(</div>
        <div className={s.title}>
          {translate(locale, 'errorBoundary.title')}
        </div>
        <div className={s.description}>
          {translate(locale, 'errorBoundary.description')}
        </div>
        <button onClick={this.handleReload} className={s.reloadButton}>
          {translate(locale, 'errorBoundary.reload')}
        </button>
      </div>
    )
  }
}
