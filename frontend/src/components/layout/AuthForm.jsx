import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import apiClient from '../../services/api/client'
import Button from '../ui/Button'
import Card from '../ui/Card'
import Input from '../ui/Input'

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export default function AuthForm({ mode }) {
  const isRegister = mode === 'register'
  const { login, sessionMessage, clearSessionMessage } = useAuth()
  const navigate = useNavigate()
  const [values, setValues] = useState({ name: '', email: '', password: '', confirmPassword: '' })
  const [errors, setErrors] = useState({})
  const [notice, setNotice] = useState('')
  const [noticeType, setNoticeType] = useState('success')
  const [submitting, setSubmitting] = useState(false)

  function updateValue(event) {
    const { name, value } = event.target
    setValues((current) => ({ ...current, [name]: value }))
    setErrors((current) => ({ ...current, [name]: '' }))
    setNotice('')
    clearSessionMessage()
  }

  const displayedNotice = notice || (!isRegister ? sessionMessage : '')
  const displayedNoticeType = notice ? noticeType : 'error'

  async function handleSubmit(event) {
    event.preventDefault()
    const nextErrors = {}

    if (isRegister && !values.name.trim()) nextErrors.name = 'Enter your name.'
    if (!values.email.trim()) nextErrors.email = 'Enter your email address.'
    else if (!emailPattern.test(values.email.trim())) nextErrors.email = 'Enter a valid email address.'
    if (!values.password) nextErrors.password = 'Enter a password.'
    if (isRegister && !values.confirmPassword) nextErrors.confirmPassword = 'Confirm your password.'
    else if (isRegister && values.confirmPassword !== values.password) {
      nextErrors.confirmPassword = 'Passwords do not match.'
    }

    setErrors(nextErrors)
    if (Object.keys(nextErrors).length > 0) return

    setSubmitting(true)
    setNotice('')
    const payload = isRegister
      ? { name: values.name.trim(), email: values.email.trim(), password: values.password }
      : { email: values.email.trim(), password: values.password }

    try {
      if (!isRegister) {
        await login(payload.email, payload.password)
        navigate('/dashboard', { replace: true })
        return
      }

      const { data } = await apiClient.post('/auth/register', payload)
      setNoticeType('success')
      setNotice(data.message)
      setValues((current) => ({ ...current, password: '', confirmPassword: '' }))
    } catch (error) {
      setNoticeType('error')
      setNotice(error.response?.data?.error?.message || (
        error.response
          ? 'Unable to complete your request. Please try again.'
          : 'Cannot connect to ServeFlow. Check that the backend is running.'
      ))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Card>
      <span className="auth-card__eyebrow">SERVEFLOW ACCOUNT</span>
      <h1>{isRegister ? 'Create your account' : 'Welcome back'}</h1>
      <p className="auth-card__intro">
        {isRegister ? 'Set up your account to get started.' : 'Sign in to continue to your service workspace.'}
      </p>
      {displayedNotice && (
        <p className={`auth-notice auth-notice--${displayedNoticeType}`} role={displayedNoticeType === 'error' ? 'alert' : 'status'}>
          {displayedNotice}
        </p>
      )}
      <form className="auth-form" noValidate onSubmit={handleSubmit}>
        {isRegister && (
          <Input
            autoComplete="name"
            error={errors.name}
            id="name"
            label="Name"
            name="name"
            onChange={updateValue}
            value={values.name}
          />
        )}
        <Input
          autoComplete="email"
          error={errors.email}
          id="email"
          label="Email"
          name="email"
          onChange={updateValue}
          type="email"
          value={values.email}
        />
        <Input
          autoComplete={isRegister ? 'new-password' : 'current-password'}
          error={errors.password}
          id="password"
          label="Password"
          name="password"
          onChange={updateValue}
          type="password"
          value={values.password}
        />
        {isRegister && (
          <Input
            autoComplete="new-password"
            error={errors.confirmPassword}
            id="confirmPassword"
            label="Confirm password"
            name="confirmPassword"
            onChange={updateValue}
            type="password"
            value={values.confirmPassword}
          />
        )}
        <Button className="auth-form__submit" disabled={submitting} type="submit">
          {submitting ? 'Please wait...' : isRegister ? 'Create account' : 'Log in'}
        </Button>
      </form>
      <p className="auth-card__switch">
        {isRegister ? 'Already have an account?' : 'New to ServeFlow?'}{' '}
        <Link to={isRegister ? '/login' : '/register'}>{isRegister ? 'Log in' : 'Create an account'}</Link>
      </p>
    </Card>
  )
}