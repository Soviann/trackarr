import Router from 'preact-router'

export function App() {
  return (
    <div style={{ minHeight: '100vh' }}>
      <Router>
        <Home path="/" />
      </Router>
    </div>
  )
}

function Home() {
  return (
    <div style={{ padding: '20px' }}>
      <h1 style={{ fontSize: '20px', fontWeight: 700 }}>PlexTracker</h1>
      <p style={{ color: '#666', marginTop: '8px' }}>Coming soon.</p>
    </div>
  )
}
