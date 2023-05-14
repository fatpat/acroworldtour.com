// ** MUI Imports
import Box from '@mui/material/Box'
import Badge from '@mui/material/Badge'
import Card from '@mui/material/Card'
import Button from '@mui/material/Button'
import Link from '@mui/material/Link'
import Avatar from '@mui/material/Avatar'
import CardMedia from '@mui/material/CardMedia'
import Typography from '@mui/material/Typography'
import CardContent from '@mui/material/CardContent'
import ReactCountryFlag from 'react-country-flag'
import countriesQuery from 'countries-code'

const CardPilot = (props) => {
  const pilot = props.pilot

  return (
    <Card sx={{ position: 'relative' }}>
      <CardMedia sx={{ height: '12.625rem' }} image={pilot.background_picture} />
      <Avatar
        alt={pilot.name}
        src={pilot.photo}
        sx={{
          width: 150,
          height: 150,
          left: '1.313rem',
          top: '4.5rem',
          position: 'absolute',
          border: theme => `0.25rem solid ${theme.palette.common.white}`
        }}
      />
      <CardContent>
        <Box
          sx={{
            mt: 5.75,
            mb: 8.75,
            display: 'flex',
            flexWrap: 'wrap',
            alignItems: 'center',
            justifyContent: 'space-between'
          }}
        >
          <Box sx={{ mr: 2, mb: 1, display: 'flex', flexDirection: 'column' }}>
            <Typography variant='h6'>
              <Link href={pilot.link} target="_blank" rel="noopener noreferrer">{pilot.name}</Link>{' '}
              <span onClick={() => props.changeGender(pilot.civlid)}>{pilot.gender == "man" ? "♂️" : "♀️"}</span>
              &nbsp;
              <ReactCountryFlag
                countryCode={countriesQuery.convertAlphaCode(pilot.country)}
                svg
                title={countriesQuery.getCountry(pilot.country)}
              />
            </Typography>
            <Typography variant='caption'>CIVL ID: {pilot.civlid}</Typography>
            <Typography variant='caption'>Rank: #{pilot.rank}</Typography>
          </Box>
          <Button variant='contained' onClick={() => props.updatePilot(pilot.civlid)} >
            Update pilot
          </Button>
        </Box>
      </CardContent>
    </Card>
  )
}

export default CardPilot
