// ** react
import { useEffect } from 'react'

// ** local
import { useNotifications } from 'src/util/notifications'

const Logout = () => {
  const [success, info, warning, error] = useNotifications()

  useEffect(() => {
    localStorage.removeItem('token')
    success('Logout successfull')
    window.location = `${process.env.NEXT_PUBLIC_APP_BASE_PATH}/login`
  }, [])

  return 'logout'
}

export default Logout
