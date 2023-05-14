// ** React Imports
import { useEffect, useState } from 'react'

// ** MUI Imports
import Grid from '@mui/material/Grid'
import EnhancedTable from 'src/views/tables/EnhancedTable'
import CardContent from '@mui/material/CardContent'
import Button from '@mui/material/Button'
import AddIcon from '@mui/icons-material/Add'
import Autocomplete from '@mui/material/Autocomplete';
import TextField from '@mui/material/TextField'
import Box from '@mui/material/Box'
import Link from '@mui/material/Link'
import Avatar from '@mui/material/Avatar'
import DeleteIcon from '@mui/icons-material/Delete'

// ** local imports
import {usePilots} from 'src/util/backend'

const TabCompetitions = ({competitions}) => {
  // ** State
  const [value, setValue] = useState([])

  const headCells = [
    {
      id: 'name',
      type: 'LINK',
      href: (v, comp) => `/competitions/show?cid=${comp.code}`,
    },
    {
      id: 'type',
    },
    {
      id: 'start_date',
    },
    {
      id: 'location',
    },
    {
      id: 'number_of_runs',
    }
  ]

  useEffect(() =>{
  }, [])

  return (
    <CardContent>
      <Grid container spacing={7}>
        <Grid item xs={12} sm={12}>
          <EnhancedTable rows={competitions} headCells={headCells} orderById='rank' pagination={false}/>
        </Grid>
      </Grid>
    </CardContent>
  )
}

export default TabCompetitions
