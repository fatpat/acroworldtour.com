// ** Icon imports
import Login from 'mdi-material-ui/Login'
import Table from 'mdi-material-ui/Table'
import CubeOutline from 'mdi-material-ui/CubeOutline'
import HomeOutline from 'mdi-material-ui/HomeOutline'
import FormatLetterCase from 'mdi-material-ui/FormatLetterCase'
import AccountCogOutline from 'mdi-material-ui/AccountCogOutline'
import CreditCardOutline from 'mdi-material-ui/CreditCardOutline'
import AccountPlusOutline from 'mdi-material-ui/AccountPlusOutline'
import AlertCircleOutline from 'mdi-material-ui/AlertCircleOutline'
import GoogleCirclesExtended from 'mdi-material-ui/GoogleCirclesExtended'
import Account from 'mdi-material-ui/Account'
import AccountCowboyHat from 'mdi-material-ui/AccountCowboyHat'
import AccountGroup from 'mdi-material-ui/AccountGroup'
import ScriptText from 'mdi-material-ui/ScriptText'
import ScriptTextOutline from 'mdi-material-ui/ScriptTextOutline'
import DvrIcon from '@mui/icons-material/Dvr'
import SettingsIcon from '@mui/icons-material/Settings'
import SportsScoreIcon from '@mui/icons-material/SportsScore'

const navigation = () => {
  return [
    {
      title: 'Dashboard',
      icon: HomeOutline,
      path: '/'
    },
    {
      sectionTitle: 'Main Menu'
    },
    {
      title: 'Pilots',
      icon: Account,
      path: '/pilots'
    },
    {
      title: 'Teams',
      icon: AccountGroup,
      path: '/teams'
    },
    {
      title: 'Judges',
      icon: AccountCowboyHat,
      path: '/judges'
    },
    {
      title: 'Tricks',
      icon: ScriptTextOutline,
      path: '/tricks'
    },
    {
      title: 'UniqueTricks',
      icon: ScriptText,
      path: '/unique-tricks'
    },
    {
      title: 'Competitions',
      icon: DvrIcon,
      path: '/competitions'
    },
    {
      title: 'Seasons',
      icon: SportsScoreIcon,
      path: '/seasons'
    },
    {
      title: 'Utils',
      icon: SettingsIcon,
      path: '/utils'
    },
  ]
}

export default navigation
