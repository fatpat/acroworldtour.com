// ** React Imports
import { useEffect, useState } from 'react'

// ** MUI Imports
import Grid from '@mui/material/Grid'
import CardContent from '@mui/material/CardContent'
import Button from '@mui/material/Button'
import AddIcon from '@mui/icons-material/Add'
import Autocomplete from '@mui/material/Autocomplete'
import TextField from '@mui/material/TextField'
import Box from '@mui/material/Box'
import Avatar from '@mui/material/Avatar'
import Link from '@mui/material/Link'
import DeleteIcon from '@mui/icons-material/Delete'

// ** local imports
import {useTeams} from 'src/util/backend'
import EnhancedTable from 'src/views/tables/EnhancedTable'

const TabTeams = ({teams, allTeams, update}) => {
  // ** State
  const [value, setValue] = useState([])

  const removeTeam = async(id) => {
    if (!id) return
    const name = allTeams.filter(p => p._id == id)[0].name
    if (!confirm(`Are you sure you want to remove team ${name} (${id}) ?`)) return
    update(teams.filter(p => p.id != id))
  }

  const headCells = [
    {
      id: 'name',
    },
    {
      id: 'pilots',
      rewrite: (v) => {
        return (
          <Box>
            <Link href={v[0].link} target="_blank" rel="noopener noreferrer"><Avatar alt={v[0].name} src={v[0].photo} />{v[0].name}</Link>
            <Link href={v[0].link} target="_blank" rel="noopener noreferrer"><Avatar alt={v[1].name} src={v[1].photo} />{v[1].name}</Link>
          </Box>
        )
      },
    },
    {
      id: 'delete',
      type: 'ACTION',
      func: removeTeam,
      rewrite: (v) => { return <DeleteIcon /> }
    }
  ]

  useEffect(() =>{
    teams = teams.map(p => {
      p.delete = 'delete'
      p.id = p._id
      return p
    })
  }, [])

  return (
    <CardContent>
      <Grid container spacing={7}>
{ allTeams &&
<>
        <Grid item xs={6} sm={6}>
                    <Autocomplete
                      multiple
                      disablePortal
                      id="autocomplete-teams"
                      options={allTeams.filter(p => teams.filter(p2 => p2.id == p.id).length == 0)}
                      getOptionLabel={(p) => `${p.name}`}
                      value={value}
                      renderInput={(params) => <TextField {...params} name="teams" label="Teams" onKeyPress={(e) => {
                          e.key === 'Enter' && update(value.concat(teams))
                      }}/>}
                      onChange={(e, v) => {
                        setValue(v)
                      }}
                    />
        </Grid>
        <Grid item xs={6} sm={6}>
          <Button variant='contained' onClick={() => {update(value.concat(teams))}}><AddIcon /></Button>
        </Grid>
</>
}
        <Grid item xs={12} sm={12}>
          <EnhancedTable rows={teams} headCells={headCells} orderById='rank' pagination={false}/>
        </Grid>
      </Grid>
    </CardContent>
  )
}

export default TabTeams
