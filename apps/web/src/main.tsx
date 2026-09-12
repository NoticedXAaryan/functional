import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import CommunityAlerts from './CommunityAlerts.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    {window.location.pathname.startsWith('/alerts') ? <CommunityAlerts /> : <App />}
  </StrictMode>,
)
