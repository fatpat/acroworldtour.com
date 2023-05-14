import { useRouter } from 'next/router'
import { useState, useEffect } from "react"

function getStorageValue(key, defaultValue) {
  if (typeof(window) === "undefined")  return defaultValue
  const saved = localStorage.getItem(key)
  const initial = JSON.parse(saved)
  return initial || defaultValue
}

export const useAuth = (props) => {
  const [user, setUser] = useState(() => {
    return getStorageValue('user', {})
  })

  useEffect(() => {
    localStorage.setItem('user', JSON.stringify(user))
  }, [user])

  return [user, setUser]
}
