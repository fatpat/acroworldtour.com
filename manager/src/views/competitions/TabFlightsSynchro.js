// ** React Imports
import { useEffect, useState, useRef } from 'react'
import { useRouter } from 'next/router'

// ** MUI Imports
import Grid from '@mui/material/Grid'
import NativeSelect from '@mui/material/NativeSelect'
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
import LinearProgress from '@mui/material/LinearProgress'
import DeleteIcon from '@mui/icons-material/Delete'

// ** local
import EnhancedTable from 'src/views/tables/EnhancedTable'
import { useNotifications } from 'src/util/notifications'
import { APIRequest, useUniqueTricks } from 'src/util/backend'
import InputMark from 'src/components/InputMark'

const TabFlightsSynchro = ({ comp, run, rid }) => {
  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** states
  const [loadingMessage, setLoading] = useState(null)
  const [currentFlight, setCurrentFlight] = useState(0)
  const [team, setTeam] = useState(null)
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

  // ** refs
  const nameRef = useRef()

  const loadTeam = async(i) => {
    if (i<0 || i>=run.teams.length) return
    const currentFlight = i
    let team = run.teams[currentFlight]
    setLoading(`Loading flight for ${team.name}`)

    const [err, retData, headers, status] = await APIRequest(`/competitions/${comp.code}/runs/${rid}/flights/${team._id}`, {
      expected_status: [200, 404]
    })
    if (err) {
        console.log(`error while fetching flight: ${err}`)
        setLoading(null)
        return
    }
    let data
    let result
    let resultsOK
    if (status == 404) {
      data = {
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
    console.log("retrieved Data", data)

    setData(data)
    setResult(result)
    setResultsOK(resultsOK)
    setTeam(team)
    setCurrentFlight(currentFlight)
    setLoading(null)
  }


  const prevTeam = () => {
    loadTeam(currentFlight-1)
  }

  const nextTeam = () => {
    loadTeam(currentFlight+1)
  }

  const simulateScore = async(data) => {

    const body = {
      tricks: data.tricks.filter(t => t!=null).map(t => t.name),
      marks: data.marks,
      did_no_start: data.did_not_start,
      warnings: data.warnings,
    }

    const [err, retData, headers] = await APIRequest(`/competitions/${comp.code}/runs/${rid}/flights/${team._id}/new?save=${false}`, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(body),
    })

    if (err) {
        setResult({
          judges_mark:{},
          notes:[],
        })
        setResultsOK(false)
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
      did_no_start: data.did_not_start,
      warnings: data.warnings,
    }

    const [err, retData, headers] = await APIRequest(`/competitions/${comp.code}/runs/${rid}/flights/${team._id}/new?published=${publish}&save=${true}`, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(body),
    })

    if (err) {
        error(`error while saving score: ${err}`)
        return
    }

    success(`${team.name}'s flights saved with a ${retData.score} score`)
    if (next != 0) loadTeam(currentFlight + next)
  }

  const headCells = [
    {
      id: 'rank',
    },
    {
      id: 'Team',
      rewrite: (p) => p && p.name,
    },
    {
      id: 'score',
      numeric: true,
    }
  ]

  const didNotStart = async(e) => {
    if (!confirm('Are you sure to publish a DNS flight ?')) return
    data.did_not_start = true
    saveResults(true)
  }

  const addWarning = async(e, warning) => {
    if (!warning || warning === "") warning = prompt("Warning message")
    if (!warning || warning === "") return
    if (!data.warnings) data.warnings = []
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
    if (!confirm(`Are you sure to delete run #${rid} of ${team.name}  ?`)) return
    if (!confirm(`Are you REALLY sure to delete run #${rid} of ${team.name}  ?`)) return

    const [err, retData, headers] = await APIRequest(`/competitions/${comp.code}/runs/${rid}/flights/${team._id}`, {
      method: 'DELETE',
      expected_status: [204],
    })

    if (err) {
        error(`error while deleting run #${rid} of ${team.name}: ${err}`)
        return
    }
    success(`run #${rid} of ${team.name} has been deleted`)
    loadPilot(currentFlight)
  }

  useEffect(() => {
    loadTeam(0)
  }, [])

  if (loadingMessage) {
    return (
      <Box sx={{ width: '100%', textAlign: 'center' }}>
        <LinearProgress />
        {loadingMessage}
      </Box>
    )
  }

  return (
    <CardContent>
{ team &&
      <Grid container>
        <Grid item xs={1} sm={1}>
            <IconButton onClick={prevTeam} >
              <NavigateBeforeIcon />
            </IconButton>
        </Grid>
        <Grid item xs={10} sm={10}>
                    <Autocomplete
                      id="autocomplete-team"
                      options={run.teams}
                      value={team}
                      getOptionLabel={(t) => `${t.name}`}
                      renderInput={(params) => <TextField {...params} name="team" label="Team" />}
                      onChange={(e, v) => {
                        if (!v) return
                        for(const [i,t] of run.teams.entries()){
                            if (t._id == v._id) {
                                loadTeam(i)
                                return
                            }
                        }
                      }}
                    />
        </Grid>
        <Grid item xs={1} sm={1}>
            <IconButton onClick={nextTeam} >
              <NavigateNextIcon />
            </IconButton>
        </Grid>
      </Grid>
}
      <Grid container spacing={2}>
        &nbsp;
      </Grid>
      <Grid container spacing={2}>
        {/* 1st column / maneuvers*/}
        <Grid container xs={6}>
            <Grid xs={12}>
              <Typography variant="h5">Maneuvers</Typography>
            </Grid>
{ [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20].map(i => {
            var trick = null
            if (data && data.tricks && data.tricks[i]) trick = data.tricks[i]
            return(
            <Grid xs={12} key={i}>
                    <Autocomplete
                      id="autocomplete-trick-{i}"
                      key="autocomplete-trick-{i}"
                      options={uniqueTricks}
                      groupBy={t => t.base_trick}
                      getOptionLabel={(p) => `${p.name} (${p.acronym}) (${p.technical_coefficient})`}
                      renderInput={(params) => <TextField {...params} name="trick" />}
                      value={trick}
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
        <Grid container xs={6}>
          {/* marks */}
          <Grid container xs={12}>
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
          <TableCell>
            Synchro
          </TableCell>
      </TableRow>
    </TableHead>
            <TableBody>
{ run.judges.map((j) => {
    var technical = null
    var choreography = null
    var landing = null
    var synchro = null
    for (let m in data.marks) {
      m = data.marks[m]
      if (m.judge == j._id) {
        technical = m.technical
        choreography = m.choreography
        landing = m.landing
        synchro = m.synchro
        break
      }
    }
    return (
              <TableRow>
                <TableCell>
                  <Typography>{ j.name }</Typography>
                </TableCell>
                <TableCell>
                  <InputMark onChange={value => {setMark('technical', j, value)}} value={technical}/>
                </TableCell>
                <TableCell>
                  <InputMark onChange={value => {setMark('choreography', j, value)}} value={choreography}/>
                </TableCell>
                <TableCell>
                  <InputMark onChange={value => {setMark('landing', j, value)}} value={landing}/>
                </TableCell>
                <TableCell>
                  <InputMark onChange={value => {setMark('synchro', j, value)}} value={synchro}/>
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
          <Grid container xs={12}>
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
                  <TableCell>
                    <Typography>Synchro: {result.judges_mark.synchro ?? ""}</Typography>
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
                  <TableCell>
                    <Typography>Synchro: {result.synchro ?? ""}</Typography>
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
  { result.warnings.map((warning, i) => <li>{warning} <DeleteIcon onClick={() => removeWarning(i)} /></li>)}
  </ul>
  </>)}
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </Grid>
          {/* actions */}
          <Grid container xs={12}>
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
    </CardContent>
  )
}

export default TabFlightsSynchro
