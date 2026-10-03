import { createBrowserRouter } from 'react-router-dom'
import App from '../App'
import Bookings from '../pages/Bookings'
import ComingSoon from '../pages/ComingSoon'
import Customers from '../pages/Customers'
import Dashboard from '../pages/Dashboard'
import Invoices from '../pages/Invoices'
import Landing from '../pages/Landing'
import Login from '../pages/Login'
import NotFound from '../pages/NotFound'
import Notifications from '../pages/Notifications'
import Register from '../pages/Register'
import Services from '../pages/Services'
import Technicians from '../pages/Technicians'
import RequireAuth from '../components/layout/RequireAuth'

const upcomingPaths = [
  '/tickets',
  '/settings',
]

export const router = createBrowserRouter([
  {
    element: <App />,
    children: [
      { path: '/', element: <Landing /> },
      { path: '/login', element: <Login /> },
      { path: '/register', element: <Register /> },
      { path: '/dashboard', element: <RequireAuth><Dashboard /></RequireAuth> },
      { path: '/analytics', element: <RequireAuth><Dashboard /></RequireAuth> },
      { path: '/services', element: <RequireAuth><Services /></RequireAuth> },
      { path: '/customers', element: <RequireAuth><Customers /></RequireAuth> },
      { path: '/bookings', element: <RequireAuth><Bookings /></RequireAuth> },
      { path: '/appointments', element: <RequireAuth><Bookings appointments /></RequireAuth> },
      { path: '/technicians', element: <RequireAuth><Technicians /></RequireAuth> },
      { path: '/invoices', element: <RequireAuth><Invoices /></RequireAuth> },
      { path: '/notifications', element: <RequireAuth><Notifications /></RequireAuth> },
      { path: '/404', element: <NotFound /> },
      ...upcomingPaths.map((path) => ({
        path,
        element: <ComingSoon />,
      })),
      { path: '*', element: <NotFound /> },
    ],
  },
])