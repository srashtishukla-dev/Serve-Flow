export default function Button({ children, className = '', ...props }) {
  return (
    <button className={`button button--primary ${className}`.trim()} {...props}>
      {children}
    </button>
  )
}