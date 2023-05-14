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
import {useTricks} from 'src/util/backend'

const TabRepeatableTricks = ({tricks, allTricks, update}) => {
  // ** State
  const [value, setValue] = useState([])

  const removeRepeatableTrick = async(id) => {
    if (!id) return
    const name = allTricks.filter(j => j.id == id)[0].name
    if (!confirm(`Are you sure you want to remove repeatable_trick ${name} (${id}) ?`)) return
    update(tricks.filter(p => p.id != id))
  }

  const headCells = [
    {
      id: 'name',
    },
    {
      id: 'acronym',
    },
    {
      id: 'delete',
      type: 'ACTION',
      func: removeRepeatableTrick,
      rewrite: (v) => { return <DeleteIcon /> }
    }
  ]

  useEffect(() =>{
    tricks = tricks.map(p => {
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
                      id="autocomplete-tricks"
                      options={allTricks.filter(p => tricks.filter(p2 => p2.name == p.name).length == 0)}
                      getOptionLabel={(p) => `${p.name}`}
                      value={value}
                      renderInput={(params) => <TextField {...params} name="tricks" label="Tricks" onKeyPress={(e) => {
                          e.key === 'Enter' && update(value.concat(tricks))
                      }}/>}
                      onChange={(e, v) => {
                        setValue(v)
                      }}
                    />
        </Grid>
        <Grid item xs={6} sm={6}>
          <Button variant='contained' onClick={() => {update(value.concat(tricks))}}><AddIcon /></Button>
        </Grid>
        <Grid item xs={12} sm={12}>
          <EnhancedTable rows={tricks} headCells={headCells} orderById='technical_coefficient' pagination={false}/>
        </Grid>
      </Grid>
    </CardContent>
  )
}

export default TabRepeatableTricks
