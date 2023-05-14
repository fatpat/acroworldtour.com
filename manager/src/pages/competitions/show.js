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
import TabRuns from 'src/views/competitions/TabRuns'
import TabTeams from 'src/views/competitions/TabTeams'
import TabPilots from 'src/views/competitions/TabPilots'
import TabJudges from 'src/views/competitions/TabJudges'
import TabConfig from 'src/views/competitions/TabConfig'
import TabCompResults from 'src/views/competitions/TabCompResults'
import TabRepeatableTricks from 'src/views/competitions/TabRepeatableTricks'

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

const CompetitionPage = () => {
  // ** params
  const router = useRouter()
  const { cid, rid } = router.query

  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** auth/user
  const { user, authError, authIisLoading } = useUser();

  // ** local
  const [comp, setComp] = useState({})
  const [tempComp, setTempComp] = useState({})
  const [isLoading, setLoading] = useState(true)
  const [tabContext, setTabContext] = useState('actions')
  const [allPilots] = usePilots()
  const [allTeams] = useTeams()
  const [allJudges] = useJudges()
  const [allTricks] = useTricks()

  // ** refs
  const nameRef = useRef()
  const codeRef = useRef()
  const startDateRef = useRef()
  const endDateRef = useRef()
  const locationRef = useRef()
  const inputRef = useRef()
  const seasonsRef = useRef()

  const loadCompetition = async () => {
    setLoading(true)

    const [err, data, headers] = await APIRequest(`/competitions/${cid}`, {expect_json: true})

    if (err) {
        setComp(false)
        setTempComp(false)
        error(`Error while retrieving competitions list: ${err}`)
        return
    }

    data.delete = 'delete'
    data.update = 'update'
    data.id = data._id
    data.acronym = data.name.split(/[\W]+/).map(w => w[0]).join('').toUpperCase()

    setComp(data)
    setTempComp(Object.assign({}, data)) // clone data before assigning it to tempComp, otherwise they'll share the same object
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
        error(`error while updating competition ${cid}: ${err}`)
        return
    }
    console.log(retData)

    tempComp.image = retData.id
    setTempComp(tempComp)
    
    await updateCompetition(new Event('image'))
  }

  const updateCompetition = async(event) => {
    event.preventDefault()

    var route = `/competitions/${cid}`
    var method = 'PATCH'
    var expected_status = 204

    var image = null
    if (tempComp.image != null) {
      image = tempComp.image.split('/').at(-1)
    }

    const updatedCompetition = {
        name: tempComp.name,
        code: tempComp.code,
        start_date: tempComp.start_date,
        end_date: tempComp.end_date,
        location: tempComp.location,
        published: tempComp.published,
        type: tempComp.type,
        image: image,
        seasons: tempComp.seasons,
    }

    const [err, retData, headers] = await APIRequest(route, {
      expected_status: expected_status,
      method: method,
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(updatedCompetition),
    })

    if (err) {
        error(`error while updating competition ${cid}: ${err}`)
        return
    }

    if (tempComp.code != comp.code) return router.push(`/competitions/show?cid=${tempComp.code}`)
    loadCompetition()
  }

  const setPilots = async(pilots) => {
    const [err, retData, headers] = await APIRequest(`/competitions/${cid}/pilots`, {
        expected_status: 204,
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(pilots.map(p => p.civlid)),
    })

    if (err) {
      error(`error while updating pilots list ${cid}: ${err}`)
      return
    }
    loadCompetition()
  }

  const setTeams = async(teams) => {
    const [err, retData, headers] = await APIRequest(`/competitions/${cid}/teams`, {
        expected_status: 204,
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(teams.map(j => j.id)),
    })

    if (err) {
      error(`error while updating teams list ${cid}: ${err}`)
      return
    }
    loadCompetition()
  }

  const setJudges = async(judges) => {
    const [err, retData, headers] = await APIRequest(`/competitions/${cid}/judges`, {
        expected_status: 204,
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(judges.map(p => p.id)),
    })

    if (err) {
      error(`error while updating judges list ${cid}: ${err}`)
      return
    }
    loadCompetition()
  }

  const setRepeatableTricks = async(tricks) => {
    const [err, retData, headers] = await APIRequest(`/competitions/${cid}/repeatable_tricks`, {
        expected_status: 204,
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(tricks.map(t => t.id)),
    })

    if (err) {
      error(`error while updating repeatable tricks list ${cid}: ${err}`)
      return
    }
    loadCompetition()
  }

  const setConfig = async(config) => {
    const [err, retData, headers] = await APIRequest(`/competitions/${cid}/config`, {
        expected_status: 204,
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(config),
    })

    if (err) {
      error(`error while updating config ${cid}: ${err}`)
      return
    }
    loadCompetition()
  }

  const setState = async(status) => {
    if (!confirm(`Are you sure to ${status} this competition ?`)) return

    const [err, retData, headers] = await APIRequest(`/competitions/${cid}/${status}`, {
        expected_status: 204,
        method: 'POST',
    })

    if (err) {
      error(`error while ${status} competition ${cid}: ${err}`)
      return
    }
    loadCompetition()
  }

  const deleteCompetition = async (e) => {

    alert('No yet implemented! #TODO')
    return
    
    const id = e.target.dataset.id
    if (!confirm(`Are you sure you want to delete Competition ${name} (${id}) ?`)) return

    setLoading(true)
    const [err, data, headers] = await APIRequest(`/competitions/${id}`, {method: "DELETE", expected_status: 204})
    if (err) {
      error(`Error while deleting Competition ${id}: ${err}`)
    } else {
      success(`Competition ${id} successfully deleted`)
    }
    loadCompetition()
  }

  useEffect(() => {
      if (!router.isReady) return
      loadCompetition()
  }, [router.isReady])

  if (isLoading || !router.isReady) {
    return (
      <Box sx={{ width: '100%', textAlign: 'center' }}>
        <LinearProgress />
        Loading
      </Box>
    )
  }

  if (!comp) {
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
          <Avatar src={comp.image} onClick={updateImage}>{comp.acronym}</Avatar>
          &nbsp;
          {comp.name}<RefreshIcon className="hideToPrint" onClick={loadCompetition} />
        </Typography>
      </Grid>

      <Grid item xs={12} md={6} sx={{ paddingBottom: 4 }}>
        <Typography>
          <Editable
            text={tempComp.name}
            title="Name"
            onChange={updateCompetition}
            onCancel={(e) => {
              setTempComp(comp)
            }}
            childRef={nameRef}
          >
                    <TextField
                      fullWidth name="name" label='Name' placeholder='Name' defaultValue={tempComp.name} inputProps={ {ref:nameRef} }
                      onChange={(e) => {
                        tempComp.name = e.target.value
                        setTempComp(tempComp)
                      }}
                    />
          </Editable>
        </Typography>
        <Typography>
          <Editable
            text={tempComp.code}
            title="Code"
            onChange={updateCompetition}
            onCancel={(e) => {
              setTempComp(comp)
            }}
            childRef={codeRef}
          >
                    <TextField
                      fullWidth name="code" label='Code' placeholder='Code' defaultValue={tempComp.code} inputProps={ {ref:codeRef} }
                      onChange={(e) => {
                        tempComp.code = e.target.value
                        setTempComp(tempComp)
                      }}
                    />
          </Editable>
        </Typography>
        <Typography>
          Status: <strong>{tempComp.state}</strong>
{ comp.state == 'init' &&
          <Button variant='outlined' className="hideToPrint" startIcon={<RocketLaunchIcon />} onClick={() => setState('open') }>Open</Button>
}
{ comp.state == 'open' &&
          <Button variant='outlined' className="hideToPrint" startIcon={<CloseIcon />} onClick={() => setState('close') }>Close</Button>
}
{ comp.state == 'closed' &&
          <Button variant='outlined' className="hideToPrint" startIcon={<AutorenewIcon />} onClick={() => setState('reopen') }>Reopen</Button>
}
        </Typography>
        <Typography>
          Type: <strong>{tempComp.type}</strong>
        </Typography>
        <Typography>
          <Editable
            text={tempComp.seasons.length > 0 ? tempComp.seasons.join(', ') : "none"}
            title="Seasons"
            onChange={updateCompetition}
            onCancel={(e) => {
              setTempComp(comp)
            }}
            childRef={seasonsRef}
          >
                    <TextField
                      fullWidth name="seasons" label='Seasons' placeholder='Seasons' defaultValue={tempComp.seasons.join(', ')} inputProps={ {ref:seasonsRef} }
                      onChange={(e) => {
                        tempComp.seasons = e.target.value.split(/[, ]+/).filter(t => t != "")
                        setTempComp(tempComp)
                      }}
                    />
          </Editable>
        </Typography>
      </Grid>

      <Grid item xs={12} md={6} sx={{ paddingBottom: 4 }}>
        <Typography>
          <Editable
            text={tempComp.start_date}
            title="Start date"
            onChange={updateCompetition}
            onCancel={(e) => {
              setTempComp(comp)
            }}
            childRef={startDateRef}
          >
            <TextField
              fullWidth name="start_date" label='Start date' placeholder='Start date' defaultValue={tempComp.start_date} inputProps={ {ref:startDateRef} }
              onChange={(e) => {
                tempComp.start_date = e.target.value
                setTempComp(tempComp)
              }}
            />
          </Editable>
        </Typography>
        <Typography>
          <Editable
            text={tempComp.end_date}
            title="End date"
            onChange={updateCompetition}
            onCancel={(e) => {
              setTempComp(comp)
            }}
            childRef={endDateRef}
          >
            <TextField
              fullWidth name="end_date" label='End date' placeholder='End date' defaultValue={tempComp.end_date} inputProps={ {ref:endDateRef} }
              onChange={(e) => {
                tempComp.end_date = e.target.value
                setTempComp(tempComp)
              }}
            />
          </Editable>
        </Typography>

        <Typography>
          <Editable
            text={tempComp.location}
            title="Location"
            onChange={updateCompetition}
            onCancel={(e) => {
              setTempComp(comp)
            }}
            childRef={locationRef}
          >
            <TextField
              fullWidth name="location" label='Location' placeholder='Location' defaultValue={tempComp.location} inputProps={ {ref:locationRef} }
              onChange={(e) => {
                tempComp.location = e.target.value
                setTempComp(tempComp)
              }}
            />
          </Editable>
        </Typography>

        <Typography>
          <section>
            <div>
              <span>
                Published: 
                <Checkbox checked={tempComp.published} 
                  onChange={(e) => {
                    if (!confirm(`Are you sure to ${e.target.checked ? 'publish' : 'unpublish'} the competition ?`)) {
                        e.target.checked = !e.target.checked
                        return
                    }
                    tempComp.published = e.target.checked
                    setTempComp(tempComp)
                    updateCompetition(e)
                }}/>
              </span>
            </div>
          </section>
        </Typography>
      </Grid>

      <Grid item xs={12}>
        <Card>
          <TabContext value={tabContext}>
            <TabList className="hideToPrint"
              onChange={(e, v) => {setTabContext(v)}}
              aria-label='account-settings tabs'
              sx={{ borderBottom: theme => `1px solid ${theme.palette.divider}` }}
            >
{ comp.type == "solo" &&
              <Tab
                value='pilots'
                label={
                  <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    <AccountOutline />
                    <TabName>Pilots</TabName>
                  </Box>
                }
              />
}
{ comp.type == "synchro" &&
              <Tab
                value='teams'
                label={
                  <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    <AccountGroup />
                    <TabName>Teams</TabName>
                  </Box>
                }
              />
}
              <Tab
                value='judges'
                label={
                  <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    <AccountCowboyHat />
                    <TabName>Judges</TabName>
                  </Box>
                }
              />

              <Tab
                value='repeatable_tricks'
                label={
                  <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    <RepeatIcon />
                    <TabName>Repeatables tricks</TabName>
                  </Box>
                }
              />
              <Tab
                value='settings'
                label={
                  <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    <SettingsIcon />
                    <TabName>Competition Settings</TabName>
                  </Box>
                }
              />
              <Tab
                value='runs'
                label={
                  <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    <FlightIcon />
                    <TabName>Runs</TabName>
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
              <TabPilots pilots={comp.pilots} allPilots={allPilots} update={v => setPilots(v) } />
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='teams'>
              <TabTeams teams={comp.teams} allTeams={allTeams} update={v => setTeams(v) }/>
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='judges'>
              <TabJudges judges={comp.judges} allJudges={allJudges} update={v => setJudges(v) }/>
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='repeatable_tricks'>
              <TabRepeatableTricks tricks={comp.repeatable_tricks} allTricks={allTricks} update={v => setRepeatableTricks(v) }/>
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='settings'>
              <TabConfig config={comp.config} update={v => setConfig(v) } type={comp.type}/>
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='runs'>
              <TabRuns comp={comp} refresh={loadCompetition}/>
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='results'>
              <TabCompResults code={cid} />
            </TabPanel>
          </TabContext>
        </Card>
      </Grid>
    </Grid>
  )
}

export default CompetitionPage
