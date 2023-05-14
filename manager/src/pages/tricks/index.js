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
import Checkbox from '@mui/material/Checkbox'
import FormControlLabel from '@mui/material/FormControlLabel';

// ** local
import EnhancedTable from 'src/views/tables/EnhancedTable'
import CardPilot from 'src/views/cards/CardPilot'
import { countryListAllIsoData } from 'src/util/countries'
import { useNotifications } from 'src/util/notifications'
import { APIRequest } from 'src/util/backend'
import modalStyle from 'src/configs/modalStyle'

const TricksPage = () => {
  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** auth/user
  const { user, authError, authIisLoading } = useUser();

  // ** local
  const [data, setData] = useState([])
  const [fullData, setFullData] = useState([])
  const [bonuses, setBonuses] = useState([])
  const [directions, setDirections] = useState([])
  const [isLoading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [modalTitle, setModalTitle] = useState('')
  const [newTrick, setNewTrick] = useState({})

  const loadTricks = async () => {
    setLoading(true)

    const [err, data, headers] = await APIRequest('/tricks', {expect_json: true})

    if (err) {
        setData(false)
        setFullData(false)
        error(`Error while retrieving tricks list: ${err}`)
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

  const loadBonuses = async () => {
    const [err, data, headers] = await APIRequest('/tricks/bonuses', {expect_json: true})

    if (err) {
        setBonuses([])
        error(`Error while retrieving trick available bonusess: ${err}`)
        return
    }
    setBonuses(data)
  }

  const loadDirections = async () => {
    const [err, data, headers] = await APIRequest('/tricks/directions', {expect_json: true})

    if (err) {
        setDirections([])
        error(`Error while retrieving trick available directions: ${err}`)
        return
    }
    data = data.map(d => d.name)
    setDirections(data)
  }

  const createOrUpdateTrick = async(event) => {
    event.preventDefault()

    var route = '/tricks/new'
    var method = 'POST'
    var expected_status = 201

    if (newTrick._id) {
      route = `/tricks/${newTrick._id}`
      method = 'PUT'
      expected_status = 204
    }

    newTrick.directions = newTrick.directions ?? []
    newTrick.bonuses = newTrick.bonuses ?? []
    newTrick.solo = newTrick.solo ?? true
    newTrick.synchro = newTrick.synchro ?? true
    newTrick.repeatable = newTrick.repeatable ?? false

    const [err, data, headers] = await APIRequest(route, {
      expected_status: expected_status,
      method: method,
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(newTrick),
    })

    if (err) {
        if (newTrick._id) {
            error(`error while updating trick ${newTrick._id}: ${err}`)
        } else {
            error(`error while creating new trick: ${err}`)
        }
        return
    }

    setModalOpen(false)
    loadTricks()
  }

  const deleteTrick = async (id) => {
    if (!confirm(`Are you sure you want to delete Trick ${name} (${id}) ?`)) return

    setLoading(true)
    const [err, data, headers] = await APIRequest(`/tricks/${id}`, {method: "DELETE", expected_status: 204})
    if (err) {
      error(`Error while deleting Trick ${id}: ${err}`)
    } else {
      success(`Trick ${id} successfully deleted`)
    }
    loadTricks()
  }

  const openCreateModal = () => {
    setModalTitle('New trick')
    setNewTrick({})
    setModalOpen(true)
  }

  const openUpdateModal = (id) => {
    const trick = data.find(j => j._id == id)
    setModalTitle(`Updating trick ${id}`)
    setNewTrick(trick)
    setModalOpen(true)
  }

  const updateSearch = async(e) => {
    const s = e.target.value
    // https://stackoverflow.com/questions/990904/remove-accents-diacritics-in-a-string-in-javascript
    // to compare ignoring accents
    s = s.normalize("NFD").replace(/[\u0300-\u036f]/g, "")
    const r = new RegExp(s, "i");
    var d = fullData.filter(trick => trick.name.normalize("NFD").replace(/[\u0300-\u036f]/g, "").match(r) || trick.acronym.match(r))
    setData(d)
    info(`${d.length} tricks filtered over ${fullData.length}`)
  }

  const headCells = [
    {
      id: 'name',
    },
    {
      id: 'acronym',
    },
    {
      id: 'directions',
      rewrite: (directions) => { return directions.join(', ')},
    },
    {
      id: 'technical_coefficient',
      numeric: true,
    },
    {
      id: 'solo',
      type: 'BOOLEAN',
    },
    {
      id: 'repeatable',
      type: 'BOOLEAN',
    },
    {
      id: 'bonuses',
      rewrite: (bonuses) => { return bonuses.map(b => `${b.name} (${b.bonus})`).join(', ') },
    },
    {
      id: 'synchro',
      type: 'BOOLEAN',
    },
    {
      id: 'first_maneuver',
      numeric: true,
    },
    {
      id: 'no_first_maneuver',
      numeric: true,
    },
    {
      id: 'last_maneuver',
      numeric: true,
    },
    {
      id: 'no_last_maneuver',
      numeric: true,
    },
    {
      id: 'update',
      type: 'ACTION',
      func: openUpdateModal,
    },
    {
      id: 'delete',
      type: 'ACTION',
      func: deleteTrick,
    }
  ]

  useEffect(() => {
      loadTricks()
      loadBonuses()
      loadDirections()
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
        <Typography variant='h5'>Tricks<RefreshIcon onClick={loadTricks} /></Typography>
      </Grid>
      <Grid item xs={4} sm={4}>
        <TextField fullWidth id='outlined-basic' label='Search trick' variant='outlined' onChange={updateSearch} />
      </Grid>
      <Grid item xs={8} sm={8} container>
        <Button
          variant='contained'
          onClick={openCreateModal}
          startIcon={<AddIcon />}
        >new trick</Button>
        <Modal
          open={modalOpen}
          onClose={() => setModalOpen(false)}
          aria-labelledby='modal-modal-title'
          aria-describedby='modal-modal-description'
        >
          <Card sx={modalStyle}>
            <form onSubmit={createOrUpdateTrick}>
              <CardHeader
                title={modalTitle}
                titleTypographyProps={{ variant: 'h6' }}
              />
              <CardContent>
                <Grid container spacing={5}>
                  <Grid item xs={6}>
                    <TextField
                      fullWidth name="name" label='Name' placeholder='Trick name' defaultValue={newTrick.name ?? ""}
                      onChange={(e) => {
                        newTrick.name = e.target.value
                        setNewTrick(newTrick)
                      }}
                    />
                  </Grid>
                  <Grid item xs={6}>
                    <TextField
                      fullWidth name="acronym" label='Acronym' placeholder='Trick Acronym' defaultValue={newTrick.acronym ?? ""}
                      onChange={(e) => {
                        newTrick.acronym = e.target.value
                        setNewTrick(newTrick)
                      }}
                    />
                  </Grid>
                  <Grid item xs={6}>
                    <Autocomplete
                      disablePortal
                      id="autocomplete-directions"
                      multiple
                      options={directions}
                      defaultValue={newTrick.directions}
                      renderInput={(params) => <TextField {...params} name="directions" label="Directions" />}
                      onChange={(e, v) => {
                        newTrick.directions = v
                        setNewTrick(newTrick)
                      }}
                    />
                  </Grid>
                  <Grid item xs={6}>
                    Bonuses ##TODO##
                  </Grid>
                  <Grid item xs={4}>
                    <FormControlLabel
                      control={
                        <Checkbox
                          defaultChecked={newTrick.solo ?? true}
                          onChange={e => {
                            newTrick.solo = e.target.checked
                            setNewTrick(newTrick)
                          }}
                          />
                      }
                      label="Solo ?"
                    />
                  </Grid>
                  <Grid item xs={4}>
                    <FormControlLabel
                      control={
                        <Checkbox
                          defaultChecked={newTrick.synchro ?? true}
                          onChange={e => {
                            newTrick.synchro = e.target.checked
                            setNewTrick(newTrick)
                          }}
                          />
                      }
                      label="Synchro ?"
                    />
                  </Grid>
                  <Grid item xs={4}>
                    <FormControlLabel
                      control={
                        <Checkbox
                          defaultChecked={newTrick.repeatable ?? false}
                          onChange={e => {
                            newTrick.repeatable = e.target.checked
                            setNewTrick(newTrick)
                          }}
                          />
                      }
                      label="Repeatable ?"
                    />
                  </Grid>
                  <Grid item xs={6}>
                    <TextField
                      fullWidth name="technical_coefficient" label='Technical Coefficient' placeholder='Technical Coefficient' defaultValue={newTrick.technical_coefficient ?? ""}
                      onChange={(e) => {
                        newTrick.technical_coefficient = e.target.value
                        setNewTrick(newTrick)
                      }}
                    />
                  </Grid>
                  <Grid item xs={6}>
                    free space
                  </Grid>
                  <Grid item xs={3}>
                    <TextField
                      fullWidth name="first_maneuver" label='First Maneuver' type="Number" defaultValue={newTrick.first_maneuver ?? 0} 
                      onChange={(e) => {
                        newTrick.first_maneuver = e.target.value
                        setNewTrick(newTrick)
                      }}
                    />
                  </Grid>
                  <Grid item xs={3}>
                    <TextField
                      fullWidth name="no_first_maneuver" label='No First Maneuver' type="Number" defaultValue={newTrick.no_first_maneuver ?? 0} 
                      onChange={(e) => {
                        newTrick.no_first_maneuver = e.target.value
                        setNewTrick(newTrick)
                      }}
                    />
                  </Grid>
                  <Grid item xs={3}>
                    <TextField
                      fullWidth name="last_maneuver" label='Last Maneuver' type="Number" defaultValue={newTrick.last_maneuver ?? 0} 
                      onChange={(e) => {
                        newTrick.last_maneuver = e.target.value
                        setNewTrick(newTrick)
                      }}
                    />
                  </Grid>
                  <Grid item xs={3}>
                    <TextField
                      fullWidth name="no_last_maneuver" label='No Last Maneuver' type="Number" defaultValue={newTrick.no_last_manoeuver ?? 0} 
                      onChange={(e) => {
                        newTrick.no_last_maneuver = e.target.value
                        setNewTrick(newTrick)
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
          <EnhancedTable rows={data} headCells={headCells} orderById='technical_coefficient' pagination={false}/>
        </Card>
      </Grid>
    </Grid>
  )
}

export default withPageAuthRequired(TricksPage)
