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

const emptySeason = {
  year: (new Date()).getFullYear()
}

const SeasonsPage = () => {
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
  const [newSeason, setNewSeason] = useState(emptySeason)

  const loadSeasons = async () => {
    setLoading(true)

    const [err, data, headers] = await APIRequest('/seasons', {expect_json: true})

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

  const createOrUpdateSeason = async(event) => {
    event.preventDefault()

    const [err, data, headers] = await APIRequest(`/seasons/new`, {
      expected_status: 201,
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(newSeason),
    })

    if (err) {
        error(`error while creating new season: ${err}`)
        return
    }

    setModalOpen(false)
    loadSeasons()
  }

  const openCreateModal = () => {
    setModalTitle('New Season')
    setNewSeason(emptySeason)
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
      href: (v, season) => `/seasons/show?sid=${season.code}`,
    },
    {
      id: 'code',
    },
    {
      id: 'type',
    },
    {
      id: 'year',
    },
    {
      id: 'number_of_competitions',
    },
    {
      id: 'number_of_pilots',
    },
    {
      id: 'number_of_teams',
    }
  ]

  useEffect(() => {
      loadSeasons()
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
        <Typography variant='h5'>Seasons<RefreshIcon onClick={loadSeasons} /></Typography>
      </Grid>
      <Grid item xs={4} sm={4}>
        <TextField fullWidth id='outlined-basic' label='Search seasons' variant='outlined' onChange={updateSearch} />
      </Grid>
      <Grid item xs={8} sm={8} container>
        <Button
          variant='contained'
          onClick={openCreateModal}
          startIcon={<AddIcon />}
        >New Season</Button>
        <Modal
          open={modalOpen}
          onClose={() => setModalOpen(false)}
          aria-labelledby='modal-modal-title'
          aria-describedby='modal-modal-description'
        >
          <Card sx={modalStyle}>
            <form onSubmit={createOrUpdateSeason}>
              <CardHeader
                title={modalTitle}
                titleTypographyProps={{ variant: 'h6' }}
              />
              <CardContent>
                <Grid container spacing={5}>
                  <Grid item xs={12}>
                    <TextField
                      fullWidth name="name" label='Name' placeholder='Season name' defaultValue={newSeason.name ?? ""}
                      onChange={(e) => {
                        newSeason.name = e.target.value
                        setNewSeason(newSeason)
                      }}
                    />
                  </Grid>
                  <Grid item xs={6}>
                    <TextField
                      fullWidth name="code" label='code' placeholder='Code' defaultValue={newSeason.code ?? ""}
                      onChange={(e) => {
                        newSeason.code = e.target.value
                        setNewSeason(newSeason)
                      }}
                    />
                  </Grid>
                  <Grid item xs={6}>
                    <ResponsiveDatePicker
                      views={['year']}
                      label="Year"
                      default={new Date(newSeason.year, 1, 1)}
                      onChange={(v) => {
                        newSeason.year = parseInt(v.getFullYear())
                        setNewSeason(newSeason)
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
          <EnhancedTable rows={data} headCells={headCells} orderById='year' defaultOrder='desc' pagination={false}/>
        </Card>
      </Grid>
    </Grid>
  )
}

export default SeasonsPage
