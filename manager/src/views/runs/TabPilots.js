// ** React Imports
import { forwardRef, useState } from 'react'

// ** MUI Imports

import Grid from '@mui/material/Grid'
import EnhancedTable from 'src/views/tables/EnhancedTable'
import CardContent from '@mui/material/CardContent'
import Button from '@mui/material/Button'
import AddIcon from '@mui/icons-material/Add'

function createData(civlid) {
  return {
    civlid
  }
}

const TabPilots = ({ pilots }) => {
  // ** State
  const [date, setDate] = useState(null)

  const headCells = [
    {
      id: 'civlid',
      numeric: false,
      disablePadding: true,
      label: 'CIVL ID'
    },
    {
      id: 'name',
      numeric: false,
      disablePadding: false,
      label: 'Name'
    },
    {
      id: 'country',
      numeric: false,
      disablePadding: false,
      label: 'Country'
    },
    {
      id: 'link',
      numeric: false,
      link: true,
      disablePadding: false,
      label: 'Link'
    }
  ]

  return (
    <CardContent>
      <Grid container spacing={7}>
        <Grid item xs={12} sm={12} container>
          <Button variant='contained' startIcon={<AddIcon />}>
            {' '}
            Add pilot
          </Button>
        </Grid>
        <Grid item xs={12} sm={12}>
          <EnhancedTable rows={pilots.map(p => createData(p))} headCells={headCells} orderById='rank' pagination={false}/>
        </Grid>
      </Grid>
    </CardContent>
  )
}

export default TabPilots
