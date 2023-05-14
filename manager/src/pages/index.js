// ** auth
import { withPageAuthRequired, useUser } from '@auth0/nextjs-auth0';

// ** react
import { useState, useEffect } from 'react'

// ** next
import Link from 'next/link'

const Login = () => {

  const { user, error, isLoading, checkSession } = useUser();

  useEffect(() => {
  }, [])

  if (isLoading) return <div>Loading...</div>
  if (error) return <div>{error.message}</div>

  if (user) {
    return (
      <div>
        Welcome {user.name}!
      </div>
    )
  }

  return 'internal error :('
}

export default withPageAuthRequired(Login)
