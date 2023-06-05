// ** react
import { useState, useEffect, useRef } from 'react';

// ** nextjs
import { useRouter } from 'next/router'
import Image from 'next/image';

// ** auth
import { withPageAuthRequired, useUser } from '@auth0/nextjs-auth0';

// ** MUI Imports
import Grid from '@mui/material/Grid'
import Typography from '@mui/material/Typography'
import Button from '@mui/material/Button'
import IconButton from '@mui/material/IconButton'
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
import NoPhotographyIcon from '@mui/icons-material/NoPhotography';
import ClearIcon from '@mui/icons-material/Clear';

// ** others
import Moment from 'react-moment'

// ** local
import EnhancedTable from 'src/views/tables/EnhancedTable'
import CardPilot from 'src/views/cards/CardPilot'
import { countryListAllIsoData } from 'src/util/countries'
import { useNotifications } from 'src/util/notifications'
import { APIRequest, usePilots, useTeams, useJudges, useTricks, useCompetitions } from 'src/util/backend'
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

const EventPage = () => {
  // ** params
  const router = useRouter()
  const { eid, rid } = router.query

  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** auth/user
  const { user, authError, authIisLoading } = useUser();

  // ** local
  const [event, setEvent] = useState({})
  const [tempEvent, setTempEvent] = useState({})
  const [isLoading, setLoading] = useState(true)
  const [tabContext, setTabContext] = useState('actions')
  const [allCompetitions] = useCompetitions()
  const [competitions, setCompetitions] = useState([])

  // ** refs
  const nameRef = useRef()
  const codeRef = useRef()
  const startDateRef = useRef()
  const endDateRef = useRef()
  const countryRef = useRef()
  const locationRef = useRef()
  const inputLogo = useRef()
  const inputImage = useRef()
  const seasonsRef = useRef()
  const websiteRef = useRef()
  const streamingUrlRef = useRef()
  const descriptionRef = useRef()


  var headCells = [
    {
      id: 'name',
      type: 'LINK',
      href: (v, comp) => `/competitions/show?cid=${comp.code}`,
    },
    {
      id: 'code',
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
      id: 'number_of_runs',
    },
    {
      id: 'published',
      type: 'BOOLEAN',
    }
  ]

  const loadEvent = async () => {
    setLoading(true)

    const [err, data, headers] = await APIRequest(`/events/${eid}`, {expect_json: true})

    if (err) {
        setEvent(false)
        setTempEvent(false)
        setCompetitions([])
        error(`Error while retrieving events list: ${err}`)
        return
    }

    data.delete = 'delete'
    data.update = 'update'
    data.id = data._id
    data.acronym = data.name.split(/[\W]+/).map(w => w[0]).join('').toUpperCase()

    setEvent(data)
    setTempEvent(Object.assign({}, data)) // clone data before assigning it to tempEvent, otherwise they'll share the same object
    setCompetitions(data.competitions.map(code => allCompetitions.find(c => c.code == code)))
    console.log("comps", competitions)
    setLoading(false)
  }

  const updateImage = async(event) => {
    inputImage.current.click()
  }

  const updateLogo = async(event) => {
    inputLogo.current.click()
  }

  const uploadFile = async(event) => {
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
        return error(`error while updating event ${eid}: ${err}`)
    }
    console.log(retData)

    return retData
  }

  const uploadImage = async(event) => {

    const image = await uploadFile(event)
    if (typeof(image) !== 'object') {
      return
    }

    tempEvent.image = image.id
    setTempEvent(tempEvent)

    await updateEvent(new Event('image'))
  }

  const uploadLogo = async(event) => {

    const logo = await uploadFile(event)
    if (typeof(logo) !== 'object') {
      return
    }

    tempEvent.logo = logo.id
    setTempEvent(tempEvent)

    await updateEvent(new Event('logo'))
  }

  const removeImage = async(event) => {
    if (!confirm(`Are you sure to remove the image of the event ?`)) return
    tempEvent.image = null
    setTempEvent(tempEvent)
    await updateEvent(new Event('remove image'))
  }

  const removeLogo = async(event) => {
    if (!confirm(`Are you sure to remove the logo of the event ?`)) return
    tempEvent.logo = null
    setTempEvent(tempEvent)
    await updateEvent(new Event('remove logo'))
  }

  const updateEvent = async(e) => {
    e.preventDefault()

    var route = `/events/${eid}`
    var method = 'PATCH'
    var expected_status = 204

    var image = tempEvent.image != null ? tempEvent.image.split('/').at(-1) : null
    var logo = tempEvent.logo != null ? tempEvent.logo.split('/').at(-1) : null

    const updatedEvent = {
        name: tempEvent.name,
        code: tempEvent.code,
        start_date: tempEvent.start_date,
        end_date: tempEvent.end_date,
        country: tempEvent.country,
        location: tempEvent.location,
        image: image,
        logo: logo,
        website: tempEvent.website,
        description: tempEvent.description,
        competitions: tempEvent.competitions,
    }

    const [err, retData, headers] = await APIRequest(route, {
      expected_status: expected_status,
      method: method,
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(updatedEvent),
    })

    if (err) {
        error(`error while updating event ${eid}: ${err}`)
        return
    }

    if (tempEvent.code != event.code) return router.push(`/events/show?eid=${tempEvent.code}`)
    loadEvent()
  }

  const setPilots = async(pilots) => {
    const [err, retData, headers] = await APIRequest(`/events/${eid}/pilots`, {
        expected_status: 204,
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(pilots.map(p => p.civlid)),
    })

    if (err) {
      error(`error while updating pilots list ${eid}: ${err}`)
      return
    }
    loadEvent()
  }

  const setTeams = async(teams) => {
    const [err, retData, headers] = await APIRequest(`/events/${eid}/teams`, {
        expected_status: 204,
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(teams.map(j => j.id)),
    })

    if (err) {
      error(`error while updating teams list ${eid}: ${err}`)
      return
    }
    loadEvent()
  }

  const setJudges = async(judges) => {
    const [err, retData, headers] = await APIRequest(`/events/${eid}/judges`, {
        expected_status: 204,
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(judges.map(p => p.id)),
    })

    if (err) {
      error(`error while updating judges list ${eid}: ${err}`)
      return
    }
    loadEvent()
  }

  const setRepeatableTricks = async(tricks) => {
    const [err, retData, headers] = await APIRequest(`/events/${eid}/repeatable_tricks`, {
        expected_status: 204,
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(tricks.map(t => t.id)),
    })

    if (err) {
      error(`error while updating repeatable tricks list ${eid}: ${err}`)
      return
    }
    loadEvent()
  }

  const setConfig = async(config) => {
    const [err, retData, headers] = await APIRequest(`/events/${eid}/config`, {
        expected_status: 204,
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(config),
    })

    if (err) {
      error(`error while updating config ${eid}: ${err}`)
      return
    }
    loadEvent()
  }

  const setState = async(status) => {
    if (!confirm(`Are you sure to ${status} this event ?`)) return

    const [err, retData, headers] = await APIRequest(`/events/${eid}/${status}`, {
        expected_status: 204,
        method: 'POST',
    })

    if (err) {
      error(`error while ${status} event ${eid}: ${err}`)
      return
    }
    loadEvent()
  }

  const deleteEvent = async (e) => {

    alert('No yet implemented! #TODO')
    return

    const id = e.target.dataset.id
    if (!confirm(`Are you sure you want to delete Event ${name} (${id}) ?`)) return

    setLoading(true)
    const [err, data, headers] = await APIRequest(`/events/${id}`, {method: "DELETE", expected_status: 204})
    if (err) {
      error(`Error while deleting Event ${id}: ${err}`)
    } else {
      success(`Event ${id} successfully deleted`)
    }
    loadEvent()
  }

  useEffect(() => {
      if (!router.isReady) return
      loadEvent()
  }, [router.isReady])

  if (isLoading || !router.isReady) {
    return (
      <Box sx={{ width: '100%', textAlign: 'center' }}>
        <LinearProgress />
        Loading
      </Box>
    )
  }

  if (!event) {
    error("Empty or invalid data")
    return ''
  }

  return (
    <Grid container spacing={6}>

      <Grid item xs={12}>
        <input
          style={{display: 'none'}}
          ref={inputLogo}
          type="file"
          onChange={uploadLogo}
        />
        <Typography variant='h5' sx={{display: 'flex'}}>
          <Avatar src={`${process.env.NEXT_PUBLIC_API_URL}/files/${event.logo}`} onClick={updateLogo}>{event.acronym}</Avatar>
          &nbsp;
          {event.name}<RefreshIcon className="hideToPrint" onClick={loadEvent} />
          {event.logo && <ClearIcon className="hideToPrint" onClick={() => removeLogo() } />}
        </Typography>
      </Grid>

      {/* Headers First Col */}
      <Grid item xs={12} md={4} sx={{ paddingBottom: 4 }}>

        {/* NAME */}
        <Typography>
          <Editable
            text={tempEvent.name}
            title="Name"
            onChange={updateEvent}
            onCancel={(e) => {
              setTempEvent(event)
            }}
            childRef={nameRef}
          >
                    <TextField
                      fullWidth name="name" label='Name' placeholder='Name' defaultValue={tempEvent.name} inputProps={ {ref:nameRef} }
                      onChange={(e) => {
                        tempEvent.name = e.target.value
                        setTempEvent(tempEvent)
                      }}
                    />
          </Editable>
        </Typography>

        {/* CODE */}
        <Typography>
          <Editable
            text={tempEvent.code}
            title="Code"
            onChange={updateEvent}
            onCancel={(e) => {
              setTempEvent(event)
            }}
            childRef={codeRef}
          >
                    <TextField
                      fullWidth name="code" label='Code' placeholder='Code' defaultValue={tempEvent.code} inputProps={ {ref:codeRef} }
                      onChange={(e) => {
                        tempEvent.code = e.target.value
                        setTempEvent(tempEvent)
                      }}
                    />
          </Editable>
        </Typography>

        {/* START DATE */}
        <Typography>
          <Editable
            text={tempEvent.start_date}
            title="Start date"
            onChange={updateEvent}
            onCancel={(e) => {
              setTempEvent(event)
            }}
            childRef={startDateRef}
          >
            <TextField
              fullWidth name="start_date" label='Start date' placeholder='Start date' defaultValue={tempEvent.start_date} inputProps={ {ref:startDateRef} }
              onChange={(e) => {
                tempEvent.start_date = e.target.value
                setTempEvent(tempEvent)
              }}
            />
          </Editable>
        </Typography>

        {/* END DATE */}
        <Typography>
          <Editable
            text={tempEvent.end_date}
            title="End date"
            onChange={updateEvent}
            onCancel={(e) => {
              setTempEvent(event)
            }}
            childRef={endDateRef}
          >
            <TextField
              fullWidth name="end_date" label='End date' placeholder='End date' defaultValue={tempEvent.end_date} inputProps={ {ref:endDateRef} }
              onChange={(e) => {
                tempEvent.end_date = e.target.value
                setTempEvent(tempEvent)
              }}
            />
          </Editable>
        </Typography>
      </Grid>

      {/* Headers 2nd Col */}
      <Grid item xs={12} md={4} sx={{ paddingBottom: 4 }}>

        {/* LOCATION */}
        <Typography>
          <Editable
            text={tempEvent.location}
            title="Location"
            onChange={updateEvent}
            onCancel={(e) => {
              setTempEvent(event)
            }}
            childRef={locationRef}
          >
            <TextField
              fullWidth name="location" label='Location' placeholder='Location' defaultValue={tempEvent.location} inputProps={ {ref:locationRef} }
              onChange={(e) => {
                tempEvent.location = e.target.value
                setTempEvent(tempEvent)
              }}
            />
          </Editable>
        </Typography>

        {/* COUNTRY */}
        <Typography>
          <Editable
            text={tempEvent.country || "none"}
            title="Country"
            onChange={updateEvent}
            onCancel={(e) => {
              setTempEvent(event)
            }}
            childRef={countryRef}
          >
            <Autocomplete
              disablePortal
              id="autoeventlete-country"
              options={countryListAllIsoData}
              getOptionLabel={c => `${c.name} (${c.code3})`}
              defaultValue={countryListAllIsoData.find(c => c.code3.toLowerCase() == tempEvent.country)}
              renderInput={(params) => <TextField {...params} name="country" label="Country"/>}
              onChange={(e, v) => {
                tempEvent.country = v?.code3?.toLowerCase()
                setTempEvent(tempEvent)
              }}
               inputProps={ {ref:countryRef} }
            />
          </Editable>
        </Typography>

        {/* WEBSITE */}
        <Typography>
          <Editable
            text={tempEvent.website || "none"}
            title="Website"
            onChange={updateEvent}
            onCancel={(e) => {
              setTempEvent(event)
            }}
            childRef={websiteRef}
          >
                    <TextField
                      fullWidth name="website" label='Website' placeholder='Websites' defaultValue={tempEvent.website} inputProps={ {ref:websiteRef} }
                      onChange={(e) => {
                        tempEvent.website = e.target.value ? e.target.value : null
                        setTempEvent(tempEvent)
                      }}
                    />
          </Editable>
        </Typography>

        {/* STREAMING URL */}
        <Typography>
          <Editable
            text={tempEvent.streaming_url || "none"}
            title="Streaming URL"
            onChange={updateEvent}
            onCancel={(e) => {
              setTempEvent(event)
            }}
            childRef={streamingUrlRef}
          >
                    <TextField
                      fullWidth name="streaming_url" label='Streaming URL' placeholder='Streaming URL' defaultValue={tempEvent.streaming_url} inputProps={ {ref:streamingUrlRef} }
                      onChange={(e) => {
                        tempEvent.streaming_url = e.target.value ? e.target.value : null
                        setTempEvent(tempEvent)
                      }}
                    />
          </Editable>
        </Typography>
      </Grid>

      {/* Headers 3rd Col*/}
      <Grid item xs={12} md={4} sx={{ paddingBottom: 4 }}>

        {/* IMAGE */}
        <Typography>
          <input
            style={{display: 'none'}}
            ref={inputImage}
            type="file"
            onChange={uploadImage}
          />
          <Avatar src={`${process.env.NEXT_PUBLIC_API_URL}/files/${event.image}`} onClick={updateImage} variant="square"><NoPhotographyIcon /></Avatar>
          { event.image && <ClearIcon className="hideToPrint" onClick={() => removeImage() } />}
        </Typography>

        {/* DESCRIPTION */}
        <Typography>
          <Editable
            text={tempEvent.description || "none"}
            title="Description"
            onChange={updateEvent}
            onCancel={(e) => {
              setTempEvent(event)
            }}
            childRef={descriptionRef}
          >
                    <TextField
                      fullWidth name="description" label='Description' placeholder='Description' defaultValue={tempEvent.description} inputProps={ {ref:descriptionRef} }
                      multiline={true} minRows={4}
                      onChange={(e) => {
                        tempEvent.description = e.target.value ? e.target.value : null
                        setTempEvent(tempEvent)
                      }}
                    />
          </Editable>
        </Typography>
      </Grid>
      <Grid item xs={12}>
        <h2>Competitions</h2>
        <Card>
          <EnhancedTable rows={competitions} headCells={headCells} orderById='name' orderId='asc' ppagination={false}/>
        </Card>
      </Grid>
    </Grid>
  )
}

export default EventPage
