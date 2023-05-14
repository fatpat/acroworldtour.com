// ** react
import { useState, useEffect } from 'react'

// ** next
import Link from 'next/link'

// ** mui
import Grid from '@mui/material/Grid'
import Button from '@mui/material/Button'
import Typography from '@mui/material/Typography'
import Modal from '@mui/material/Modal'
import Card from '@mui/material/Card'
import CardHeader from '@mui/material/CardHeader'
import CardContent from '@mui/material/CardContent'
import TextField from '@mui/material/TextField'
import SendIcon from '@mui/icons-material/Send';

// ** local
import { useNotifications } from 'src/util/notifications'
import { APIRequest } from 'src/util/backend'

const modalStyle = {
  position: 'absolute',
  top: '50%',
  left: '50%',
  transform: 'translate(-50%, -50%)',
  width: '600px',
  bgcolor: 'background.paper',

  boxShadow: 24,
  p: 4
}

const Login = () => {
  const [success, info, warning, error] = useNotifications()

  useEffect(() => {}, [])

  const loginSubmit = async (event) => {
    event.preventDefault()
    const form = new FormData(event.target)
    var params = new URLSearchParams(data)
    params.append('grand__type', 'password')
    params.append('username', form.get('username'))
    params.append('password', form.get('password'))
    const [err, data, headers] = await APIRequest('/auth/login', {method: "POST", body: params, expect_json: true})

    if (err) {
        error(`Error while authenticate: ${err}`)
        return
    }

    if (!data.access_token) {
        error("No access token received :-(")
        return
    }
    success("Authentication successfull", form.get('username'))
    localStorage.setItem('token', data.access_token)

    const queryParams = new URLSearchParams(window.location.search);
    const returnTo = queryParams.get('returnTo');
    window.location = returnTo || `${process.env.NEXT_PUBLIC_APP_BASE_PATH}/`
  }

  return (
    <Grid container spacing={6}>
      <Grid item xs={12}>
          <Card>
            <form onSubmit={loginSubmit}>
              <CardHeader
                title="Login"
                titleTypographyProps={{ variant: 'h6' }}
              />
              <CardContent>
                <Grid container spacing={5}>
                  <Grid item xs={12}>
                    <TextField fullWidth name="username" label='Username' placeholder='Username' />
                  </Grid>
                  <Grid item xs={12}>
                    <TextField fullWidth type="password" name="password" label='Password' placeholder='Password' />
                  </Grid>
                  <Grid item xs={12}>
                    <Button type="submit" variant='contained' endIcon={<SendIcon />}>Login</Button>
                  </Grid>
                </Grid>
              </CardContent>
            </form>
          </Card>
      </Grid>
    </Grid>
  )
}

export default Login
