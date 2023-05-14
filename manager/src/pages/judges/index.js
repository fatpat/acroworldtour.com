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
import Autocomplete from '@mui/material/Autocomplete'
import Avatar from '@mui/material/Avatar'

// ** local
import EnhancedTable from 'src/views/tables/EnhancedTable'
import CardPilot from 'src/views/cards/CardPilot'
import { countryListAllIsoData } from 'src/util/countries'
import { useNotifications } from 'src/util/notifications'
import { APIRequest } from 'src/util/backend'
import modalStyle from 'src/configs/modalStyle'

const JudgesPage = () => {
  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** auth/user
  const { user, authError, authIisLoading } = useUser();

  // ** local
  const [data, setData] = useState([])
  const [fullData, setFullData] = useState([])
  const [levels, setLevels] = useState([])
  const [pilots, setPilots] = useState([])
  const [isLoading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [modalTitle, setModalTitle] = useState('')
  const [newJudge, setNewJudge] = useState({})

  const loadJudges = async () => {
    setLoading(true)

    const [err, data, headers] = await APIRequest('/judges', {expect_json: true})

    if (err) {
        setData(false)
        setFullData(false)
        error(`Error while retrieving judges list: ${err}`)
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

  const loadLevels = async () => {
    const [err, data, headers] = await APIRequest('/judges/levels', {expect_json: true})

    if (err) {
        setLevels(false)
        error(`Error while retrieving judge levels: ${err}`)
        return
    }
    setLevels(data)
  }

  const loadPilots = async () => {
    const [err, data, headers] = await APIRequest('/pilots', {expect_json: true})

    if (err) {
        setPilots([])
        error(`Error while retrieving pilots list: ${err}`)
        return
    }
    setPilots(data)
  }

  const createOrUpdateJudge = async(event) => {
    event.preventDefault()

    var route = '/judges/new'
    var method = 'POST'
    var expected_status = 201

    if (newJudge._id) {
      route = `/judges/${newJudge._id}`
      method = 'PUT'
      expected_status = 204
    }

    const [err, data, headers] = await APIRequest(route, {
      expected_status: expected_status,
      method: method,
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(newJudge),
    })

    if (err) {
        if (newJudge._id) {
            error(`error while updating judge ${newJudge._id}: ${err}`)
        } else {
            error(`error while creating new judge: ${err}`)
        }
        return
    }

    setModalOpen(false)
    loadJudges()
  }

  const deleteJudge = async (id) => {
      if (!confirm(`Are you sure you want to delete Judge ${name} (${id}) ?`)) return

      setLoading(true)
    const [err, data, headers] = await APIRequest(`/judges/${id}`, {method: "DELETE", expected_status: 204})
    if (err) {
      error(`Error while deleting Judge ${id}: ${err}`)
    } else {
      success(`Judge ${id} successfully deleted`)
    }
    loadJudges()
  }

  const openCreateModal = () => {
    setModalTitle('New judge')
    setNewJudge({})
    setModalOpen(true)
  }

  const openUpdateModal = (id) => {
    const judge = data.find(j => j._id == id)
    setModalTitle(`Updating judge ${id}`)
    setNewJudge(judge)
    setModalOpen(true)
  }

  const updateSearch = async(e) => {
    const s = e.target.value
    // https://stackoverflow.com/questions/990904/remove-accents-diacritics-in-a-string-in-javascript
    // to compare ignoring accents
    s = s.normalize("NFD").replace(/[\u0300-\u036f]/g, "")
    const r = new RegExp(s, "i");
    const d = fullData.filter(judge => judge.name.normalize("NFD").replace(/[\u0300-\u036f]/g, "").match(r))
    setData(d)
    info(`${d.length} judges filtered over ${fullData.length}`)
  }

  const headCells = [
    {
      id: 'name',
      rewrite: (name) => {
          const judge = fullData.find(j => j.name == name)
          if (!judge || !judge.civlid || judge.civlid <= 0) return name
          const pilot = pilots.find(p => p.civlid == judge.civlid)
          if (!pilot) return name
          return (
            <Box>
              <Link href={pilot.link} target="_blank" rel="noopener noreferrer">
                <Avatar alt={name} src={pilot.photo}/>
                {name}
              </Link>
            </Box>
          )
      }
    },
    {
      id: 'country',
    },
    {
      id: 'level',
    },
    {
      id: 'civlid',
    },
    {
      id: 'update',
      type: 'ACTION',
      func: openUpdateModal,
    },
    {
      id: 'delete',
      type: 'ACTION',
      func: deleteJudge,
    }
  ]

  useEffect(() => {
      loadJudges()
      loadLevels()
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
        <Typography variant='h5'>Judges<RefreshIcon onClick={loadJudges} /></Typography>
      </Grid>
      <Grid item xs={4} sm={4}>
        <TextField fullWidth id='outlined-basic' label='Search judge' variant='outlined' onChange={updateSearch} />
      </Grid>
      <Grid item xs={8} sm={8} container>
        <Button
          variant='contained'
          onClick={openCreateModal}
          startIcon={<AddIcon />}
        >new judge</Button>
        <Modal
          open={modalOpen}
          onClose={() => setModalOpen(false)}
          aria-labelledby='modal-modal-title'
          aria-describedby='modal-modal-description'
        >
          <Card sx={modalStyle}>
            <form onSubmit={createOrUpdateJudge}>
              <CardHeader
                title={modalTitle}
                titleTypographyProps={{ variant: 'h6' }}
              />
              <CardContent>
                <Grid container spacing={5}>
                  <Grid item xs={12}>
                    <TextField
                      fullWidth name="name" label='Name' placeholder='Judge name' defaultValue={newJudge.name ?? ""}
                      onChange={(e) => {
                        newJudge.name = e.target.value
                        setNewJudge(newJudge)
                      }}
                    />
                  </Grid>
                  <Grid item xs={12}>
                    <Autocomplete
                      disablePortal
                      id="autocomplete-level"
                      options={levels}
                      defaultValue={newJudge.level}
                      renderInput={(params) => <TextField {...params} name="level" label="Level" />}
                      onChange={(e, v) => {
                        newJudge.level = v
                        setNewJudge(newJudge)
                      }}
                    />
                  </Grid>
                  <Grid item xs={12}>
                    <Autocomplete
                      disablePortal
                      id="autocomplete-country"
                      options={countryListAllIsoData}
                      getOptionLabel={c => `${c.name} (${c.code3})`}
                      defaultValue={countryListAllIsoData.find(c => c.code3.toLowerCase() == newJudge.country)}
                      renderInput={(params) => <TextField {...params} name="country" label="Country" />}
                      onChange={(e, v) => {
                        newJudge.country = v.code3.toLowerCase()
                        setNewJudge(newJudge)
                      }}
                    />
                  </Grid>
                  <Grid item xs={12}>
                    <TextField
                      fullWidth name="civlid" label='CIVLD ID' placeholder='CIVL ID (optional)' defaultValue={ newJudge.civlid ?? ""}
                      onChange={(e) => {
                        const v = e.target.value
                        if (v == "") v = null
                        newJudge.civlid = v
                        setNewJudge(newJudge)
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

//export default withPageAuthRequired(JudgesPage)
export default withPageAuthRequired(JudgesPage)
