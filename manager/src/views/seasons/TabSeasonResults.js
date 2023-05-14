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

const TabSeasonResults = ({season}) => {
  // ** State
  const [value, setValue] = useState([])

  const headCellsSolo = [
    {
      id: 'rank'
    },
    {
      id: 'pilot',
      rewrite: (p) => {return(
          <Box>
            <Link href={p.link} target="_blank" rel="noopener noreferrer"><Avatar alt={p.name} src={p.photo} />{p.name}</Link>
          </Box>
      )}
    },
    {
      id: 'score',
    }
  ]

  const headCellsSynchro = [
    {
      id: 'rank'
    },
    {
      id: 'team',
      rewrite: (t) => t.name,
    },
    {
      id: 'pilots',
      rewrite: (v, result) => {
        v = result.team.pilots
        return (
          <Box>
            <Link href={v[0].link} target="_blank" rel="noopener noreferrer"><Avatar alt={v[0].name} src={v[0].photo} />{v[0].name}</Link>
            <Link href={v[0].link} target="_blank" rel="noopener noreferrer"><Avatar alt={v[1].name} src={v[1].photo} />{v[1].name}</Link>
          </Box>
        )
      },
    },
    {
      id: 'score',
    }
  ]

  useEffect(() =>{
  }, [])

  return (
    <CardContent>
      <Grid container spacing={7}>
{ season.results.map(results => (
        <Grid item xs={12} sm={12}>
          <h3>{results.type}</h3>
          <EnhancedTable rows={results.results.map((result, rank) => {result.rank = rank+1; return result})} headCells={season.type =="solo" ? headCellsSolo : headCellsSynchro} orderById='score' defaultOrder="desc" pagination={false}/>
        </Grid>
))}
      </Grid>
    </CardContent>
  )
}

export default TabSeasonResults
