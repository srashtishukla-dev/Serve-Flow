import { Link } from 'react-router-dom'

export default function Logo({ light = false }) {
  return (
    <Link
      aria-label="ServeFlow home"
      className={`brand${light ? ' brand--light' : ''}`}
      to="/"
    >
      <svg aria-hidden="true" className="brand__mark" viewBox="0 0 48 48" fill="none">
        <rect width="48" height="48" rx="14" fill="currentColor" />
        <path
          d="M12 17.5h13.5a6.5 6.5 0 0 1 0 13H21a4 4 0 0 1 0-8h13"
          stroke="white"
          strokeWidth="4"
          strokeLinecap="round"
        />
        <path
          d="m30 16 5 6.5-5 6.5"
          stroke="#D7CBFF"
          strokeWidth="4"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
      <span className="brand__name">Serve<span>Flow</span></span>
    </Link>
  )
}