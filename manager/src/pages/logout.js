// ** react
import { useEffect } from 'react'

// ** local
import { useNotifications } from 'src/util/notifications'

const Logout = () => {
  const [success, info, warning, error] = useNotifications()

  useEffect(() => {
    localStorage.removeItem('token')
    success('Logout successfull')
    window.location = '/login'
  }, [])

  return 'logout'
}

export default Logout
