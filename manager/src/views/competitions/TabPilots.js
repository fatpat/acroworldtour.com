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

const TabPilots = ({pilots, allPilots, update}) => {
  // ** State
  const [value, setValue] = useState([])

  const removePilot = async(id) => {
    console.log('removePilot', id)
    if (!id) return
    const name = allPilots.filter(p => p.civlid == id)[0].name
    if (!confirm(`Are you sure you want to remove pilot ${name} (${id}) ?`)) return
    update(pilots.filter(p => p.civlid != id))
  }

  const headCells = [
    {
      id: 'name',
      rewrite: (name, p) => {return(
          <Box>
            <Link href={p.link} target="_blank" rel="noopener noreferrer"><Avatar alt={p.name} src={p.photo} />{p.name}</Link>
          </Box>
      )},
    },
    {
      id: 'country',
    },
    {
      id: 'rank',
    },
    {
      id: "delete",
      type: 'ACTION',
      func: removePilot,
      rewrite: (v) => { return <DeleteIcon /> }
    }
  ]

  useEffect(() =>{
    pilots = pilots.map(p => {
      p.delete = "delete"
      p.id = p.civlid
      return p
    })
  }, [])

  return (
    <CardContent>
      <Grid container spacing={7}>
{ allPilots && (
<>
        <Grid item xs={6} sm={6}>
                    <Autocomplete
                      multiple
                      disablePortal
                      id="autocomplete-pilots"
                      options={allPilots.filter(p => pilots.filter(p2 => p2.civlid == p.civlid).length == 0)}
                      getOptionLabel={(p) => `${p.name} (${p.civlid})`}
                      value={value}
                      renderInput={(params) => <TextField {...params} name="pilots" label="Pilots" onKeyPress={(e) => {
                          e.key === 'Enter' && update(value.concat(pilots))
                      }}/>}
                      onChange={(e, v) => {
                        setValue(v)
                      }}
                    />
        </Grid>
        <Grid item xs={6} sm={6}>
          <Button variant='contained' onClick={() => {update(value.concat(pilots))}}><AddIcon /></Button>
        </Grid>
</>
)}
        <Grid item xs={12} sm={12}>
          <EnhancedTable rows={pilots} headCells={headCells} orderById='rank' pagination={false}/>
        </Grid>
      </Grid>
    </CardContent>
  )
}

export default TabPilots
