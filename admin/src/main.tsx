import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import './index.css'
import { adminBase } from '@/base'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter basename={`${adminBase()}/admin`}>
      <App />
    </BrowserRouter>
  </StrictMode>,
)
