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
import AddIcon from '@mui/icons-material/Add'
import CardHeader from '@mui/material/CardHeader'
import CardContent from '@mui/material/CardContent'
import FormControl from '@mui/material/FormControl'
import FormGroup from '@mui/material/FormGroup';
import FormControlLabel from '@mui/material/FormControlLabel';
import Checkbox from '@mui/material/Checkbox';
import InputLabel from '@mui/material/InputLabel'
import Select from '@mui/material/Select'
import MenuItem from '@mui/material/MenuItem'
import CardActions from '@mui/material/CardActions'
import Breadcrumbs from '@mui/material/Breadcrumbs'
import Autocomplete from '@mui/material/Autocomplete';
import Avatar from '@mui/material/Avatar';
import { DatePicker } from '@mui/x-date-pickers/DatePicker'
import { AdapterDateFns } from '@mui/x-date-pickers/AdapterDateFns'
import { LocalizationProvider } from '@mui/x-date-pickers/LocalizationProvider'

// ** local
import EnhancedTable from 'src/views/tables/EnhancedTable'
import CardPilot from 'src/views/cards/CardPilot'
import { countryListAllIsoData } from 'src/util/countries'
import { useNotifications } from 'src/util/notifications'
import { APIRequest } from 'src/util/backend'
import modalStyle from 'src/configs/modalStyle'
import ResponsiveDatePicker from 'src/components/ResponsiveDatePicker'

const emptyEvent = {
  year: (new Date()).getFullYear()
}

const EventsPage = () => {
  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** auth/user
  const { user, authError, authIisLoading } = useUser();

  // ** local
  const [data, setData] = useState([])
  const [fullData, setFullData] = useState([])
  const [isLoading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [modalTitle, setModalTitle] = useState('')
  const [newEvent, setNewEvent] = useState(emptyEvent)

  const loadEvents = async () => {
    setLoading(true)

    const [err, data, headers] = await APIRequest('/events', {expect_json: true})

    if (err) {
        setData(false)
        setFullData(false)
        error(`Error while retrieving competitions list: ${err}`)
        return
    }

    data = data.map(j => {
      j.delete = 'delete'
      j.update = 'update'
      j.number_of_competitions = (j.competitions ?? []).length
      j.id = j._id
      return j
    })

    setData(data)
    setFullData(data)
    setLoading(false)
  }

  const createOrUpdateEvent = async(event) => {
    event.preventDefault()

    const [err, data, headers] = await APIRequest(`/events/new`, {
      expected_status: 201,
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(newEvent),
    })

    if (err) {
        error(`error while creating new event: ${err}`)
        return
    }

    setModalOpen(false)
    loadEvents()
  }

  const openCreateModal = () => {
    setModalTitle('New Event')
    setNewEvent(emptyEvent)
    setModalOpen(true)
  }

  const updateSearch = async(e) => {
    const s = e.target.value
    // https://stackoverflow.com/questions/990904/remove-accents-diacritics-in-a-string-in-javascript
    // to compare ignoring accents
    s = s.normalize("NFD").replace(/[\u0300-\u036f]/g, "")
    const r = new RegExp(s, "i");
    const d = fullData.filter(competition => competition.name.normalize("NFD").replace(/[\u0300-\u036f]/g, "").match(r))
    setData(d)
    info(`${d.length} competitions filtered over ${fullData.length}`)
  }

  var headCells = [
    {
      id: 'name',
      type: 'LINK',
      href: (v, event) => `/events/show?eid=${event.code}`,
    },
    {
      id: 'code',
    },
    {
      id: 'start_date',
    },
    {
      id: 'end_date',
    },
    {
      id: 'location',
    },
    {
      id: 'country',
    },
    {
      id: 'number_of_competitions',
    },
  ]

  useEffect(() => {
      loadEvents()
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
        <Typography variant='h5'>Events<RefreshIcon onClick={loadEvents} /></Typography>
      </Grid>
      <Grid item xs={4} sm={4}>
        <TextField fullWidth id='outlined-basic' label='Search events' variant='outlined' onChange={updateSearch} />
      </Grid>
      <Grid item xs={8} sm={8} container>
        <Button
          variant='contained'
          onClick={openCreateModal}
          startIcon={<AddIcon />}
        >New Event</Button>
        <Modal
          open={modalOpen}
          onClose={() => setModalOpen(false)}
          aria-labelledby='modal-modal-title'
          aria-describedby='modal-modal-description'
        >
          <Card sx={modalStyle}>
            <form onSubmit={createOrUpdateEvent}>
              <CardHeader
                title={modalTitle}
                titleTypographyProps={{ variant: 'h6' }}
              />
              <CardContent>
                <Grid container spacing={5}>
                  <Grid item xs={12}>
                    <TextField
                      fullWidth name="name" label='Name' placeholder='Event name' defaultValue={newEvent.name ?? ""}
                      onChange={(e) => {
                        newEvent.name = e.target.value
                        setNewEvent(newEvent)
                      }}
                    />
                  </Grid>
                  <Grid item xs={3}>
                    <TextField
                      fullWidth name="code" label='code' placeholder='Code' defaultValue={newEvent.code ?? ""}
                      onChange={(e) => {
                        newEvent.code = e.target.value
                        setNewEvent(newEvent)
                      }}
                    />
                  </Grid>
                  <Grid item xs={3}>
                    <ResponsiveDatePicker
                      views={['year']}
                      label="Year"
                      default={new Date(newEvent.year, 1, 1)}
                      onChange={(v) => {
                        newEvent.year = parseInt(v.getFullYear())
                        setNewEvent(newEvent)
                      }}
                    />
                  </Grid>
                  <Grid item xs={3}>
                    <Autocomplete
                      disablePortal
                      id="autocomplete-country"
                      options={countryListAllIsoData}
                      getOptionLabel={c => `${c.name} (${c.code3})`}
                      defaultValue={countryListAllIsoData.find(c => c.code3.toLowerCase() == newEvent.country)}
                      renderInput={(params) => <TextField {...params} name="country" label="Country" />}
                      onChange={(e, v) => {
                        newEvent.country = v.code3.toLowerCase()
                        setNewEvent(newEvent)
                      }}
                    />
                  </Grid>
                  <Grid item xs={3}>
                    <TextField
                      fullWidth name="index" label='index' placeholder='Index' defaultValue={newEvent.index ?? 999}
                      onChange={(e) => {
                        newEvent.index = e.target.value
                        setNewEvent(newEvent)
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
      { [...new Set(data.map(s => new Date(s.start_date).getFullYear()))].sort((a,b) => b-a).map(year => {
        return(
          <Grid key={year} item xs={12}>
            <h2>{year}</h2>
            <Card>
              <EnhancedTable rows={data.filter(d => new Date(d.start_date).getFullYear() == year)} headCells={headCells} orderById='index' defaultOrder='asc' pagination={false}/>
            </Card>
          </Grid>
        )
      })}
    </Grid>
  )
}

export default EventsPage
