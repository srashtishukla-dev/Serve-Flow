import { ArrowLeft, Compass } from 'lucide-react'
import { Link } from 'react-router-dom'
import Logo from '../../components/ui/Logo'

export default function NotFound() {
  return (
    <main className="status-page">
      <header className="status-page__header"><Logo /><Link className="text-link" to="/">Back to home <ArrowLeft size={15} /></Link></header>
      <section className="status-card" aria-labelledby="not-found-title">
        <span className="status-card__icon"><Compass size={22} /></span>
        <span className="section-kicker">404 · PAGE NOT FOUND</span>
        <h1 id="not-found-title">Looks like this<br /><em>route ran off.</em></h1>
        <p>The page may have moved, or the address may be mistyped. Let&apos;s get you back to the start.</p>
        <Link className="button button--primary" to="/">Return to the home page <ArrowLeft size={16} /></Link>
      </section>
    </main>
  )
}