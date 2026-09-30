export default function Input({ id, label, error, ...props }) {
  const errorId = error ? `${id}-error` : undefined

  return (
    <div className="form-field">
      <label htmlFor={id}>{label}</label>
      <input
        aria-describedby={errorId}
        aria-invalid={Boolean(error)}
        className={`form-input${error ? ' form-input--error' : ''}`}
        id={id}
        {...props}
      />
      {error && <span className="form-field__error" id={errorId}>{error}</span>}
    </div>
  )
}