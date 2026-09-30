import AuthForm from '../components/layout/AuthForm'
import AuthLayout from '../components/layout/AuthLayout'

export default function Login() {
  return <AuthLayout><AuthForm mode="login" /></AuthLayout>
}