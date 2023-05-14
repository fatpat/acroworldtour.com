// ** React Imports
import { useEffect, useState } from 'react'

// ** MUI Imports
import Grid from '@mui/material/Grid'
import EnhancedTable from 'src/views/tables/EnhancedTable'
import CardContent from '@mui/material/CardContent'
import Button from '@mui/material/Button'
import AddIcon from '@mui/icons-material/Add'
import Autocomplete from '@mui/material/Autocomplete'
import TextField from '@mui/material/TextField'
import DeleteIcon from '@mui/icons-material/Delete'

// ** local imports
import {useJudges} from 'src/util/backend'

const TabJudges = ({judges, allJudges, update}) => {
  // ** State
  const [value, setValue] = useState([])

  const removeJudge = async(id) => {
    if (!id) return
    const name = allJudges.filter(j => j._id == id)[0].name
    if (!confirm(`Are you sure you want to remove judge ${name} (${id}) ?`)) return
    update(judges.filter(p => p.id != id))
  }

  const headCells = [
    {
      id: 'name',
    },
    {
      id: 'country',
    },
    {
      id: 'level',
    },
    {
      id: 'delete',
      type: 'ACTION',
      func: removeJudge,
      rewrite: (v) => { return <DeleteIcon /> }
    }
  ]

  useEffect(() =>{
    console.log('useEffect()')
    judges = judges.map(p => {
      p.delete = 'delete'
      p.id = p._id
      return p
    })
  }, [])

  return (
    <CardContent>
      <Grid container spacing={7}>
        <Grid item xs={6} sm={6}>
                    <Autocomplete
                      multiple
                      disablePortal
                      id="autocomplete-judges"
                      options={allJudges.filter(p => judges.filter(p2 => p2.name == p.name).length == 0)}
                      getOptionLabel={(p) => `${p.name}`}
                      value={value}
                      renderInput={(params) => <TextField {...params} name="judges" label="Judges" onKeyPress={(e) => {
                          e.key === 'Enter' && update(value.concat(judges))
                      }}/>}
                      onChange={(e, v) => {
                        setValue(v)
                      }}
                    />
        </Grid>
        <Grid item xs={6} sm={6}>
          <Button variant='contained' onClick={() => {update(value.concat(judges))}}><AddIcon /></Button>
        </Grid>
        <Grid item xs={12} sm={12}>
          <EnhancedTable rows={judges} headCells={headCells} orderById='level' pagination={false}/>
        </Grid>
      </Grid>
    </CardContent>
  )
}

export default TabJudges
