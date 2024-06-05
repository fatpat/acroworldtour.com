// ** React Imports
import { useEffect, useState, useRef } from 'react'
import { useRouter } from 'next/router'

// ** MUI Imports
import Grid from '@mui/material/Grid'
import MenuItem from '@mui/material/MenuItem'
import Select from '@mui/material/Select'
import Button from '@mui/material/Button'
import TableBody from '@mui/material/TableBody'
import TableCell from '@mui/material/TableCell'
import TableRow from '@mui/material/TableRow'
import TableHead from '@mui/material/TableHead'
import Table from '@mui/material/Table'
import TableContainer from '@mui/material/TableContainer'
import Paper from '@mui/material/Paper'
import IconButton from '@mui/material/IconButton'
import Box from '@mui/material/Box'
import TextField from '@mui/material/TextField'
import Typography from '@mui/material/Typography'
import CardContent from '@mui/material/CardContent'
import NavigateNextIcon from '@mui/icons-material/NavigateNext'
import AddIcon from '@mui/icons-material/Add'
import NavigateBeforeIcon from '@mui/icons-material/NavigateBefore'
import Editable from 'src/components/Editable'
import Autocomplete from '@mui/material/Autocomplete'
import NativeSelect from '@mui/material/NativeSelect'
import LinearProgress from '@mui/material/LinearProgress'
import Checkbox from '@mui/material/Checkbox'
import DeleteIcon from '@mui/icons-material/Delete'

// ** local
import EnhancedTable from 'src/views/tables/EnhancedTable'
import { useNotifications } from 'src/util/notifications'
import { APIRequest, useUniqueTricks } from 'src/util/backend'
import InputMark from 'src/components/InputMark'

const TabFlights = ({ comp, run, rid }) => {
  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** states
  const [loadingMessage, setLoading] = useState(null)
  const [currentFlight, setCurrentFlight] = useState(0)
  const [pilot, setPilot] = useState(null)
  const [data, setData] = useState({
    tricks: [],
    marks: [],
    did_not_start: false,
    warnings: []
  })
  const [result, setResult] = useState({
      judges_mark:{}
  })
  const [uniqueTricks] = useUniqueTricks(comp.type)
  const [resultsOK, setResultsOK] = useState(false)
  const [published, setPublished] = useState(false)

  // ** refs
  const nameRef = useRef()

  const loadPilot = async(i) => {
    if (i<0 || i>=run.pilots.length) return
    currentFlight = i
    pilot = run.pilots[currentFlight]
    setLoading(`Loading flight for ${pilot.name}`)

    const [err, retData, headers, status] = await APIRequest(`/competitions/${comp.code}/runs/${rid}/flights/${pilot.civlid}`, {
      expected_status: [200, 404]
    })
    if (err) {
        console.log(`error while fetching flight: ${err}`)
        setLoading(null)
        return
    }
    if (status == 404) {
      data = {
        published: false,
        marks: [],
        tricks: []
      }
      result = {
        judges_mark:{}
      }
      resultsOK = false
    } else {
      data = retData
      result = retData.final_marks
      resultsOK = true
    }

    setData(data)
    setResult(result)
    setResultsOK(resultsOK)
    setPilot(pilot)
    setCurrentFlight(currentFlight)
    setPublished(resultsOK ? data.published : false)
    setLoading(null)
  }

  const prevPilot = () => {
    loadPilot(currentFlight-1)
  }

  const nextPilot = () => {
    loadPilot(currentFlight+1)
  }

  const simulateScore = async(data) => {

    const body = {
      tricks: data.tricks.filter(t => t!=null).map(t => t.name),
      marks: data.marks,
      did_not_start: data.did_not_start,
      warnings: data.warnings,
    }

    const [err, retData, headers] = await APIRequest(`/competitions/${comp.code}/runs/${rid}/flights/${pilot.civlid}/new?save=${false}`, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(body),
    })

    if (err) {
        setResult({
          judges_mark:{},
          notes: [err],
        })
        setResultsOK(false)
        warning(err)
        return
    }
    setResult(retData)
    setResultsOK(true)
  }

  const setMark = (type, judge, mark) => {
    for (const [i, m] of data.marks.entries()) {
        if (data.marks[i].judge == judge._id) {
          if (isNaN(mark)) {
            delete data.marks[i][type]
          } else {
            data.marks[i][type] = mark
          }
          setData(data)
          simulateScore(data)
          return
        }
    }
    var m = {judge: judge._id}
    m[type] = mark
    data.marks.push(m)
    setData(data)
    simulateScore(data)
  }

  const saveResults = async(publish, next) => {

    const body = {
      tricks: data.tricks.filter(t => t!=null).map(t => t.name),
      marks: data.marks,
      did_not_start: data.did_not_start,
      warnings: data.warnings,
    }

    const [err, retData, headers] = await APIRequest(`/competitions/${comp.code}/runs/${rid}/flights/${pilot.civlid}/new?published=${publish}&save=${true}`, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(body),
    })

    if (err) {
        error(`error while saving score: ${err}`)
        return
    }

    setPublished(publish)

    success(`${pilot.name}'s flights saved with a ${retData.score} score`)
    if (next != 0) loadPilot(currentFlight + next)
  }

  const didNotStart = async(e) => {
    if (!confirm('Are you sure to publish a DNS flight ?')) return
    data.did_not_start = true
    saveResults(true, false)
  }

  const addWarning = async(e, warning) => {
    if (!warning || warning === "") warning = prompt("Warning message")
    if (!warning || warning === "") return
    data.warnings.push(warning)
    setData(data)
    simulateScore(data)
  }

  const removeWarning = async(i) => {
    if (i<0 || i>= data.warnings.length) return
    data.warnings.splice(i, 1)
    setData(data)
    simulateScore(data)
  }

  const deleteRun = async(e) => {
    if (!confirm(`Are you sure to delete run #${rid} of ${pilot.name}  ?`)) return
    if (!confirm(`Are you REALLY sure to delete run #${rid} of ${pilot.name}  ?`)) return

    const [err, retData, headers] = await APIRequest(`/competitions/${comp.code}/runs/${rid}/flights/${pilot.civlid}`, {
      method: 'DELETE',
      expected_status: [204],
    })

    if (err) {
        error(`error while deleting run #${rid} of ${pilot.name}: ${err}`)
        return
    }
    success(`run #${rid} of ${pilot.name} has been deleted`)
    loadPilot(currentFlight)
  }

  useEffect(() => {
    loadPilot(0)
  }, [])

  if (loadingMessage) {
    return (
      <Box sx={{ width: '100%', textAlign: 'center' }}>
        <LinearProgress />
        {loadingMessage}
      </Box>
    )
  }

  let has_technical_marks_per_trick = false
  data.marks.forEach((m) => {
    console.log(m)
    if (m.technical_per_trick != null) has_technical_marks_per_trick = true
  })

  return (
    <CardContent>
{ pilot &&
      <Grid container>
        <Grid item xs={1} sm={1}>
            <IconButton onClick={prevPilot} >
              <NavigateBeforeIcon />
            </IconButton>
        </Grid>
        <Grid item xs={10} sm={10}>
                    <Autocomplete
                      id="autocomplete-pilot"
                      options={run.pilots}
                      value={pilot}
                      getOptionLabel={(p) => `${p.name} (${p.civlid})`}
                      renderInput={(params) => <TextField {...params} name="pilot" label="Pilot" />}
                      onChange={(e, v) => {
                        if (!v) return
                        for(const [i,p] of run.pilots.entries()){
                            if (p.civlid == v.civlid) {
                                loadPilot(i)
                                return
                            }
                        }
                      }}
                    />
        </Grid>
        <Grid item xs={1} sm={1}>
            <IconButton onClick={nextPilot} >
              <NavigateNextIcon />
            </IconButton>
        </Grid>
      </Grid>
}
{ has_technical_marks_per_trick && <Typography variant="h5">The run has already been marked with technichal mark per trick. Please use the "Flight Details" tab instead.</Typography> }
{ has_technical_marks_per_trick || <>
      <Grid container spacing={2}>
        &nbsp;
      </Grid>
      <Grid container spacing={2}>
        {/* 1st column / maneuvers*/}
        <Grid item xs={6}>
            <Grid item xs={12}>
              <Typography variant="h5">Maneuvers</Typography>
            </Grid>
{ [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20].map(i =>{
            var trick = null
            if (data && data.tricks && data.tricks[i]) trick = data.tricks[i]
            return(
            <Grid item xs={12} key={i}>
                    <Autocomplete
                      id="autocomplete-trick-{i}"
                      key={i}
                      options={uniqueTricks}
                      groupBy={t => t.base_trick}
                      getOptionLabel={(p) => `${p.name} (${p.acronym}) (${p.technical_coefficient})`}
                      renderInput={(params) => <TextField {...params} name="trick" />}
                      value={trick}
                      isOptionEqualToValue={(a,b) => a.acronym == b.acronym}
                      onChange={(e, v) => {
                          data.tricks[i] = v
                          simulateScore(data)
                          setData(data)
                      }}
                    />
            </Grid>
)})}
        </Grid>
        {/* 2nd column */}
        <Grid item xs={6}>
          {/* marks */}
          <Grid item xs={12}>
            <Typography variant="h5">Marks</Typography>
    <Box sx={{ width: '100%' }}>
      <Paper sx={{ width: '100%', mb: 2 }}>
        <TableContainer>
          <Table sx={{ minWidth: 750 }}>
    <TableHead>
      <TableRow>
          <TableCell>
            Judge
          </TableCell>
          <TableCell>
            Technical
          </TableCell>
          <TableCell>
            Choreography
          </TableCell>
          <TableCell>
            Landing
          </TableCell>
      </TableRow>
    </TableHead>
            <TableBody>
{ run.judges.map((j) => {
    var technical = null
    var choreography = null
    var landing = null
    for (const m in data.marks) {
      m = data.marks[m]
      if (m.judge == j._id) {
        technical = m.technical
        choreography = m.choreography
        landing = m.landing
        break
      }
    }
    return (
              <TableRow key={j._id}>
                <TableCell>
                  <Typography>{ j.name }</Typography>
                </TableCell>
                <TableCell>
                  <InputMark onChange={value => {setMark('technical', j, value)}} value={technical} />
                </TableCell>
                <TableCell>
                  <InputMark onChange={value => {setMark('choreography', j, value)}} value={choreography} />
                </TableCell>
                <TableCell>
                  <InputMark onChange={value => {setMark('landing', j, value)}} value={landing} />
                </TableCell>
            </TableRow>
)})}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>
    </Box>
          </Grid>
          {/* scores */}
          <Grid item xs={12}>
            <Typography variant="h5">Scores</Typography>
            <Table>
              <TableBody>
                <TableRow>
                  <TableCell>
                    <Typography>Technicity: {result.technicity ?? ""}</Typography>
                  </TableCell>
                  <TableCell>
                    <Typography>% Bonus: {result.bonus_percentage ?? ""}%</Typography>
                  </TableCell>
                  <TableCell>
                    <Typography>Published? <Checkbox disabled checked={published} /></Typography>
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell>
                    <Typography>Judge's marks</Typography>
                  </TableCell>
                  <TableCell>
                    <Typography>Technical: {result.judges_mark.technical ?? ""}</Typography>
                  </TableCell>
                  <TableCell>
                    <Typography>Choreography: {result.judges_mark.choreography ?? ""}</Typography>
                  </TableCell>
                  <TableCell>
                    <Typography>Landing: {result.judges_mark.landing ?? ""}</Typography>
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell>
                    <Typography>Final's marks</Typography>
                  </TableCell>
                  <TableCell>
                    <Typography>Technical: {result.technical ?? ""}</Typography>
                  </TableCell>
                  <TableCell>
                    <Typography>Choreography: {result.choreography ?? ""}</Typography>
                  </TableCell>
                  <TableCell>
                    <Typography>Landing: {result.landing ?? ""}</Typography>
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell>
                    <Typography>Bonuses: {result.bonus ?? ""}</Typography>
                  </TableCell>
                  <TableCell>
                    <Typography variant="h5">Final Score: {result.score ?? ""}</Typography>
                  </TableCell>
                  <TableCell colSpan={2}>
  { result.notes && result.notes.length > 0 && (<>
                    <Typography>notes:</Typography>
  <ul>
  { result.notes.map((note, i) => <li key={i}>{note}</li>)}
  </ul>
  </>)}
  { result.warnings && result.warnings.length > 0 && (<>
                    <Typography>warnings:</Typography>
  <ul>
  { result.warnings.map((warning, i) => <li key={i}>{warning} <DeleteIcon onClick={() => removeWarning(i)} /></li>)}
  </ul>
  </>)}
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </Grid>
          {/* actions */}
          <Grid container>
              <Grid item xs={4}>
                <Button variant="contained" disabled={!resultsOK} onClick={e => saveResults(false, 0)}>Save</Button>
                <Button variant="contained" disabled={!resultsOK} onClick={e => saveResults(false, 1)}>Save & Next</Button>
              </Grid>
              <Grid item xs={4}>
                <Button variant="contained" disabled={!resultsOK} onClick={e => saveResults(true, 0)}>Save & Publish</Button>
                <Button variant="contained" disabled={!resultsOK} onClick={e => saveResults(true, 1)}>Save & Publish & Next</Button>
              </Grid>
              <Grid item xs={4}>
                <Button onClick={didNotStart}>Did not start</Button>
                <Button onClick={addWarning}>Add warning</Button>
                <Button onClick={e => addWarning(e, "flight over the public")}>flight over the public</Button>
                <Button onClick={deleteRun}>Delete</Button>
              </Grid>
          </Grid>
        </Grid>
      </Grid>
</>}
    </CardContent>
  )
}

export default TabFlights
