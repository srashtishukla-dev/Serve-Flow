import { createBrowserRouter } from 'react-router-dom'
import App from '../App'
import AppointmentDetails from '../pages/AppointmentDetails'
import Analytics from '../pages/Analytics'
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

const adminRoles = ['ADMIN']

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
      { path: '/analytics', element: <RequireAuth roles={adminRoles}><Analytics /></RequireAuth> },
      { path: '/services', element: <RequireAuth roles={adminRoles}><Services /></RequireAuth> },
      { path: '/customers', element: <RequireAuth roles={adminRoles}><Customers /></RequireAuth> },
      { path: '/bookings', element: <RequireAuth roles={adminRoles}><Bookings /></RequireAuth> },
      { path: '/appointments', element: <RequireAuth><Bookings appointments /></RequireAuth> },
      { path: '/appointments/:id', element: <RequireAuth><AppointmentDetails /></RequireAuth> },
      { path: '/technicians', element: <RequireAuth roles={adminRoles}><Technicians /></RequireAuth> },
      { path: '/invoices', element: <RequireAuth roles={['ADMIN', 'CUSTOMER']}><Invoices /></RequireAuth> },
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