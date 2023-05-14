// ** auth
import { withPageAuthRequired, useUser } from '@auth0/nextjs-auth0';

// ** react
import { useState, useEffect } from 'react'

// ** mui
import Link from '@mui/material/Link'
import Box from '@mui/material/Box'
import List from '@mui/material/List'
import ListItem from '@mui/material/ListItem'
import ListItemButton from '@mui/material/ListItemButton'
import ListItemIcon from '@mui/material/ListItemIcon'
import ListItemText from '@mui/material/ListItemText'

// ** internal
import { useNotifications } from 'src/util/notifications'
import { APIRequest } from 'src/util/backend'

const Utils = () => {
  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  const { user, authError, authIsLoading, checkSession } = useUser();
  const [isLoading, setLoading] = useState(false)
  const [err, setErr] = useState(null)

  const cleanupPilots = async (e) => {
    if (!confirm("Are you sure to delete unused pilots ?")) return false
    setLoading(true)

    const [err, data, headers] = await APIRequest(`/utils/cleanup_pilots`, {
      expected_status: 204,
      method: 'POST',
    })

    if (err) error(err)
    else success("Pilots cleanup successful")
    setLoading(false)
    return false
  }

  const downloadBackup = async (e) => {
    setLoading(true)

    const [err, data, headers] = await APIRequest(`/utils/backup`)

    if (err) {
      error(err)
      setLoading(false)
      return false
    }
    var filename = headers.get('content-disposition')
    if (filename) {
        var arr = filename.match(/"(.*)"/)
        filename = arr[1]
    }

    success("Download successful")
    setLoading(true)
    const a = document.querySelector('#downloadbackup')
    a.href = window.URL.createObjectURL(new Blob([data]))
    if (filename) {
        a.setAttribute('download', filename)
    }
    a.click()
    return false
  }

  useEffect(() => {
  }, [])

  return(
    <Box>
      { err && <p>Error: {err}</p>}
      <h1>Utils</h1>
      <List>
        <ListItem><Link href='#' onClick={downloadBackup}>Download Database Backup</Link><Link id="downloadbackup" /></ListItem>
        <ListItem><Link href='#' onClick={cleanupPilots}>Cleanup unused pilots</Link></ListItem>
      </List>
    </Box>
  )
}

export default withPageAuthRequired(Utils)
