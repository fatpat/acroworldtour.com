// ** React Imports
import { useEffect, useState } from 'react'

// ** MUI Imports
import Grid from '@mui/material/Grid'
import CardContent from '@mui/material/CardContent'
import Button from '@mui/material/Button'
import AddIcon from '@mui/icons-material/Add'
import Autocomplete from '@mui/material/Autocomplete';
import TextField from '@mui/material/TextField'
import Box from '@mui/material/Box'
import MenuItem from '@mui/material/MenuItem'
import Link from '@mui/material/Link'
import Avatar from '@mui/material/Avatar'
import Dialog from '@mui/material/Dialog'
import DialogTitle from '@mui/material/DialogTitle'
import DialogContent from '@mui/material/DialogContent'
import DialogActions from '@mui/material/DialogActions'

// ** local imports
import EnhancedTable from 'src/views/tables/EnhancedTable'
import { APIRequest } from 'src/util/backend'
import { useNotifications } from 'src/util/notifications'


const TabRuns = ({comp, refresh}) => {
  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** State
  const [runs, setRuns] = useState([])
  const [newRunOpen, setNewRunOpen] = useState(false)
  const [pilotsToQualify, setPilotsToQualify] = useState(0)
  // by default set repetitionsResetPolicy to 0 (None), to 1 for every 5 awt runs, to 2 for every 2 AWQ runs
  const [repetitionsResetPolicy, setRepetitionsResetPolicy] = useState(
    (comp.seasons.filter((s) => s.match(/^awt-/)).length > 0 && comp.runs.length > 3 && comp.runs.length % 5 == 4 ? 1 : 0)
    +
    (comp.seasons.filter((s) => s.match(/^awq-/)).length > 0 && comp.runs.length > 1 && comp.runs.length % 2 == 0 ? 2 : 0)
  )

  const createNewRun = async() => {

    const [err, retData, headers] = await APIRequest(`/competitions/${comp.code}/runs/new?pilots_to_qualify=${pilotsToQualify}&repetitions_reset_policy=${repetitionsResetPolicy}`, {
        expected_status: 201,
        method: 'POST',
    })

    if (err) {
      error(`error while creating new run ${comp.code}: ${err}`)
      return
    }
    refresh()
  }

  const headCells = [
    {
      id: 'name',
      type: 'LINK',
      href: (name, run) => {
        return `/competitions/run?cid=${comp.code}&rid=${run.id}`
      },
    },
    {
      id: 'state',
    },
    {
      id: 'repetitions_reset_policy',
    }
  ]

  useEffect(() =>{
    setRuns(comp.runs.map((r,i) => {
      r.delete = 'delete'
      r.id = i
      r.name = `run #${i+1}`
      return r
    }))
  }, [])

  return (
    <CardContent>
    <Dialog
      sx={{ '& .MuiDialog-paper': { width: '80%', maxHeight: 435 } }}
      maxWidth="xs"
      TransitionProps={{ onEntering: () => {} }}
      open={newRunOpen}
    >
      <DialogTitle>New Run</DialogTitle>
      <DialogContent dividers>
          <TextField
            name="pilots-to-qualify"
            label="Pilots to Qualify"
            placeholder="Pilots to Qualify"
            fullWidth={true}
            defaultValue={pilotsToQualify}
            type="number"
            required={true}
            onChange={e => {setPilotsToQualify(e.target.value)}}
          />
          <TextField
            name="repetition-reset-policy"
            label="Repetition Reset Policy"
            placeholder="Repetition Reset Policy"
            fullWidth={true}
            defaultValue={repetitionsResetPolicy}
            select={true}
            required={true}
            onChange={e => {setRepetitionsResetPolicy(e.target.value)}}
          >
            <MenuItem value={0}>None</MenuItem>
            <MenuItem value={1}>Only for AWT pilots</MenuItem>
            <MenuItem value={2}>Only for AWQ pilots</MenuItem>
            <MenuItem value={3}>For all pilots</MenuItem>
          </TextField>
      </DialogContent>
      <DialogActions>
        <Button autoFocus onClick={() => {setNewRunOpen(false)}}>
          Cancel
        </Button>
        <Button onClick={() => {createNewRun()}}>Ok</Button>
      </DialogActions>
    </Dialog>
      <Grid container spacing={7}>
        <Grid item xs={6} sm={6}>
          <Button variant='contained' startIcon={<AddIcon />} onClick={() => {setNewRunOpen(true)}}>
            New Run
          </Button>
        </Grid>
        <Grid item xs={12} sm={12}>
          <EnhancedTable rows={runs} headCells={headCells} orderById='id' pagination={false}/>
        </Grid>
      </Grid>
    </CardContent>
  )
}

export default TabRuns
