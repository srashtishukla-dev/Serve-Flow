import { useState } from 'react'
import {
  ArrowRight,
  ArrowUpRight,
  BarChart3,
  CalendarDays,
  Check,
  ChevronRight,
  CircleDollarSign,
  ClipboardCheck,
  Headphones,
  Menu,
  ShieldCheck,
  UsersRound,
  Wrench,
  X,
} from 'lucide-react'
import { Link } from 'react-router-dom'
import Logo from '../../components/ui/Logo'
import DashboardPreview from '../../components/layout/DashboardPreview'

const features = [
  { icon: ClipboardCheck, number: '01', title: 'Smart ticket management', text: 'Keep every request moving with clear ownership, priorities, and a full service history.' },
  { icon: UsersRound, number: '02', title: 'Customer relationships', text: 'Give your team the context they need with one dependable view of every customer.' },
  { icon: Wrench, number: '03', title: 'Technician operations', text: 'Balance the day, assign the right work, and keep field teams in sync.' },
  { icon: CalendarDays, number: '04', title: 'Appointments that fit', text: 'Coordinate visits with less back-and-forth and a shared view of the schedule.' },
  { icon: CircleDollarSign, number: '05', title: 'Invoicing, connected', text: 'Bring completed work and billing into one straightforward workflow.' },
  { icon: BarChart3, number: '06', title: 'Useful analytics', text: 'See service patterns clearly and make the next operational decision with confidence.' },
]

const steps = [
  { number: '01', title: 'Create your workspace', text: 'Set up a home for your service team and the work you do every day.' },
  { number: '02', title: 'Manage service requests', text: 'Bring requests, customer details, and schedules into a single view.' },
  { number: '03', title: 'Resolve and track work', text: 'Follow each job through completion and understand how your operation is moving.' },
]

export default function Landing() {
  const [menuOpen, setMenuOpen] = useState(false)
  const closeMenu = () => setMenuOpen(false)

  return (
    <>
      <header className="site-header">
        <div className="site-header__inner">
          <Logo />
          <button
            aria-expanded={menuOpen}
            aria-label={menuOpen ? 'Close navigation menu' : 'Open navigation menu'}
            className="mobile-menu-button"
            onClick={() => setMenuOpen(!menuOpen)}
            type="button"
          >
            {menuOpen ? <X size={21} /> : <Menu size={21} />}
          </button>
          <nav aria-label="Main navigation" className={`main-nav${menuOpen ? ' main-nav--open' : ''}`}>
            <a href="#features" onClick={closeMenu}>Features</a>
            <a href="#solutions" onClick={closeMenu}>Solutions</a>
            <a href="#pricing" onClick={closeMenu}>Pricing</a>
            <a href="#about" onClick={closeMenu}>About</a>
            <div className="main-nav__actions">
              <Link className="nav-login" onClick={closeMenu} to="/login">Log in</Link>
              <Link className="button button--small button--primary" onClick={closeMenu} to="/register">Get started <ArrowUpRight size={15} /></Link>
            </div>
          </nav>
        </div>
      </header>

      <main>
        <section className="hero" id="about">
          <div className="hero__texture" aria-hidden="true" />
          <div className="hero__inner page-width">
            <div className="hero__copy">
              <div className="eyebrow"><span className="eyebrow__dot" /> THE SERVICE TEAM&apos;S NEW HOME</div>
              <h1>Service operations,<br /><em>simplified.</em></h1>
              <p className="hero__description">Bring customers, service requests, technicians, and day-to-day operations together in one clear workspace.</p>
              <div className="hero__actions">
                <Link className="button button--primary" to="/register">Start free <ArrowRight size={17} /></Link>
                <a className="button button--outline" href="#product">View the product <span className="play-icon">▶</span></a>
              </div>
              <div className="hero__note"><ShieldCheck size={15} /> A calmer way to run service, from day one.</div>
            </div>
            <div className="hero__visual">
              <div className="hero__visual-note"><span className="live-indicator" /> A clearer view of the day</div>
              <DashboardPreview />
              <div className="floating-note"><span className="floating-note__icon"><Check size={14} /></span><span><strong>One shared picture</strong><small>Work, team, and schedule</small></span></div>
            </div>
          </div>
          <div className="hero__bottom page-width"><span>BUILT FOR THE PEOPLE WHO KEEP THINGS RUNNING</span><div><span>FIELD SERVICE</span><i /> <span>REPAIR TEAMS</span><i /> <span>SERVICE OPERATIONS</span></div></div>
        </section>

        <section className="section features-section" id="features">
          <div className="page-width">
            <div className="section-heading section-heading--split">
              <div><span className="section-kicker">LESS JUGGLING, MORE FLOW</span><h2>Everything your service day needs.<br /><em>Nothing it doesn&apos;t.</em></h2></div>
              <p>Good service is a team effort. ServeFlow gives every part of the operation a place to connect, so work keeps moving and people stay in the loop.</p>
            </div>
            <div className="feature-grid">
              {features.map(({ icon: Icon, number, title, text }) => (
                <article className="feature" key={number}>
                  <div className="feature__top"><span className="feature__icon"><Icon size={19} strokeWidth={1.8} /></span><span className="feature__number">{number}</span></div>
                  <h3>{title}</h3><p>{text}</p>
                  <a href="#product" aria-label={`Explore ${title}`}><ChevronRight size={17} /></a>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section className="section workflow-section" id="solutions">
          <div className="page-width workflow-layout">
            <div className="workflow-copy">
              <span className="section-kicker">A BETTER WAY THROUGH THE DAY</span>
              <h2>From first request<br />to <em>all wrapped up.</em></h2>
              <p>Less chasing updates. Fewer details slipping through. Just a shared, straightforward path from the first hello to a job well done.</p>
              <a className="text-link" href="#product">See the workspace <ArrowRight size={16} /></a>
            </div>
            <div className="steps-list">
              {steps.map((step, index) => (
                <article className="step" key={step.number}>
                  <div className="step__number">{step.number}</div>
                  <div className="step__content"><span>STEP {index + 1}</span><h3>{step.title}</h3><p>{step.text}</p></div>
                  <ChevronRight className="step__arrow" size={19} />
                </article>
              ))}
              <div className="step-note"><Headphones size={17} /><span>One connected service flow, shaped around your team.</span></div>
            </div>
          </div>
        </section>

        <section className="product-section" id="product">
          <div className="page-width product-layout">
            <div className="product-copy">
              <span className="section-kicker">THE WHOLE PICTURE, AT A GLANCE</span>
              <h2>Make room for<br /><em>great service.</em></h2>
              <p>See the shape of your day, spot what needs attention, and keep every handoff connected. The right details, right where your team needs them.</p>
              <ul className="product-checks"><li><Check size={15} /> Work stays visible from start to finish</li><li><Check size={15} /> Team capacity is easier to understand</li><li><Check size={15} /> Customer context travels with the work</li></ul>
              <Link className="button button--dark" to="/register">Get early access <ArrowRight size={16} /></Link>
            </div>
            <div className="product-visual"><div className="product-visual__caption"><span>WORKSPACE PREVIEW</span><span><i /> ILLUSTRATIVE SAMPLE DATA</span></div><DashboardPreview variant="showcase" /></div>
          </div>
        </section>

        <section className="section proof-section" aria-labelledby="proof-heading">
          <div className="page-width">
            <div className="section-heading section-heading--center"><span className="section-kicker">A NOTE ON WHAT YOU SEE</span><h2 id="proof-heading">Built for service teams.<br /><em>Still finding its first ones.</em></h2><p>ServeFlow is at the beginning. These cards are design placeholders, not customer feedback.</p></div>
            <div className="quote-grid">
              {[1, 2, 3].map((item) => (
                <article className="quote-card" key={item}>
                  <span className="quote-card__label">PLACEHOLDER · NOT A CUSTOMER TESTIMONIAL</span>
                  <div className="quote-card__marks" aria-hidden="true">“ ”</div>
                  <p>Illustrative quote will appear here once a service team has shared their experience with ServeFlow.</p>
                  <div className="quote-card__person"><span>{String(item).padStart(2, '0')}</span><div><strong>Future customer story</strong><small>Placeholder profile</small></div></div>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section className="pricing-strip" id="pricing">
          <div className="page-width pricing-strip__inner"><div><span className="section-kicker">A GOOD THING IS TAKING SHAPE</span><h2>Simple service software.<br /><em>Thoughtful plans to come.</em></h2></div><p>We&apos;re building ServeFlow with service teams in mind. Plan details will be shared as the product takes shape.</p></div>
        </section>

        <section className="cta-section">
          <div className="cta-section__pattern" aria-hidden="true" />
          <div className="page-width cta-section__inner"><span className="cta-orbit" aria-hidden="true"><Wrench size={23} /></span><span className="section-kicker">YOUR NEXT SERVICE DAY, WITH MORE FLOW</span><h2>Ready to simplify your<br /><em>service operations?</em></h2><p>Give your team one clear place to keep good work moving.</p><Link className="button button--light" to="/register">Get started <ArrowRight size={16} /></Link><span className="cta-footnote">No credit card required for early access</span></div>
        </section>
      </main>

      <footer className="site-footer">
        <div className="page-width">
          <div className="footer-main"><div className="footer-brand"><Logo light /><p>Service work has a lot of moving parts.<br />ServeFlow helps them move together.</p></div>
            <FooterLinks title="Product" links={[["Features", "#features"], ["Solutions", "#solutions"], ["Pricing", "#pricing"], ["Get started", "/register"]]} />
            <FooterLinks title="Company" links={[["About", "#about"], ["Contact", "mailto:hello@serveflow.example"]]} />
            <FooterLinks title="Resources" links={[["Product preview", "#product"], ["Service operations", "#solutions"], ["Log in", "/login"]]} />
          </div>
          <div className="footer-bottom"><span>© {new Date().getFullYear()} ServeFlow. Made for the work that keeps things working.</span><span className="footer-bottom__status"><i /> Early access is in development</span></div>
        </div>
      </footer>
    </>
  )
}

function FooterLinks({ title, links }) {
  return (
    <div className="footer-links"><h3>{title}</h3>{links.map(([label, href]) => (
      href.startsWith('/') ? <Link key={label} to={href}>{label}</Link> : <a href={href} key={label}>{label}</a>
    ))}</div>
  )
}