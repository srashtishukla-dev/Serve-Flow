export default function Card({ children, className = '' }) {
  return <section className={`auth-card ${className}`.trim()}>{children}</section>
}