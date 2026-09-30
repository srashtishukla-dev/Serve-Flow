import {
  ArrowUpRight,
  CalendarDays,
  CircleDollarSign,
  ClipboardCheck,
  Clock3,
  UsersRound,
} from 'lucide-react'

const tickets = [
  { id: 'SF-2048', title: 'HVAC inspection', customer: 'Northstar Dental', state: 'In progress', tone: 'violet' },
  { id: 'SF-2047', title: 'Washer not draining', customer: 'Maya Chen', state: 'New request', tone: 'amber' },
  { id: 'SF-2046', title: 'Quarterly maintenance', customer: 'Parkside Offices', state: 'Scheduled', tone: 'green' },
]

export default function DashboardPreview({ variant = 'hero' }) {
  return (
    <section
      aria-label="Illustrative ServeFlow dashboard using sample data"
      className={`dashboard dashboard--${variant}`}
    >
      <div className="dashboard__topbar">
        <div className="dashboard__window-dots" aria-hidden="true"><i /><i /><i /></div>
        <span className="dashboard__workspace">Good morning, Morgan <span aria-hidden="true">✳</span></span>
        <span className="demo-label">SAMPLE DATA</span>
      </div>
      <div className="dashboard__body">
        <aside className="dashboard__rail" aria-hidden="true">
          <span className="dashboard__rail-brand">S</span>
          <span className="dashboard__rail-icon dashboard__rail-icon--active"><ClipboardCheck size={16} /></span>
          <span className="dashboard__rail-icon"><UsersRound size={16} /></span>
          <span className="dashboard__rail-icon"><CalendarDays size={16} /></span>
          <span className="dashboard__rail-icon"><CircleDollarSign size={16} /></span>
        </aside>
        <div className="dashboard__content">
          <div className="dashboard__heading">
            <div>
              <span className="dashboard__eyebrow">MONDAY, OCTOBER 14</span>
              <h2>Service overview</h2>
            </div>
            <button aria-label="Sample date range: this week" className="dashboard__date" type="button">
              This week <CalendarDays size={13} />
            </button>
          </div>
          <div className="metric-grid">
            <Metric icon={ClipboardCheck} label="Open tickets" value="24" change="6 need attention" accent="purple" />
            <Metric icon={CircleDollarSign} label="Revenue" value="$8,420" change="Sample period" accent="green" />
            <Metric icon={Clock3} label="Avg. resolution" value="3.2 hrs" change="Illustrative" accent="orange" />
            <Metric icon={UsersRound} label="Customers served" value="186" change="Sample period" accent="blue" />
          </div>
          <div className="dashboard__lower">
            <div className="chart-panel">
              <div className="panel-heading">
                <div><span className="panel-heading__label">TICKET ACTIVITY</span><strong>Requests resolved</strong></div>
                <span className="chart-change"><ArrowUpRight size={13} /> 12%</span>
              </div>
              <div aria-label="Illustrative weekly activity chart" className="chart" role="img">
                {[34, 51, 42, 68, 55, 78, 62, 88, 69, 100, 76, 91].map((height, index) => (
                  <span key={index} style={{ '--bar-height': `${height}%` }} />
                ))}
              </div>
              <div className="chart__axis"><span>MON</span><span>TUE</span><span>WED</span><span>THU</span><span>FRI</span><span>SAT</span></div>
            </div>
            <div className="workload-panel">
              <div className="panel-heading"><div><span className="panel-heading__label">TEAM CAPACITY</span><strong>Today&apos;s workload</strong></div></div>
              <div className="workload-ring"><span><strong>72%</strong><small>assigned</small></span></div>
              <div className="workload-foot"><span><i /> Available</span><strong>4 techs</strong></div>
            </div>
          </div>
          <div className="ticket-panel">
            <div className="ticket-panel__heading"><strong>Recent requests</strong><span>View all <ArrowUpRight size={12} /></span></div>
            {tickets.map((ticket) => (
              <div className="ticket-row" key={ticket.id}>
                <span className="ticket-row__id">{ticket.id}</span>
                <span className="ticket-row__main"><strong>{ticket.title}</strong><small>{ticket.customer}</small></span>
                <span className={`ticket-state ticket-state--${ticket.tone}`}><i />{ticket.state}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}

function Metric({ icon: Icon, label, value, change, accent }) {
  return (
    <div className="metric">
      <div className={`metric__icon metric__icon--${accent}`}><Icon size={15} /></div>
      <span className="metric__label">{label}</span>
      <strong className="metric__value">{value}</strong>
      <small className="metric__change">{change}</small>
    </div>
  )
}