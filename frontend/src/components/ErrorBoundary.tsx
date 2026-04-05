import { Component } from 'preact'
import type { ComponentChildren } from 'preact'
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

  handleReload = () => {
    window.location.reload()
  }

  render() {
    if (!this.state.hasError) return this.props.children

    return (
      <div className={s.container}>
        <div className={s.emoticon}>:(</div>
        <div className={s.title}>
          Something went wrong
        </div>
        <div className={s.description}>
          An unexpected error occurred. Please reload the page.
        </div>
        <button onClick={this.handleReload} className={s.reloadButton}>
          Reload
        </button>
      </div>
    )
  }
}
