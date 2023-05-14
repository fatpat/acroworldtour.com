// ** react
import { useState, useEffect, useRef } from 'react';

// ** nextjs
import { useRouter } from 'next/router'

// ** auth
import { withPageAuthRequired, useUser } from '@auth0/nextjs-auth0';

// ** MUI Imports
import Grid from '@mui/material/Grid'
import Typography from '@mui/material/Typography'
import Button from '@mui/material/Button'
import Modal from '@mui/material/Modal'
import RefreshIcon from '@mui/icons-material/Refresh'
import TextField from '@mui/material/TextField'
import CircularProgress from '@mui/material/CircularProgress'
import LinearProgress from '@mui/material/LinearProgress'
import Box from '@mui/material/Box'
import Card from '@mui/material/Card'
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
import { DatePicker } from '@mui/x-date-pickers/DatePicker'
import { AdapterDateFns } from '@mui/x-date-pickers/AdapterDateFns'
import { LocalizationProvider } from '@mui/x-date-pickers/LocalizationProvider'
import TabList from '@mui/lab/TabList'
import TabPanel from '@mui/lab/TabPanel'
import TabContext from '@mui/lab/TabContext'
import { styled } from '@mui/material/styles'
import MuiTab from '@mui/material/Tab'
import LockOpenOutline from 'mdi-material-ui/LockOpenOutline'
import AccountOutline from 'mdi-material-ui/AccountOutline'
import InformationOutline from 'mdi-material-ui/InformationOutline'
import Checkbox from '@mui/material/Checkbox'
import RocketLaunchIcon from '@mui/icons-material/RocketLaunch'
import CloseIcon from '@mui/icons-material/Close'
import AutorenewIcon from '@mui/icons-material/Autorenew'
import AccountCowboyHat from 'mdi-material-ui/AccountCowboyHat'
import RepeatIcon from '@mui/icons-material/Repeat'
import SettingsIcon from '@mui/icons-material/Settings'
import EmojiEventsIcon from '@mui/icons-material/EmojiEvents'
import FlightIcon from '@mui/icons-material/Flight'
import DeleteIcon from '@mui/icons-material/Delete'
import AccountGroup from 'mdi-material-ui/AccountGroup'


// ** others
import Moment from 'react-moment'

// ** local
import EnhancedTable from 'src/views/tables/EnhancedTable'
import CardPilot from 'src/views/cards/CardPilot'
import { countryListAllIsoData } from 'src/util/countries'
import { useNotifications } from 'src/util/notifications'
import { APIRequest, usePilots, useTeams, useJudges, useTricks } from 'src/util/backend'
import modalStyle from 'src/configs/modalStyle'
import ResponsiveDatePicker from 'src/components/ResponsiveDatePicker'
import Editable from 'src/components/Editable'

// ** Tabs Imports
import TabCompetitions from 'src/views/seasons/TabCompetitions'
import TabTeams from 'src/views/competitions/TabTeams'
import TabPilots from 'src/views/competitions/TabPilots'
import TabSeasonResults from 'src/views/seasons/TabSeasonResults'

const Tab = styled(MuiTab)(({ theme }) => ({
  [theme.breakpoints.down('md')]: {
    minWidth: 100
  },
  [theme.breakpoints.down('sm')]: {
    minWidth: 67
  }
}))

const TabName = styled('span')(({ theme }) => ({
  lineHeight: 1.71,
  fontSize: '0.875rem',
  marginLeft: theme.spacing(2.4),
  [theme.breakpoints.down('md')]: {
    display: 'none'
  }
}))

const SeasonPage = () => {
  // ** params
  const router = useRouter()

  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** auth/user
  const { user, authError, authIisLoading } = useUser();

  // ** local
  const [sid, setSid] = useState(router.query['sid'])
  const [season, setSeason] = useState({})
  const [tempSeason, setTempSeason] = useState({})
  const [isLoading, setLoading] = useState(false)
  const [tabContext, setTabContext] = useState('')

  // ** refs
  const nameRef = useRef()
  const codeRef = useRef()
  const yearRef = useRef()
  const endDateRef = useRef()
  const locationRef = useRef()
  const inputRef = useRef()

  const loadSeason = async () => {
    setLoading(true)
    sid = sid ?? router.query['sid']

    const [err, data, headers] = await APIRequest(`/seasons/${sid}/export`, {expect_json: true})

    if (err) {
        setSeason(false)
        setTempSeason(false)
        error(`Error while retrieving seasons list: ${err}`)
        return
    }

    data.delete = 'delete'
    data.update = 'update'
    data.id = data._id

    data.pilots = {}
    data.teams = {}
    data.competitions.forEach(c => {
      c.pilots.forEach(p => data.pilots[p.civlid] = p)
      c.teams.forEach(t => {
        data.teams[t.name] = t
        t.pilots.forEach(p => data.pilots[p.civlid] = p)
      })
    })
    data.pilots = Object.values(data.pilots)
    data.teams = Object.values(data.teams)

    data.acronym = data.name.split(/[\W]+/).map(w => w[0]).join('').toUpperCase()

    setSeason(data)
    setTempSeason(Object.assign({}, data)) // clone data before assigning it to tempSeason, otherwise they'll share the same object
    setLoading(false)
  }

  const updateImage = async(event) => {
    inputRef.current.click()
  }

  const uploadImage = async(event) => {
    const file = event.target.files && event.target.files[0]
    if (!file) return

    if (!file.type.match(/^image\//)) {
      return error(`Error: only images are allowed (got '${file.type}')`)
    }

    // reset file input
    event.target.value = null;

    // create a formData
    const formData = new FormData()
    formData.append("file", file, file.name)

    const [err, retData, headers] = await APIRequest('/files/new', {
      expected_status: 200,
      method: 'POST',
      body: formData,
    })

    if (err) {
        error(`error while updating season ${sid}: ${err}`)
        return
    }

    tempSeason.image = retData.id
    setTempSeason(tempSeason)
    
    await updateSeason(new Event('image'))
  }

  const updateSeason = async(event) => {
    event.preventDefault()

    var route = `/seasons/${sid}`
    var method = 'put'
    var expected_status = 204

    var image = null
    if (tempSeason.image != null) {
      image = tempSeason.image.split('/').at(-1)
    }


    const updatedSeason = {
        name: tempSeason.name,
        code: tempSeason.code,
        year: tempSeason.year,
        image: image,
    }

    const [err, retData, headers] = await APIRequest(route, {
      expected_status: expected_status,
      method: method,
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(updatedSeason),
    })

    if (err) {
        error(`error while updating season ${sid}: ${err}`)
        return
    }
    setSid(tempSeason.code)
    if (tempSeason.code != season.code) return router.push(`/seasons/show?sid=${tempSeason.code}`)
    loadSeason()
  }

  const deleteSeason = async (e) => {

    if (!confirm(`Are you sure you want to delete Season ${season.name} ?`)) return

    setLoading(true)
    const [err, data, headers] = await APIRequest(`/seasons/${season.id}`, {method: "DELETE", expected_status: 204})
    if (err) return error(`Error while deleting Competition ${season.id}: ${err}`)

    success(`Competition ${season.name} successfully deleted`)
    return router.push(`/seasons`)
  }

  useEffect(() => {
      if (!router.isReady) return
      loadSeason()
      setTabContext('results')
  }, [router.isReady])

  if (isLoading || !router.isReady) {
    return (
      <Box sx={{ width: '100%', textAlign: 'center' }}>
        <LinearProgress />
        Loading
      </Box>
    )
  }

  if (!season) {
    error("Empty or invalid data")
    return ''
  }

  return (
    <Grid container spacing={6}>

      <Grid item xs={12}>
        <input
          style={{display: 'none'}}
          ref={inputRef}
          type="file"
          onChange={uploadImage}
        />  
        <Typography variant='h5' sx={{display: 'flex'}}>
          <Avatar src={season.image} onClick={updateImage}>{season.acronym}</Avatar>
          &nbsp;
          {season.name}<RefreshIcon className="hideToPrint" onClick={loadSeason} />
        </Typography>
      </Grid>

      <Grid item xs={8} sx={{ paddingBottom: 4 }}>
        <Typography>
          <Editable
            text={tempSeason.name}
            title="Name"
            onChange={updateSeason}
            onCancel={(e) => {
              setTempSeason(season)
            }}
            childRef={nameRef}
          >
                    <TextField
                      fullWidth name="name" label='Name' placeholder='Name' defaultValue={tempSeason.name} inputProps={ {ref:nameRef} }
                      onChange={(e) => {
                        tempSeason.name = e.target.value
                        setTempSeason(tempSeason)
                      }}
                    />
          </Editable>
        </Typography>
        <Typography>
          <Editable
            text={tempSeason.code}
            title="Code"
            onChange={updateSeason}
            onCancel={(e) => {
              setTempSeason(season)
            }}
            childRef={codeRef}
          >
                    <TextField
                      fullWidth name="code" label='Code' placeholder='Code' defaultValue={tempSeason.code} inputProps={ {ref:codeRef} }
                      onChange={(e) => {
                        tempSeason.code = e.target.value
                        setTempSeason(tempSeason)
                      }}
                    />
          </Editable>
        </Typography>
        <Typography>
          <Editable
            text={tempSeason.year}
            title="Year"
            onChange={updateSeason}
            onCancel={(e) => {
              setTempSeason(season)
            }}
            childRef={yearRef}
          >
            <TextField
              fullWidth name="year" label='Year' placeholder='Year' defaultValue={tempSeason.year} inputProps={ {ref:yearRef} }
              onChange={(e) => {
                tempSeason.year = parseInt(e.target.value)
                setTempSeason(tempSeason)
              }}
            />
          </Editable>
        </Typography>
      </Grid>
      <Grid item xs={4} sx={{ paddingBottom: 4, 'text-align': 'right' }}>
        <Button variant="contained" color="error" onClick={deleteSeason} startIcon={<DeleteIcon />}>Delete</Button>
      </Grid>

      <Grid item xs={12}>
        <Card>
          <TabContext value={tabContext}>
            <TabList className="hideToPrint"
              onChange={(e, v) => {setTabContext(v)}}
              aria-label='account-settings tabs'
              sx={{ borderBottom: theme => `1px solid ${theme.palette.divider}` }}
            >
              <Tab
                value='pilots'
                label={
                  <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    <AccountOutline />
                    <TabName>Pilots</TabName>
                  </Box>
                }
              />
              <Tab
                value='teams'
                label={
                  <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    <AccountGroup />
                    <TabName>Teams</TabName>
                  </Box>
                }
              />
              <Tab
                value='competitions'
                label={
                  <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    <FlightIcon />
                    <TabName>Competitions</TabName>
                  </Box>
                }
              />
              <Tab
                value='results'
                label={
                  <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    <EmojiEventsIcon />
                    <TabName>Results</TabName>
                  </Box>
                }
              />
            </TabList>

            <TabPanel sx={{ p: 0 }} value='pilots'>
              <TabPilots pilots={season.pilots} />
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='teams'>
              <TabTeams teams={season.teams} />
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='competitions'>
              <TabCompetitions competitions={season.competitions} />
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='results'>
              <TabSeasonResults season={season} />
            </TabPanel>
          </TabContext>
        </Card>
      </Grid>
    </Grid>
  )
}

export default SeasonPage
