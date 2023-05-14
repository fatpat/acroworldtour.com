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

const CompetitionsPage = () => {
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
  const [newCompetition, setNewCompetition] = useState({})

  const loadCompetitions = async () => {
    setLoading(true)

    const [err, data, headers] = await APIRequest('/competitions', {expect_json: true})

    if (err) {
        setData(false)
        setFullData(false)
        error(`Error while retrieving competitions list: ${err}`)
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

  const createOrUpdateCompetition = async(event) => {
    event.preventDefault()

    const [err, data, headers] = await APIRequest(`/competitions/new`, {
      expected_status: 201,
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(newCompetition),
    })

    if (err) {
        error(`error while creating new competition: ${err}`)
        return
    }

    setModalOpen(false)
    loadCompetitions()
  }

  const openCreateModal = () => {
    setModalTitle('New competition')
    setNewCompetition({})
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
      href: (v, comp) => `/competitions/show?cid=${comp.code}`,
    },
    {
      id: 'type',
    },
    {
      id: 'seasons',
      rewrite: (v) => v.join(', ')
    },
    {
      id: 'state',
    },
    {
      id: 'start_date',
    },
    {
      id: 'location',
    },
    {
      id: 'number_of_runs',
    },
    {
      id: 'code',
    },
    {
      id: 'published',
      type: 'BOOLEAN',
    }
  ]

  useEffect(() => {
      loadCompetitions()
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
        <Typography variant='h5'>Competitions<RefreshIcon onClick={loadCompetitions} /></Typography>
      </Grid>
      <Grid item xs={4} sm={4}>
        <TextField fullWidth id='outlined-basic' label='Search competition' variant='outlined' onChange={updateSearch} />
      </Grid>
      <Grid item xs={8} sm={8} container>
        <Button
          variant='contained'
          onClick={openCreateModal}
          startIcon={<AddIcon />}
        >new competition</Button>
        <Modal
          open={modalOpen}
          onClose={() => setModalOpen(false)}
          aria-labelledby='modal-modal-title'
          aria-describedby='modal-modal-description'
        >
          <Card sx={modalStyle}>
            <form onSubmit={createOrUpdateCompetition}>
              <CardHeader
                title={modalTitle}
                titleTypographyProps={{ variant: 'h6' }}
              />
              <CardContent>
                <Grid container spacing={5}>
                  <Grid item xs={8}>
                    <TextField
                      fullWidth name="name" label='Name' placeholder='Competition name' defaultValue={newCompetition.name ?? ""}
                      onChange={(e) => {
                        newCompetition.name = e.target.value
                        setNewCompetition(newCompetition)
                      }}
                    />
                  </Grid>
                  <Grid item xs={4}>
                    <Autocomplete
                      disablePortal
                      id="autocomplete-type"
                      options={['solo', 'synchro']}
                      defaultValue={newCompetition.type ?? ""}
                      renderInput={(params) => <TextField {...params} name="type" label="Type" />}
                      onChange={(e, v) => {
                        newCompetition.type = v
                        setNewCompetition(newCompetition)
                      }}
                    />
                  </Grid>
                  <Grid item xs={6}>
                    <ResponsiveDatePicker
                      label="Start Date"
                      default={newCompetition.start_date ?? ""}
                      onChange={(v) => {
                        newCompetition.start_date = v
                        setNewCompetition(newCompetition)
                      }}
                    />
                  </Grid>
                  <Grid item xs={6}>
                    <ResponsiveDatePicker
                      label="End Date"
                      default={newCompetition.end_date ?? ""}
                      onChange={(v) => {
                        newCompetition.end_date = v
                        setNewCompetition(newCompetition)
                      }}
                    />
                  </Grid>
                  <Grid item xs={6}>
                    <TextField
                      fullWidth name="location" label='Location' placeholder='Location' defaultValue={newCompetition.location ?? ""}
                      onChange={(e) => {
                        newCompetition.location = e.target.value
                        setNewCompetition(newCompetition)
                      }}
                    />
                  </Grid>
                  <Grid item xs={4}>
                    <TextField
                      fullWidth name="code" label='Code' placeholder='Code' defaultValue={newCompetition.code ?? ""}
                      onChange={(e) => {
                        if (e.target.value.length > 0) {
                          newCompetition.code = e.target.value
                          setNewCompetition(newCompetition)
                        } else {
                          delete newCompetition.code
                        }
                      }}
                    />
                  </Grid>
                  <Grid item xs={2}>
                    <FormGroup>
                      <FormControlLabel
                        control={<Checkbox defaultCHecked={newCompetition.published ?? true} onChange={(e) => {
                            newCompetition.published = e.target.checked
                            setNewCompetition(newCompetition)
                        }}/>}
                        label="Published"
                      />
                    </FormGroup>
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
          <EnhancedTable rows={data} headCells={headCells} orderById='start_date' orderId='desc' pagination={false}/>
        </Card>
      </Grid>
    </Grid>
  )
}

export default CompetitionsPage
