// ** React Imports
import { useEffect, useState } from 'react'
import { useRouter } from 'next/router'

// ** MUI Imports
import Grid from '@mui/material/Grid'
import ListItem from '@mui/material/ListItem'
import List from '@mui/material/List'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Typography from '@mui/material/Typography'
import CardContent from '@mui/material/CardContent'
import TableBody from '@mui/material/TableBody'
import TableCell from '@mui/material/TableCell'
import TableRow from '@mui/material/TableRow'
import TableHead from '@mui/material/TableHead'
import Table from '@mui/material/Table'
import TableContainer from '@mui/material/TableContainer'
import Checkbox from '@mui/material/Checkbox'
import EmojiEventsIcon from '@mui/icons-material/EmojiEvents'
import ArticleIcon from '@mui/icons-material/Article'
import CloudDownloadIcon from '@mui/icons-material/CloudDownload'

// ** jquery
import $ from 'jquery'

// ** local
import EnhancedTable from 'src/views/tables/EnhancedTable'
import { useNotifications } from 'src/util/notifications'
import { APIRequest } from 'src/util/backend'

const TabResults = ({ code, rid }) => {
  // ** notification messages
  const [success, info, warning, error] = useNotifications()

  // ** State
  const [results, setResults] = useState(false)

  const loadResults = async() => {

    const [err, data, headers] = await APIRequest(`/competitions/${code}/results/${rid}?published_only=false`, {
      expect_json: true
    })

    if (err) {
        error(`error while retrieving results for competition ${code}: ${err}`)
        setResults(false)
        return
    }

    data.results = data.results.map((r, i) => {
      r.rank = i+1
      return r
    })

    setResults(data)
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
        <Button href={new URL(`/competitions/${code}/results/${rid}/export`, process.env.NEXT_PUBLIC_API_URL).toString()} startIcon={<ArticleIcon />}>CIVL Run Export Run Results</Button>
        <Button href={new URL(`/competitions/${code}/results/export?limit_run=${rid}`, process.env.NEXT_PUBLIC_API_URL).toString()} startIcon={<ArticleIcon />}>CIVL Export Overall Results after Run</Button>
        <Button href={new URL(`/competitions/${code}/results/${rid}/export?filetype=html`, process.env.NEXT_PUBLIC_API_URL).toString()} startIcon={<CloudDownloadIcon />} target="_blank" rel="noreferrer">PDF Run Results</Button>
        <Button href={new URL(`/competitions/${code}/results/export?limit_run=${rid}&filetype=html`, process.env.NEXT_PUBLIC_API_URL).toString()} startIcon={<CloudDownloadIcon />} target="_blank" rel="noreferrer">PDF Overall Results after Run</Button>
      </Box>
      <Box sx={{display: 'flex',justifyContent: 'center'}}>
      <Typography variant="h4">
        <EmojiEventsIcon fontSize="large"/>{ results.final ? 'Final' : 'Intermediate' } Run {parseInt(rid)+1} Results
      </Typography>
      </Box>
      <Grid container spacing={7}>
        <Grid item xs={12} sm={12}>
          <TableContainer>
            <Table sx={{ minWidth: 750 }}>
              <TableHead>
                <TableRow>
                  <TableCell>Rank</TableCell>
                  <TableCell>Team</TableCell>
                  <TableCell>Tricks</TableCell>
                  <TableCell>Warnings</TableCell>
                  <TableCell>Technicity</TableCell>
                  <TableCell>Judges marks</TableCell>
                  <TableCell>Final marks</TableCell>
                  <TableCell>Bonus</TableCell>
                  <TableCell>Score</TableCell>
                  <TableCell>Published</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
{ results.results.sort((a,b) => b.final_marks.score-a.final_marks.score).map((r,rank) => {
  return(
                <TableRow key={rank}>
                  <TableCell>
                    {rank+1}
                  </TableCell>
                  <TableCell>
                    {results.type == "solo" && r.pilot.name}
                    {results.type == "synchro" && <p>{r.team.name}<ul><li>{r.team.pilots[0].name}</li><li>{r.team.pilots[1].name}</li></ul></p>}
                  </TableCell>
                  <TableCell>
{r.tricks.map((t, i) => {
    return(
        <p key={i}>{t.name} ({t.technical_coefficient}, {t.bonus}%)</p>
    )
})}
                  </TableCell>
                  <TableCell sx={{width: "30%"}}>
                    <p>warnings: {r.final_marks.warnings.length}
{ r.final_marks.warnings.length > 0 &&
                      <List>
  { r.final_marks.warnings.map(w => (
                        <ListItem>{w}</ListItem>
  ))}
                      </List>
}
                    </p>
                    <p>Malus: {r.final_marks.malus}%</p>
{ r.final_marks.notes.length > 0 &&
                    <p>
                      <List>
  { r.final_marks.notes.map(n => (
                        <ListItem>{n}</ListItem>
  ))}
                      </List>
                    </p>
}
                    { r.did_not_start && <p>DID NOT START</p>}
                  </TableCell>
                  <TableCell>
                    <p>Techniciy: {r.final_marks.technicity.toFixed(3)}</p>
                    <p>Bonus: {r.final_marks.bonus_percentage}%</p>
                  </TableCell>
                  <TableCell>
                    <p>Technical: {r.final_marks.judges_mark.technical.toFixed(3)}</p>
                    <p>Choreography: {r.final_marks.judges_mark.choreography.toFixed(3)}</p>
                    <p>Landing: {r.final_marks.judges_mark.landing.toFixed(3)}</p>
{ results.type == "synchro" &&
                    <p>Synchro: {r.final_marks.judges_mark.synchro.toFixed(3)}</p>
}
                  </TableCell>
                  <TableCell>
                    <p>Technical: {r.final_marks.technical.toFixed(3)}</p>
                    <p>Choreography: {r.final_marks.choreography.toFixed(3)}</p>
                    <p>Landing: {r.final_marks.landing.toFixed(3)}</p>
{ results.type == "synchro" &&
                    <p>Synchro: {r.final_marks.synchro.toFixed(3)}</p>
}
                  </TableCell>
                  <TableCell>{r.final_marks.bonus.toFixed(3)}</TableCell>
                  <TableCell><strong>{r.final_marks.score.toFixed(3)}</strong></TableCell>
                  <TableCell><Checkbox disabled checked={r.published} /></TableCell>
                </TableRow>
  )
})}
              </TableBody>
            </Table>
          </TableContainer>
        </Grid>
      </Grid>
    </CardContent>
  )
}

export default TabResults
