// ** react
import { useState, useEffect } from 'react';

// ** next
import Router from 'next/router'

// ** auth
import { withPageAuthRequired, useUser } from '@auth0/nextjs-auth0';

// ** MUI Imports
import Grid from '@mui/material/Grid'
import Typography from '@mui/material/Typography'
import Button from '@mui/material/Button'
import Modal from '@mui/material/Modal'
import RefreshIcon from '@mui/icons-material/Refresh'
import TextField from '@mui/material/TextField'
import CircularProgress from '@mui/material/CircularProgress';
import LinearProgress from '@mui/material/LinearProgress';
import Box from '@mui/material/Box';
import Card from '@mui/material/Card';
import Link from '@mui/material/Link'
import AddIcon from '@mui/icons-material/Add'
import CardHeader from '@mui/material/CardHeader'
import CardContent from '@mui/material/CardContent'
import FormControl from '@mui/material/FormControl'
import InputLabel from '@mui/material/InputLabel'
import Select from '@mui/material/Select'
import MenuItem from '@mui/material/MenuItem'
import CardActions from '@mui/material/CardActions'
import Breadcrumbs from '@mui/material/Breadcrumbs'
import Autocomplete from '@mui/material/Autocomplete';
import Avatar from '@mui/material/Avatar';

// ** local
import EnhancedTable from 'src/views/tables/EnhancedTable'
import CardPilot from 'src/views/cards/CardPilot'
import { countryListAllIsoData } from 'src/util/countries'
import { useNotifications } from 'src/util/notifications'
import { APIRequest } from 'src/util/backend'
import modalStyle from 'src/configs/modalStyle'

const TeamsPage = () => {
  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** auth/user
  const { user, authError, authIisLoading } = useUser();

  // ** local
  const [data, setData] = useState([])
  const [fullData, setFullData] = useState([])
  const [pilots, setPilots] = useState([])
  const [isLoading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [modalTitle, setModalTitle] = useState('')
  const [newTeam, setNewTeam] = useState({})

  const loadTeams = async () => {
    setLoading(true)

    const [err, data, headers] = await APIRequest('/teams', {expect_json: true})

    if (err) {
        setData(false)
        setFullData(false)
        error(`Error while retrieving teams list: ${err}`)
        return
    }

    data = data.map(j => {
      j.delete = 'delete'
      j.update = 'update'
      j.id = j._id
      return j
    })

    setData(data)
    setFullData(data)
    setLoading(false)
  }

  const loadPilots = async () => {
    const [err, data, headers] = await APIRequest('/pilots', {expect_json: true})

    if (err) {
        setPilots([])
        error(`Error while retrieving judge levels: ${err}`)
        return
    }
    setPilots(data)
  }


  const createOrUpdateTeam = async(event) => {
    event.preventDefault()

    var route = '/teams/new'
    var method = 'POST'
    var expected_status = 201

    if (newTeam._id) {
      route = `/teams/${newTeam._id}`
      method = 'PUT'
      expected_status = 204
    }

    newTeam.pilots = newTeam.pilots ?? []
    newTeam.pilots = newTeam.pilots.map(p => p.civlid)

    const [err, data, headers] = await APIRequest(route, {
      expected_status: expected_status,
      method: method,
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(newTeam),
    })

    if (err) {
        if (newTeam._id) {
            error(`error while updating team ${newTeam._id}: ${err}`)
        } else {
            error(`error while creating new team: ${err}`)
        }
        return
    }

    setModalOpen(false)
    loadTeams()
  }

  const deleteTeam = async (id) => {
      if (!confirm(`Are you sure you want to delete Team ${name} (${id}) ?`)) return

      setLoading(true)
    const [err, data, headers] = await APIRequest(`/teams/${id}`, {method: "DELETE", expected_status: 204})
    if (err) {
      error(`Error while deleting Team ${id}: ${err}`)
    } else {
      success(`Team ${id} successfully deleted`)
    }
    loadTeams()
  }

  const openCreateModal = () => {
    setModalTitle('New team')
    setNewTeam({})
    setModalOpen(true)
  }

  const openUpdateModal = (id) => {
    const team = data.find(j => j._id == id)
    setModalTitle(`Updating team ${id}`)
    setNewTeam(team)
    setModalOpen(true)
  }

  const updateSearch = async(e) => {
    const s = e.target.value
    // https://stackoverflow.com/questions/990904/remove-accents-diacritics-in-a-string-in-javascript
    // to compare ignoring accents
    s = s.normalize("NFD").replace(/[\u0300-\u036f]/g, "")
    const r = new RegExp(s, "i");
    const d = fullData.filter(team => team.name.normalize("NFD").replace(/[\u0300-\u036f]/g, "").match(r))
    setData(d)
    info(`${d.length} teams filtered over ${fullData.length}`)
  }

  const headCells = [
    {
      id: 'name',
    },
    {
      id: 'pilots',
      rewrite: (v) => {
        return (
          <Box>
            <Link href={v[0].link} target="_blank" rel="noopener noreferrer"><Avatar alt={v[0].name} src={v[0].photo} />{v[0].name}</Link>
            <Link href={v[0].link} target="_blank" rel="noopener noreferrer"><Avatar alt={v[1].name} src={v[1].photo} />{v[1].name}</Link>
          </Box>
        )
      },
    },
    {
      id: 'update',
      type: 'ACTION',
      func: openUpdateModal,
    },
    {
      id: 'delete',
      type: 'ACTION',
      func: deleteTeam,
    }
  ]

  useEffect(() => {
      loadTeams()
      loadPilots()
  }, [])

  if (isLoading) {
    return (
      <Box sx={{ width: '100%', textAlign: 'center' }}>
        <LinearProgress />
        Loading
      </Box>
    )
  }

  if (!data) {
    error("Empty or invalid data")
    return ''
  }

  return (
    <Grid container spacing={6}>
      <Grid item xs={12}>
        <Typography variant='h5'>Teams<RefreshIcon onClick={loadTeams} /></Typography>
      </Grid>
      <Grid item xs={4} sm={4}>
        <TextField fullWidth id='outlined-basic' label='Search team' variant='outlined' onChange={updateSearch} />
      </Grid>
      <Grid item xs={8} sm={8} container>
        <Button
          variant='contained'
          onClick={openCreateModal}
          startIcon={<AddIcon />}
        >new team</Button>
        <Modal
          open={modalOpen}
          onClose={() => setModalOpen(false)}
          aria-labelledby='modal-modal-title'
          aria-describedby='modal-modal-description'
        >
          <Card sx={modalStyle}>
            <form onSubmit={createOrUpdateTeam}>
              <CardHeader
                title={modalTitle}
                titleTypographyProps={{ variant: 'h6' }}
              />
              <CardContent>
                <Grid container spacing={5}>
                  <Grid item xs={12}>
                    <TextField
                      fullWidth name="name" label='Name' placeholder='Team name' defaultValue={newTeam.name ?? ""}
                      onChange={(e) => {
                        newTeam.name = e.target.value
                        setNewTeam(newTeam)
                      }}
                    />
                  </Grid>
                  <Grid item xs={12}>
                    <Autocomplete
                      disablePortal
                      multiple
                      id="autocomplete-pilots"
                      options={pilots}
                      getOptionLabel={(p) => `${p.name} (${p.civlid})`}
                      defaultValue={pilots.filter(p => {
                          if (!newTeam.pilots) return false
                          return newTeam.pilots.find((p2) => p2.civlid == p.civlid)
                      })}
                      renderInput={(params) => <TextField {...params} name="pilots" label="Pilots" />}
                      onChange={(e, v) => {
                        newTeam.pilots = v
                        setNewTeam(newTeam)
                      }}
                    />
                  </Grid>
                </Grid>
              </CardContent>
              <CardActions>
                <Button size='large' type='submit' sx={{ mr: 2 }} variant='contained'>
                  Submit
                </Button>
                <Button size='large' color='secondary' variant='outlined' onClick={() => setModalOpen(false)}>
                  Cancel
                </Button>
              </CardActions>
            </form>
          </Card>
        </Modal>
      </Grid>
      <Grid item xs={12}>
        <Card>
          <EnhancedTable rows={data} headCells={headCells} orderById='name' pagination={false}/>
        </Card>
      </Grid>
    </Grid>
  )
}

export default withPageAuthRequired(TeamsPage)
