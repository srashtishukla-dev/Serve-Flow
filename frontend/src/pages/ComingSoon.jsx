import { ArrowLeft, Clock3 } from 'lucide-react'
import { Link, useLocation } from 'react-router-dom'
import Logo from '../components/ui/Logo'

const titles = {
  '/login': 'Sign in',
  '/register': 'Get started',
  '/dashboard': 'Dashboard',
  '/tickets': 'Tickets',
  '/customers': 'Customers',
  '/technicians': 'Technicians',
  '/appointments': 'Appointments',
  '/invoices': 'Invoices',
  '/analytics': 'Analytics',
  '/settings': 'Settings',
}

export default function ComingSoon() {
  const { pathname } = useLocation()

  return (
    <main className="status-page">
      <header className="status-page__header"><Logo /><Link className="text-link" to="/">Back to home <ArrowLeft size={15} /></Link></header>
      <section className="status-card" aria-labelledby="status-title">
        <span className="status-card__icon"><Clock3 size={22} /></span>
        <span className="section-kicker">SERVEFLOW IS IN ITS FIRST CHAPTER</span>
        <h1 id="status-title">{titles[pathname] || 'This page'}<br /><em>is on the way.</em></h1>
        <p>This part of ServeFlow is not available yet. The Day 1 foundation is focused on the public landing page and API health endpoint.</p>
        <Link className="button button--primary" to="/">Return to the home page <ArrowLeft size={16} /></Link>
      </section>
    </main>
  )
}