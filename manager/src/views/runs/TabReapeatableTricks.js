// ** React Imports
import { forwardRef, useState } from 'react'

// ** MUI Imports
import Grid from '@mui/material/Grid'
import EnhancedTable from 'src/views/tables/EnhancedTable'
import CardContent from '@mui/material/CardContent'
import Button from '@mui/material/Button'
import AddIcon from '@mui/icons-material/Add'

const headCells = [
  {
    id: 'id',
    numeric: false,
    disablePadding: true,
    label: 'Id'
  },
  {
    id: 'name',
    numeric: false,
    disablePadding: false,
    label: 'Name'
  }
]

function createData(id, name) {
  return {
    id,
    name
  }
}

const TabReapeatableTricks = ({ tricks }) => {
  // ** State
  const [date, setDate] = useState(null)

  return (
    <CardContent>
      <Grid container spacing={7}>
        <Grid item xs={12} sm={12} container>
          <Button variant='contained' startIcon={<AddIcon />}>
            {' '}
            Add trick
          </Button>
        </Grid>
        <Grid item xs={12} sm={12}>
          <EnhancedTable rows={tricks.map(j => createData(j, ''))} headCells={headCells} orderById='rank' pagination={false}/>
        </Grid>
      </Grid>
    </CardContent>
  )
}

export default TabReapeatableTricks
