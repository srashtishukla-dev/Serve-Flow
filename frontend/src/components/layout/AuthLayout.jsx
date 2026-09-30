import { ArrowLeft } from 'lucide-react'
import { Link } from 'react-router-dom'
import Logo from '../ui/Logo'

export default function AuthLayout({ children }) {
  return (
    <main className="auth-page">
      <header className="auth-navbar">
        <Logo />
        <Link className="auth-navbar__home" to="/"><ArrowLeft size={15} /> Back to home</Link>
      </header>
      <div className="auth-page__content">{children}</div>
      <footer className="auth-page__footer">ServeFlow <span>·</span> Service work, moving forward.</footer>
    </main>
  )
}