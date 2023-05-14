// ** React Imports
import { useEffect, useState } from 'react'
import { useRouter } from 'next/router'

// ** MUI Imports
import Grid from '@mui/material/Grid'
import Box from '@mui/material/Box'
import Link from '@mui/material/Link'
import Button from '@mui/material/Button'
import Typography from '@mui/material/Typography'
import CardContent from '@mui/material/CardContent'
import TableBody from '@mui/material/TableBody'
import TableCell from '@mui/material/TableCell'
import TableRow from '@mui/material/TableRow'
import TableHead from '@mui/material/TableHead'
import Table from '@mui/material/Table'
import TableContainer from '@mui/material/TableContainer'
import EmojiEventsIcon from '@mui/icons-material/EmojiEvents'
import ArticleIcon from '@mui/icons-material/Article'
import CloudDownloadIcon from '@mui/icons-material/CloudDownload'

// ** jquery
import $ from 'jquery'

// ** local
import EnhancedTable from 'src/views/tables/EnhancedTable'
import TabRunResults from 'src/views/competitions/TabRunResults'
import { useNotifications } from 'src/util/notifications'
import { APIRequest } from 'src/util/backend'

const TabResults = ({ code }) => {
  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** State
  const [results, setResults] = useState(false)

  const loadResults = async() => {

    const [err, retData, headers] = await APIRequest(`/competitions/${code}/results`, {
      expect_json: true
    })

    if (err) {
        error(`error while retrieving results for competition ${code}: ${err}`)
        setResults(false)
        return
    }

    retData.overall_results = retData.overall_results.map((r, i) => {
      r.rank = i+1
      return r
    })

    setResults(retData)
  }

  useEffect(() => {
    loadResults()
  }, [])

  if (!results) return('loading ...')

  window.onbeforeprint = (event) => {
    $('.hideToPrint').hide()
  }; 
  window.onafterprint = (event) => {
    $('.hideToPrint').show()
  }; 

  return (
    <CardContent>
      <Box sx={{display: 'flex',justifyContent: 'right'}}>
        <Button href={new URL(`/competitions/${code}/results/export`, process.env.NEXT_PUBLIC_API_URL).toString()} startIcon={<ArticleIcon />}>Download CIVL Export</Button>
        <Button href={new URL(`/competitions/${code}/results/export?filetype=html`, process.env.NEXT_PUBLIC_API_URL).toString()} startIcon={<CloudDownloadIcon />} target="_blank" rel="noreferrer">PDF Export</Button>
      </Box>
      <Box sx={{display: 'flex',justifyContent: 'center'}}>
      <Typography variant="h4">
        <EmojiEventsIcon fontSize="large"/>{ results.final ? 'Final' : 'Intermediate' } Overall
      </Typography>
      </Box>
      <Grid container spacing={7}>
        <Grid item xs={12} sm={12}>
          <TableContainer>
            <Table sx={{ minWidth: 750 }}>
              <TableHead>
                <TableRow>
                  <TableCell>Rank</TableCell>
                  <TableCell>Pilot</TableCell>
                  <TableCell>Run</TableCell>
                  <TableCell>Score</TableCell>
                  <TableCell>Rank</TableCell>
                  <TableCell>Score</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
{ results.overall_results.map((r,rank) => (
                <TableRow key={`result-${rank}`}>
                  <TableCell>{rank+1}</TableCell>
                  <TableCell>
                    {results.type == "solo" && r.pilot.name}
                    {results.type == "synchro" && <p>{r.team.name}<ul><li>{r.team.pilots[0].name}</li><li>{r.team.pilots[1].name}</li></ul></p>}
                  </TableCell>
                  <TableCell>
                    {r.result_per_run.map((rr, rid) => (`Run ${rid+1}`)).reduce((res, v) => {
                      if (!res) return [v]
                      return [...res, <br />, v]
                    })}
                  </TableCell>
                  <TableCell>
                    {r.result_per_run.map((rr, rid) => (rr.score.toFixed(3))).reduce((res, v) => {
                      if (!res) return [v]
                      return [...res, <br />, v]
                    })}
                  </TableCell>
                  <TableCell>
                    {r.result_per_run.map((rr, rid) => (`${rr.rank}`)).reduce((res, v) => {
                      if (!res) return [v]
                      return [...res, <br />, v]
                    })}
                  </TableCell>
                  <TableCell>{r.score.toFixed(3)}</TableCell>
                </TableRow>
))}
              </TableBody>
            </Table>
          </TableContainer>
        </Grid>
      </Grid>
{/*
{ results.runs_results.map((r, rid) => {
    return(
      <TabRunResults code={code} rid={rid} />
)})}
*/}
    </CardContent>
  )
}

export default TabResults
