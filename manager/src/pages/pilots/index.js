// ** react
import { useState, useEffect } from 'react';

// ** nextjs
import Router from 'next/router'

// ** auth
import { withPageAuthRequired, useUser } from '@auth0/nextjs-auth0';

// ** MUI Imports
import Grid from '@mui/material/Grid'
import Typography from '@mui/material/Typography'
import Button from '@mui/material/Button'
import RefreshIcon from '@mui/icons-material/Refresh'
import TextField from '@mui/material/TextField'
import CircularProgress from '@mui/material/CircularProgress';
import LinearProgress from '@mui/material/LinearProgress';
import Box from '@mui/material/Box';

// ** internal
import CardPilot from 'src/views/cards/CardPilot'
import { useNotifications } from 'src/util/notifications'
import { APIRequest } from 'src/util/backend'

const PilotsPage = () => {
  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** auth/user
  const { user, authError, authIisLoading } = useUser();

  // ** local
  const [data, setData] = useState([])
  const [fullData, setFullData] = useState([])
  const [isLoading, setLoading] = useState(false)

  const loadPilots = async () => {
    setLoading('Loading pilots list')

    const [err, data, headers] = await APIRequest('/pilots/', {expect_json: true})

    if (err) {
        setData(false)
        setFullData(false)
        error(`Error while retrieving pilots list: ${err}`)
        return
    }

    setData(data)
    setFullData(data)
    setLoading(false)
  }

  const handleSubmitUpdatePilot = (e) => {
      event.preventDefault()
      updatePilot(parseInt(document.getElementById('civlid').value))
  }

  const updatePilot = async (civlid) => {
      if (civlid < 1 || isNaN(civlid)) return

      setLoading(`Updating pilot #${civlid}`)

      const [err, retData, headers] = await APIRequest(`/pilots/${civlid}`, {method: 'POST', expected_status: 201})
      if (err) return error(`Error updating Pilot #${civlid}: ${err}`)

      success(`Pilot #${civlid} successfully updated`)
      setData(data.map(p => p.civlid == civlid ? retData : p))
      setFullData(fullData.map(p => p.civlid == civlid ? retData : p))
      setLoading(null)
  }

  const changeGender = async(civlid) => {
      if (civlid < 1 || isNaN(civlid)) return
      setLoading(`Chaging gender of pilot #${civlid}`)
      const [err, retData, headers] = await APIRequest(`/pilots/${civlid}/gender`, {method: 'PATCH', expected_status: 200})
      if (err) {
          return error(`Error changing gender of pilot #${civlid}: ${err}`)
      }

      setData(data.map(p => p.civlid == civlid ? retData : p))
      setFullData(fullData.map(p => p.civlid == civlid ? retData : p))
      setLoading(null)

      success(`Pilot #${civlid} gender changed to ${retData.gender}`)

  }

  const updateAllPilots = async () => {

      if (!confirm('Are you sure to update all pilots ?')) return;

      setLoading(`Updating all pilots`)

      const [err, data, headers] = await APIRequest(`/pilots/update_all`, {method: 'POST', expected_status: 201})
      if (err) {
          error(`Error updating all pilots: ${err}`)
      } else {
          success(`Pilots successfully updated`)
      }

      loadPilots()
  }

  const updateRankings = async () => {
    setLoading('Updating rankings')
    const [err, data, headers] = await APIRequest('/pilots/update_rankings', {method: 'POST', expected_status: 201})
    if (err) {
        error(`Error updating rankings: ${err}`)
    } else {
        success(`Rankings successfully updated`)
    }

    loadPilots()
  }

  const updateSearch = async(e) => {
    const s = e.target.value
    const civlid = parseInt(s)
    if (civlid > 0) {
      setData(fullData.filter(pilot => pilot.civlid == civlid))
      return
    }
    // https://stackoverflow.com/questions/990904/remove-accents-diacritics-in-a-string-in-javascript
    // to compare ignoring accents
    s = s.normalize("NFD").replace(/[\u0300-\u036f]/g, "")
    const r = new RegExp(s, "i");
    const d = fullData.filter(pilot => pilot.name.normalize("NFD").replace(/[\u0300-\u036f]/g, "").match(r))
    setData(d)
//    info(`${d.length} pilots filtered over ${fullData.length}`)
  }

  useEffect(() => {
      loadPilots()
  }, [])

  if (isLoading) {
    return (
      <Box sx={{ width: '100%', textAlign: 'center' }}>
        <LinearProgress />
        { isLoading }
      </Box>
    )
  }

  if (!data) return <p>No data</p>

  return (
    <Grid container spacing={6}>
      <Grid item xs={12} sx={{ paddingBottom: 4 }}>
        <Typography variant='h5'>Pilots<RefreshIcon onClick={loadPilots} /></Typography>
      </Grid>
      <Grid item xs={8} sm={8}>
        <TextField fullWidth id='outlined-basic' label='Filter pilots' variant='outlined' onChange={updateSearch} />
      </Grid>
      <Grid item xs={4} sm={4}>
        <form onSubmit={handleSubmitUpdatePilot}>
          <TextField id='civlid' label='CIVL ID' />
          <Button type="submit">Add or Update pilot from CIVL</Button>
        </form>
      </Grid>
{/* TODO: reenable when fixed from backend (broken because of CIVL shit)
      <Grid item xs={4} sm={4} container direction='row' justifyContent='flex-end'>
        <Button variant='outlined' startIcon={<RefreshIcon />} onClick={updateRankings} disabled>Update rankings</Button>
        <Button variant='outlined' startIcon={<RefreshIcon />} onClick={updateAllPilots} disabled>Synchronize from CIVL</Button>
      </Grid>
*/}
      {data.map(p => (
        <Grid item xs={12} sm={4} key={p.civlid}>
          <CardPilot pilot={p} updatePilot={updatePilot} changeGender={changeGender}/>
        </Grid>
      ))}
    </Grid>
  )
}

export default withPageAuthRequired(PilotsPage)
