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

const UniqueTricksPage = () => {
  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** auth/user
  const { user, authError, authIisLoading } = useUser();

  // ** local
  const [data, setData] = useState([])
  const [fullData, setFullData] = useState([])
  const [isLoading, setLoading] = useState(false)

  const loadTricks = async () => {
    setLoading(true)

    const [err, data, headers] = await APIRequest('/tricks/uniques', {expect_json: true})

    if (err) {
        setData(false)
        setFullData(false)
        error(`Error while retrieving tricks list: ${err}`)
        return
    }

    setData(data)
    setFullData(data)
    setLoading(false)
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
      id: 'technical_coefficient',
      numeric: true,
    },
    {
      id: 'bonus',
      numeric: true,
      rewrite: (b) => { return `${b}%`},
      label: "Bonus (%)",
    },
    {
      id: 'bonus_types',
      rewrite: (types) => { return types.join(', ')},
    },
    {
      id: 'base_trick',
    }
  ]

  useEffect(() => {
      loadTricks()
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
      <Grid item xs={6} sm={6}>
        <TextField fullWidth id='outlined-basic' label='Search trick' variant='outlined' onChange={updateSearch} />
      </Grid>
      <Grid item xs={12}>
        <Card>
          <EnhancedTable rows={data} headCells={headCells} orderById='technical_coefficient' pagination={false}/>
        </Card>
      </Grid>
    </Grid>
  )
}

export default withPageAuthRequired(UniqueTricksPage)
