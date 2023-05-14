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

// ** local
import EnhancedTable from 'src/views/tables/EnhancedTable'
import { useNotifications } from 'src/util/notifications'
import { APIRequest, useUniqueTricks } from 'src/util/backend'

const SelectMark = (props) => {
  return (
    <NativeSelect onChange={e => {props.onChange(e)}}>
      { Array.from({length: 21}, (v, k) => k*0.5).map(i => (<option value={i} selected={props.value==i}>{i}</option>))}
    </NativeSelect>
  )
}

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
        console.log(`error while simlating score: ${err}`)
        setResult({
          judges_mark:{}
        })
        setResultsOK(false)
        return
    }
    console.log("simulated score:", retData)
    setResult(retData)
    setResultsOK(true)
  }

  const setMark = (type, judge, mark) => {
    for (const [i, m] of data.marks.entries()) {
        if (data.marks[i].judge == judge._id) {
          data.marks[i][type] = mark
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
                      options={run.pilots.sort((a,b) => b.rank-a.rank)}
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
      <Grid container spacing={2}>
        &nbsp;
      </Grid>
      <Grid container spacing={2}>
        {/* 1st column / maneuvers*/}
        <Grid container xs={6}>
            <Grid xs={12}>
              <Typography variant="h5">Maneuvers</Typography>
            </Grid>
{ [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20].map(i =>{
            var trick = null
            if (data && data.tricks && data.tricks[i]) trick = data.tricks[i]
            return(
            <Grid xs={12} key={i}>
                    <Autocomplete
                      id="autocomplete-trick-{i}"
                      key="autocomplete-trick-{i}"
                      options={uniqueTricks}
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
      </TableRow>
    </TableHead>
            <TableBody>
{ run.judges.map((j) => {
    var technical = 0
    var choreography = 0
    var landing = 0
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
              <TableRow>
                <TableCell>
                  <Typography>{ j.name }</Typography>
                </TableCell>
                <TableCell>
                  <SelectMark onChange={e => {setMark('technical', j, e.target.value)}} value={technical}/>
                </TableCell>
                <TableCell>
                  <SelectMark onChange={e => {setMark('choreography', j, e.target.value)}} value={choreography}/>
                </TableCell>
                <TableCell>
                  <SelectMark onChange={e => {setMark('landing', j, e.target.value)}} value={landing}/>
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
              </Grid>
          </Grid>
        </Grid>
      </Grid>
    </CardContent>
  )
}

export default TabFlights
