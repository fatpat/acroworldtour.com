// ** react
import { useState, useEffect, useRef } from 'react';

// ** nextjs
import { useRouter } from 'next/router'

// ** auth
import { withPageAuthRequired, useUser } from '@auth0/nextjs-auth0/client'

// ** MUI Imports
import Grid from '@mui/material/Grid'
import Typography from '@mui/material/Typography'
import Button from '@mui/material/Button'
import Modal from '@mui/material/Modal'
import RefreshIcon from '@mui/icons-material/Refresh'
import TextField from '@mui/material/TextField'
import CircularProgress from '@mui/material/CircularProgress'
import LinearProgress from '@mui/material/LinearProgress'
import Box from '@mui/material/Box';
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
import ParaglidingIcon from '@mui/icons-material/Paragliding'
import ListIcon from '@mui/icons-material/List'
import TableBody from '@mui/material/TableBody'
import TableCell from '@mui/material/TableCell'
import TableRow from '@mui/material/TableRow'
import TableHead from '@mui/material/TableHead'
import Table from '@mui/material/Table'
import TableContainer from '@mui/material/TableContainer'
import List from "@mui/material/List";
import FunctionsIcon from '@mui/icons-material/Functions';

// ** others
import Moment from 'react-moment'
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'

// ** local
import EnhancedTable from 'src/views/tables/EnhancedTable'
import CardPilot from 'src/views/cards/CardPilot'
import { countryListAllIsoData } from 'src/util/countries'
import { useNotifications } from 'src/util/notifications'
import { APIRequest, useTricks } from 'src/util/backend'
import modalStyle from 'src/configs/modalStyle'
import Editable from 'src/components/Editable'

// ** Tabs Imports
import TabRuns from 'src/views/competitions/TabRuns'
import TabTeams from 'src/views/competitions/TabTeams'
import TabPilots from 'src/views/competitions/TabPilots'
import TabJudges from 'src/views/competitions/TabJudges'
import TabConfig from 'src/views/competitions/TabConfig'
import TabRunResults from 'src/views/competitions/TabRunResults'
import TabRepeatableTricks from 'src/views/competitions/TabRepeatableTricks'
import TabFlightsSolo from 'src/views/competitions/TabFlightsSolo'
import TabFlightsSoloDetails from 'src/views/competitions/TabFlightsSoloDetails'
import TabFlightsSynchro from 'src/views/competitions/TabFlightsSynchro'
import SortablePilotStartingOrder from 'src/views/runs/SortablePilotStartingOrder'


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

const RunPage = () => {
  // ** params
  const router = useRouter()
  const { cid, rid } = router.query

  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** auth/user
  const { user, authError, authIisLoading } = useUser();

  // ** local
  const [comp, setComp] = useState(false)
  const [run, setRun] = useState(false)
  const [pilots, setPilotsVar] = useState([])
  const [teams, setTeamsVar] = useState([])
  const [tempComp, setTempComp] = useState({})
  const [isLoading, setLoading] = useState(false)
  const [tabContext, setTabContext] = useState('actions')
  const [allTricks] = useTricks()

  // ** refs
  const nameRef = useRef()
  const codeRef = useRef()
  const startDateRef = useRef()
  const endDateRef = useRef()
  const locationRef = useRef()

  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  );

  const loadCompetition = async () => {
    setLoading(true)

    const [err, data, headers] = await APIRequest(`/competitions/${cid}`, {expect_json: true})

    if (err) {
        setComp(false)
        setRun(false)
        setPilotsVar([])
        setTeamsVar([])
        setTempComp(false)
        error(`Error while retrieving competitions list: ${err}`)
        return
    }

    data.delete = 'delete'
    data.update = 'update'
    data.id = data._id

    setComp(data)
    setRun(data.runs[rid])
    data.type === 'solo' && setPilotsVar(data.runs[rid].pilots)
    data.type === 'synchro' && setTeamsVar(data.runs[rid].teams)
    setTempComp(Object.assign({}, data)) // clone data before assigning it to tempComp, otherwise they'll share the same object
    setLoading(false)
  }

  const setState = async(status) => {
    if (!confirm(`Are you sure to ${status} run ${rid} of competition ${cid} ?`)) return

    const [err, retData, headers] = await APIRequest(`/competitions/${cid}/runs/${rid}/${status}`, {
        expected_status: 204,
        method: 'POST',
    })

    if (err) {
      error(`error while ${status} run ${rid} of competition ${cid}: ${err}`)
      return
    }
    loadCompetition()
  }

  const setPilots = async(pilots) => {
    const [err, retData, headers] = await APIRequest(`/competitions/${cid}/runs/${rid}/pilots`, {
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
    const [err, retData, headers] = await APIRequest(`/competitions/${cid}/runs/${rid}/teams`, {
        expected_status: 204,
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(teams.map(j => j._id)),
    })

    if (err) {
      error(`error while updating teams list ${cid}: ${err}`)
      return
    }
    loadCompetition()
  }

  const setJudges = async(judges) => {
    const [err, retData, headers] = await APIRequest(`/competitions/${cid}/runs/${rid}/judges`, {
        expected_status: 204,
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(judges.map(p => p._id)),
    })

    if (err) {
      error(`error while updating judges list ${cid}: ${err}`)
      return
    }
    loadCompetition()
  }

  const setRepeatableTricks = async(tricks) => {
    const [err, retData, headers] = await APIRequest(`/competitions/${cid}/runs/${rid}/repeatable_tricks`, {
        expected_status: 204,
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(tricks.map(t => t._id)),
    })

    if (err) {
      error(`error while updating repeatable tricks list ${cid}: ${err}`)
      return
    }
    loadCompetition()
  }

  const setConfig = async(config) => {
    const [err, retData, headers] = await APIRequest(`/competitions/${cid}/runs/${rid}/config`, {
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

  const deleteRun = async (e) => {

    alert('No yet implemented! #TODO')
    return
    
    const id = e.target.dataset.id
    if (!confirm(`Are you sure you want to delete run #${rid} from Competition ${name} ?`)) return

    setLoading(true)
    const [err, data, headers] = await APIRequest(`/competitions/${id}/runs/${rid}`, {method: "DELETE", expected_status: 204})
    if (err) {
      error(`Error while deleting run #${rid} from competition ${cid}: ${err}`)
    } else {
      success(`Run #${rid} from competition ${cid} successfully deleted`)
    }
    loadCompetition()
  }

  const changeStartingOrder = async (event) => {
    const {active, over} = event
    if (active.id === over.id) return

    if (comp.type === 'solo') {
      const oldIndex = run.pilots.findIndex(p => p.civlid == active.id.civlid)
      const newIndex = run.pilots.findIndex(p => p.civlid == over.id.civlid)
      if (oldIndex < 0 || newIndex < 0) return

      setPilots(arrayMove(pilots, oldIndex, newIndex))
      return
    }

    if (comp.type === 'synchro') {
      const oldIndex = run.teams.findIndex(t => t._id == active.id._id)
      const newIndex = run.teams.findIndex(t => t._id == over.id._id)
      if (oldIndex < 0 || newIndex < 0) return

      setTeams(arrayMove(teams, oldIndex, newIndex))
      return
    }
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

  if (!comp || !run) {
    return ''
  }

  return (
    <Grid container spacing={6}>

      <Grid item xs={12}>
        <Typography variant='h6'>
          <Link href={`/competitions/show?cid=${comp.code}`}>{comp.name}</Link>
        </Typography>
        <Typography variant='h3' className="hideToPrint">Run #{parseInt(rid)+1}<RefreshIcon onClick={loadCompetition} /></Typography>
      </Grid>

      <Grid item xs={12} md={6} sx={{ paddingBottom: 4 }} className="hideToPrint">
        <Typography>
          Status: <strong>{run.state}</strong>
{ run.state == 'init' &&
          <Button variant='outlined' className="hideToPrint" startIcon={<RocketLaunchIcon />} onClick={() => setState('open') }>Open</Button>
}
{ run.state == 'open' &&
          <Button variant='outlined' className="hideToPrint" startIcon={<CloseIcon />} onClick={() => setState('close') }>Close</Button>
}
{ run.state == 'closed' &&
          <Button variant='outlined' className="hideToPrint" startIcon={<AutorenewIcon />} onClick={() => setState('reopen') }>Reopen</Button>
}
        </Typography>
        <Typography>
          Type: <strong>{tempComp.type}</strong>
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
                    <TabName>Run Settings</TabName>
                  </Box>
                }
              />
              <Tab
                value='flights'
                label={
                  <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    <ParaglidingIcon />
                    <TabName>Flights</TabName>
                  </Box>
                }
              />
{ comp.type == "solo" &&
              <Tab
                value='flights_details'
                label={
                  <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    <FunctionsIcon />
                    <TabName>Flights Details</TabName>
                  </Box>
                }
              />
}
              <Tab
                value='starting_order'
                label={
                  <Box sx={{ display: 'flex', alignItems: 'center' }}>
                    <ListIcon />
                    <TabName>Starting Order</TabName>
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
              <TabPilots pilots={run.pilots} allPilots={comp.pilots} update={v => setPilots(v) } />
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='teams'>
              <TabTeams teams={run.teams} allTeams={comp.teams} update={v => setTeams(v) }/>
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='judges'>
              <TabJudges judges={run.judges} allJudges={comp.judges} update={v => setJudges(v) }/>
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='repeatable_tricks'>
              <TabRepeatableTricks tricks={run.repeatable_tricks} allTricks={allTricks} update={v => setRepeatableTricks(v) }/>
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='settings'>
              <TabConfig config={run.config} update={v => setConfig(v) } type={comp.type}/>
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='flights'>
{ comp.type == "solo" && <TabFlightsSolo comp={comp} run={run} rid={rid}/>}
{ comp.type == "synchro" && <TabFlightsSynchro comp={comp} run={run} rid={rid}/>}
            </TabPanel>
{ comp.type == "solo" &&
            <TabPanel sx={{ p: 0 }} value='flights_details'>
              <TabFlightsSoloDetails comp={comp} run={run} rid={rid}/>
            </TabPanel>
}
            <TabPanel sx={{ p: 0 }} value='starting_order'>
              <TableContainer>
                <CardContent>
                  <Box sx={{display: 'flex',justifyContent: 'center'}}>
                    <Typography variant="h4">
                      Run {parseInt(rid)+1} Starting Order
                    </Typography>
                  </Box>
                    { comp.type == "solo" &&
                    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={changeStartingOrder}>
                      <SortableContext items={pilots} strategy={verticalListSortingStrategy}>
                        { pilots.map((p,i) => <SortablePilotStartingOrder key={p.civlid} id={p} text={`${i+1} ${p.name}${p.is_awt ? "⭐" : ""}`} />)}
                      </SortableContext>
                    </DndContext>
                    }
                    { comp.type == "synchro" &&
                    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={changeStartingOrder}>
                      <SortableContext items={teams} strategy={verticalListSortingStrategy}>
                        { teams.map((p,i) => <SortablePilotStartingOrder key={p.id} id={p} text={`${i+1} ${p.name}`} />)}
                      </SortableContext>
                    </DndContext>
                    }
                </CardContent>
              </TableContainer>
            </TabPanel>
            <TabPanel sx={{ p: 0 }} value='results'>
              <TabRunResults code={cid} rid={rid} />
            </TabPanel>
          </TabContext>
        </Card>
      </Grid>
    </Grid>
  )
}

export default RunPage
